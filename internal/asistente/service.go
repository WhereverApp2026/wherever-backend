package asistente

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendador"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Fuentes del plan generado.
const (
	FuenteIA     = "ia"
	FuenteReglas = "reglas"
)

const (
	avisoIA          = "Plan sugerido a partir de tu frase. Revisa horarios y precios antes de salir."
	retryLimiteIA    = 60 // segundos, si el proveedor no dice cuánto esperar
	maxAnticipacion  = 30 * 24 * time.Hour
	toleranciaPasado = 5 * time.Minute
)

// zona es la hora local del piloto (constitución: zona de la ciudad, por defecto CDMX).
var zona = func() *time.Location {
	loc, err := time.LoadLocation("America/Mexico_City")
	if err != nil {
		return time.FixedZone("CST", -6*60*60)
	}
	return loc
}()

// Usuarios lee el perfil (gustos, alergias, preferencias y consentimientos).
type Usuarios interface {
	FindByID(ctx context.Context, id bson.ObjectID) (usuarios.User, error)
}

// Config son las dependencias de valor del asistente.
type Config struct {
	Pesos       recomendador.Pesos
	Velocidades planes.Velocidades
	Ahora       func() time.Time // nil = time.Now
}

// Service arma planes desde una frase. Sin modelo (IA apagada) responde 503 hasta nv-33.
type Service struct {
	modelo    ia.Modelo
	fuente    recomendador.Fuente
	users     Usuarios
	descartes Descartes
	cfg       Config
	log       *slog.Logger
}

// NewService valida los pesos al arrancar. modelo puede ser nil.
func NewService(modelo ia.Modelo, fuente recomendador.Fuente, users Usuarios, descartes Descartes, cfg Config, log *slog.Logger) (*Service, error) {
	if err := cfg.Pesos.Validate(); err != nil {
		return nil, errors.Join(recomendador.ErrPesos, err)
	}
	if cfg.Velocidades == nil {
		cfg.Velocidades = planes.DefaultVelocidades()
	}
	if cfg.Ahora == nil {
		cfg.Ahora = time.Now
	}
	return &Service{modelo: modelo, fuente: fuente, users: users, descartes: descartes, cfg: cfg, log: log}, nil
}

// Generar: frase → Interpretar (IA) → Recomendar (15 candidatos) → itinerario (IA) →
// Verificar contra los candidatos → Recalcular.
func (s *Service) Generar(ctx context.Context, subject string, req GenerarRequest) (GenerarResponse, error) {
	if s.modelo == nil {
		return GenerarResponse{}, apperr.Unavailable("el asistente no está disponible")
	}
	user, err := s.user(ctx, subject)
	if err != nil {
		return GenerarResponse{}, err
	}
	ahora := s.cfg.Ahora().In(zona)
	inicio, err := fechaPedida(req.Fecha, ahora)
	if err != nil {
		return GenerarResponse{}, err
	}
	in, err := Interpretar(ctx, s.modelo, req.Frase, ahora)
	if err != nil {
		return GenerarResponse{}, s.errorIA(err)
	}
	if !user.Privacy.Mood {
		in.Animo = ""
	}
	if inicio.IsZero() {
		inicio = inicioDe(in.Hora, ahora)
	}
	pedido := Pedido{Inicio: inicio, Personas: personasDe(req.Personas, in), Movilidad: s.movilidad(req.Movilidad, user)}
	cctx := contexto(*req.Lat, *req.Lng, inicio, in, user.Tastes)
	pedido.PresupuestoMax = cctx.PresupuestoMax
	recs, err := recomendador.Recomendar(ctx, s.fuente, s.cfg.Pesos, cctx, MaxCandidatos)
	if err != nil {
		s.log.ErrorContext(ctx, "asistente: candidatos no disponibles")
		return GenerarResponse{}, apperr.Unavailable("servicio no disponible")
	}
	plan, usados, err := s.itinerario(ctx, recs, pedido, in)
	if err != nil {
		return GenerarResponse{}, err
	}
	return GenerarResponse{Fuente: FuenteIA, Aviso: avisoIA, Parametros: parametros(in), Plan: planDTO(plan, usados)}, nil
}

// itinerario pide las paradas a la IA, las verifica contra los candidatos y registra los
// descartes. Sin candidatos no llama a la IA.
func (s *Service) itinerario(ctx context.Context, recs []recomendador.Recomendacion, pedido Pedido, in Interpretacion) (planes.Plan, []recomendador.Recomendacion, error) {
	sinLugares := apperr.Validation("no hay lugares abiertos y compatibles cerca; prueba otra hora o zona")
	if len(recs) == 0 {
		return planes.Plan{}, nil, sinLugares
	}
	it, err := GenerarItinerario(ctx, s.modelo, recs, pedido, in)
	if err != nil {
		return planes.Plan{}, nil, s.errorIA(err)
	}
	plan, usados, descartes, err := Verificar(it, recs, pedido, s.cfg.Velocidades)
	s.registrar(ctx, descartes)
	if errors.Is(err, ErrSinCandidatos) {
		return planes.Plan{}, nil, sinLugares
	}
	if err != nil {
		return planes.Plan{}, nil, err
	}
	s.log.InfoContext(ctx, "asistente: plan generado", "fuente", FuenteIA, "candidatos", len(recs),
		"propuestas", len(it.Paradas), "descartes", len(descartes), "paradas", len(plan.Paradas))
	return plan, usados, nil
}

// registrar guarda los descartes; si la base falla solo se avisa: la medición no tumba el plan.
func (s *Service) registrar(ctx context.Context, ds []Descarte) {
	for _, d := range ds {
		s.log.InfoContext(ctx, "asistente: parada descartada", "motivo", d.Motivo)
	}
	if err := s.descartes.Registrar(ctx, s.modelo.Nombre(), ds); err != nil {
		s.log.WarnContext(ctx, "asistente: no se registraron los descartes", "cantidad", len(ds))
	}
}

func (s *Service) user(ctx context.Context, subject string) (usuarios.User, error) {
	id, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return usuarios.User{}, apperr.Unauthorized("credenciales inválidas")
	}
	u, err := s.users.FindByID(ctx, id)
	if errors.Is(err, usuarios.ErrNotFound) {
		return usuarios.User{}, apperr.Unauthorized("credenciales inválidas")
	}
	if err != nil {
		return usuarios.User{}, apperr.Unavailable("servicio no disponible")
	}
	return u, nil
}

// errorIA traduce la falla del proveedor. El log lleva solo el tipo: ni la frase ni el
// texto del proveedor.
func (s *Service) errorIA(err error) error {
	var limite *ia.LimiteError
	switch {
	case errors.As(err, &limite):
		s.log.Warn("asistente: límite del proveedor de IA")
		retry := int(limite.Espera.Seconds())
		if retry <= 0 {
			retry = retryLimiteIA
		}
		return apperr.RateLimited(retry)
	case errors.Is(err, ia.ErrLimiteProveedor):
		s.log.Warn("asistente: límite del proveedor de IA")
		return apperr.RateLimited(retryLimiteIA)
	default:
		s.log.Warn("asistente: la IA no respondió", "motivo", motivoIA(err))
		return apperr.Unavailable("el asistente no está disponible; intenta de nuevo")
	}
}

func motivoIA(err error) string {
	for _, m := range []struct {
		err    error
		motivo string
	}{
		{ia.ErrTiempo, "tiempo"}, {ia.ErrRespuestaInvalida, "respuesta_invalida"},
		{ia.ErrBloqueada, "bloqueada"}, {context.Canceled, "cancelada"},
	} {
		if errors.Is(err, m.err) {
			return m.motivo
		}
	}
	return "proveedor"
}

func (s *Service) movilidad(pedida string, u usuarios.User) string {
	for _, m := range []string{pedida, u.Preferences.Mobility} {
		if _, ok := s.cfg.Velocidades[m]; ok {
			return m
		}
	}
	return planes.MobilityWalk
}

// fechaPedida valida `fecha`: de ahora (con 5 min de tolerancia) a 30 días. Cero = no vino.
func fechaPedida(f *time.Time, ahora time.Time) (time.Time, error) {
	if f == nil {
		return time.Time{}, nil
	}
	if f.Before(ahora.Add(-toleranciaPasado)) || f.After(ahora.Add(maxAnticipacion)) {
		return time.Time{}, apperr.Validation("datos inválidos",
			apperr.FieldError{Field: "fecha", Message: "debe estar entre ahora y 30 días"})
	}
	return f.In(zona), nil
}

// inicioDe: la hora de la frase hoy (mañana si ya pasó) o ahora redondeado a 5 minutos.
func inicioDe(hora string, ahora time.Time) time.Time {
	h, err := time.ParseInLocation(formatoHora, hora, zona)
	if err != nil {
		if r := ahora.Truncate(redondeoLlegada); r.Before(ahora) {
			return r.Add(redondeoLlegada)
		}
		return ahora
	}
	t := time.Date(ahora.Year(), ahora.Month(), ahora.Day(), h.Hour(), h.Minute(), 0, 0, zona)
	if t.Before(ahora) {
		t = t.AddDate(0, 0, 1)
	}
	return t
}

func personasDe(pedidas int, in Interpretacion) int {
	switch {
	case pedidas > 0:
		return pedidas
	case in.Compania == "pareja":
		return 2
	default:
		return 1
	}
}

// contexto arma la entrada del motor. Alergias y dieta del perfil son filtros duros siempre;
// el presupuesto de la frase manda sobre el del perfil.
func contexto(lat, lng float64, inicio time.Time, in Interpretacion, g *usuarios.Tastes) recomendador.Contexto {
	c := recomendador.Contexto{
		Lat: lat, Lng: lng, Momento: inicio, PresupuestoMax: in.PresupuestoMax,
		Gustos: in.PalabrasClave, ConNinos: in.ConNinos, ConMascota: in.ConMascota,
	}
	if g == nil {
		return c
	}
	c.Gustos = append(append([]string{}, g.Categories...), in.PalabrasClave...)
	c.Alergenos, c.AlergiaSinCatalogo = recomendador.ParseAlergias(g.Allergies)
	c.Dietas = recomendador.DietasDe(g.Restrictions)
	if c.PresupuestoMax == 0 {
		c.PresupuestoMin, c.PresupuestoMax = g.BudgetMin, g.BudgetMax
	}
	return c
}

// parametros: misma forma que Interpretacion; solo cambia la etiqueta de `animo`.
func parametros(in Interpretacion) Parametros { return Parametros(in) }

func planDTO(p planes.Plan, usados []recomendador.Recomendacion) PlanDTO {
	paradas := make([]ParadaDTO, 0, len(p.Paradas))
	for _, st := range p.Paradas {
		d := ParadaDTO{Parada: st}
		for _, r := range usados {
			if r.Negocio.ID == st.NegocioID {
				d.Nombre, d.Categoria, d.PorQue = r.Negocio.Name, r.Negocio.Category, r.PorQue
			}
		}
		paradas = append(paradas, d)
	}
	return PlanDTO{
		Nombre: p.Nombre, Fecha: p.Fecha, Personas: p.Personas, Presupuesto: p.Presupuesto,
		Movilidad: p.Movilidad, Paradas: paradas, Traslados: p.Traslados,
		CostoTotal: p.CostoTotal, CostoPorPersona: p.CostoPorPersona, Origen: p.Origen,
	}
}

// normalizarFrase quita espacios de los extremos antes de validar (una frase en blanco es 422).
func normalizarFrase(req *GenerarRequest) { req.Frase = strings.TrimSpace(req.Frase) }
