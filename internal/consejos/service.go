package consejos

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// IA es lo que usa el consejo de ia.Guardia (tope, caché y límites de nv-27/nv-28).
type IA interface {
	GenerarValidado(ctx context.Context, pet ia.Peticion, destino any) (ia.Respuesta, error)
}

// Negocios trae el negocio para validar al dueño.
type Negocios interface {
	FindByID(ctx context.Context, id bson.ObjectID) (negocios.Business, error)
}

// Store es el repositorio del consejo y de los agregados que lee.
type Store interface {
	Buscar(ctx context.Context, negocio bson.ObjectID, dia string) (*Consejo, error)
	Tomar(ctx context.Context, c Consejo) error
	Retomar(ctx context.Context, id bson.ObjectID, vencido, ahora time.Time) (bool, error)
	Terminar(ctx context.Context, c Consejo) error
	ConIADelDia(ctx context.Context, dia string) (int64, error)
	BorrarNegocio(ctx context.Context, negocio bson.ObjectID) error
	Dias(ctx context.Context, negocio bson.ObjectID, desde string) ([]Dia, error)
	ResenasSinRespuesta(ctx context.Context, negocio bson.ObjectID, desde time.Time) (int, error)
	CuponesSinCanjes(ctx context.Context, negocio bson.ObjectID, desde time.Time) (int, error)
}

// Service da el consejo del día. Sin IA (o sin cupo) responde la plantilla de la regla.
type Service struct {
	repo     Store
	negocios Negocios
	ia       IA
	cupo     int
	zona     *time.Location
	log      *slog.Logger
	now      func() time.Time
}

// NewService recibe sus dependencias por constructor; la IA se conecta con WithIA.
func NewService(repo Store, n Negocios, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	zona, err := time.LoadLocation(Zona)
	if err != nil {
		zona = time.UTC
	}
	return &Service{repo: repo, negocios: n, zona: zona, log: log, now: time.Now}
}

// WithIA conecta la guardia con el cupo diario de toda la app (IA_CONSEJO_DIA); 0 = sin IA.
func (s *Service) WithIA(m IA, cupo int) *Service {
	s.ia, s.cupo = m, cupo
	return s
}

// WithReloj fija el reloj (pruebas).
func (s *Service) WithReloj(now func() time.Time) *Service {
	s.now = now
	return s
}

func errStore() error { return apperr.Unavailable("servicio no disponible") }

// propio trae el negocio de la persona sin baja; uno ajeno o inexistente = 404.
func (s *Service) propio(ctx context.Context, cl *middleware.Claims, raw string) (negocios.Business, bson.ObjectID, error) {
	if cl == nil {
		return negocios.Business{}, bson.ObjectID{}, apperr.Unauthorized("autenticación requerida")
	}
	owner, err := bson.ObjectIDFromHex(cl.Subject)
	if err != nil {
		return negocios.Business{}, bson.ObjectID{}, apperr.Unauthorized("sesión inválida")
	}
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return negocios.Business{}, owner, apperr.NotFound("negocio no encontrado")
	}
	b, err := s.negocios.FindByID(ctx, id)
	if errors.Is(err, negocios.ErrNotFound) || (err == nil && (b.DeletedAt != nil || b.OwnerID != owner)) {
		return negocios.Business{}, owner, apperr.NotFound("negocio no encontrado")
	}
	if err != nil {
		return negocios.Business{}, owner, errStore()
	}
	return b, owner, nil
}

// Consejo responde el consejo del día. La primera petición del día inserta el documento en
// `generando` (ux_consejo_dia decide quién genera); la que pierde la carrera responde la
// plantilla sin guardarla. Un `generando` de más de un minuto se retoma. La IA nunca da 429 ni
// 503: solo MongoDB caído da 503.
func (s *Service) Consejo(ctx context.Context, cl *middleware.Claims, raw string) (ConsejoResponse, error) {
	b, owner, err := s.propio(ctx, cl, raw)
	if err != nil {
		return ConsejoResponse{}, err
	}
	ahora := s.now()
	hoy := ahora.In(s.zona)
	dia := hoy.Format(FormatoDia)
	doc, propio, err := s.turno(ctx, b.ID, dia, ahora)
	if err != nil {
		return ConsejoResponse{}, err
	}
	if doc != nil && doc.Estado == EstadoListo {
		return respuesta(*doc), nil
	}
	h, err := s.hecho(ctx, b, hoy)
	if err != nil {
		return ConsejoResponse{}, errStore()
	}
	if !propio {
		return ConsejoResponse{Texto: Plantilla(h), Accion: h.Accion, Fuente: FuenteReglas, Dia: dia}, nil
	}
	doc.Texto, doc.Fuente = s.redactar(ctx, owner, b.Category, h, dia)
	doc.Accion, doc.Regla, doc.Estado = h.Accion, h.Regla, EstadoListo
	if err := s.repo.Terminar(ctx, *doc); err != nil {
		// El consejo ya está redactado: se responde y el `generando` se retoma en un minuto.
		s.log.WarnContext(ctx, "consejos: no se guardó el consejo", "evento", "consejo_sin_guardar", "regla", h.Regla, "error", err.Error())
	} else {
		s.log.InfoContext(ctx, "consejos: consejo del día", "evento", "consejo_generado", "regla", h.Regla, "fuente", doc.Fuente)
	}
	return respuesta(*doc), nil
}

// turno devuelve el documento del día y si esta petición lo genera. Un documento listo se
// responde tal cual; uno en `generando` reciente es de otra petición.
func (s *Service) turno(ctx context.Context, negocio bson.ObjectID, dia string, ahora time.Time) (*Consejo, bool, error) {
	c, err := s.repo.Buscar(ctx, negocio, dia)
	if err != nil {
		return nil, false, errStore()
	}
	if c == nil {
		nuevo := Consejo{ID: bson.NewObjectID(), NegocioID: negocio, Dia: dia, Estado: EstadoGenerando, CreadoEn: ahora.UTC(), ExpiraEn: ahora.UTC().Add(VidaConsejo)}
		err = s.repo.Tomar(ctx, nuevo)
		if err == nil {
			return &nuevo, true, nil
		}
		if !errors.Is(err, ErrOcupado) {
			return nil, false, errStore()
		}
		c, err = s.repo.Buscar(ctx, negocio, dia)
		if err != nil || c == nil {
			return nil, false, errStore()
		}
		return c, false, nil
	}
	if c.Estado == EstadoListo {
		return c, false, nil
	}
	ok, err := s.repo.Retomar(ctx, c.ID, ahora.Add(-PlazoGenerando), ahora.UTC())
	if err != nil {
		return nil, false, errStore()
	}
	return c, ok, nil
}

// hecho lee los agregados del negocio y elige la regla.
func (s *Service) hecho(ctx context.Context, b negocios.Business, hoy time.Time) (Hecho, error) {
	y, m, d := hoy.Date()
	base := time.Date(y, m, d, 0, 0, 0, 0, s.zona)
	dias, err := s.repo.Dias(ctx, b.ID, base.AddDate(0, 0, -2*DiasVentana).Format(FormatoDia))
	if err != nil {
		return Hecho{}, err
	}
	desde := base.AddDate(0, 0, -DiasRecientes).UTC()
	sinRespuesta, err := s.repo.ResenasSinRespuesta(ctx, b.ID, desde)
	if err != nil {
		return Hecho{}, err
	}
	sinCanjes, err := s.repo.CuponesSinCanjes(ctx, b.ID, desde)
	if err != nil {
		return Hecho{}, err
	}
	return Elegir(Datos{Hoy: hoy, Dias: dias, ResenasSinRespuesta: sinRespuesta, CuponesSinCanjes: sinCanjes,
		AceptaReservas: aceptaReservas(b), PerfilIncompleto: strings.TrimSpace(b.Description) == "" || len(b.Photos) == 0}), nil
}

func aceptaReservas(b negocios.Business) bool {
	if b.ReservasConfig != nil {
		return b.ReservasConfig.AceptaReservas
	}
	return b.AcceptsReservations
}

// redactar pide el texto a la IA si hay guardia y cupo del día; con cualquier falla, o si el
// modelo cambia la acción, usa la plantilla. El log lleva evento, regla, fuente y error, nunca
// el contexto ni la salida.
func (s *Service) redactar(ctx context.Context, owner bson.ObjectID, categoria string, h Hecho, dia string) (string, string) {
	if s.ia == nil || s.cupo <= 0 {
		return Plantilla(h), FuenteReglas
	}
	usados, err := s.repo.ConIADelDia(ctx, dia)
	if err != nil || usados >= int64(s.cupo) {
		return Plantilla(h), FuenteReglas
	}
	var out salidaConsejo
	pet := ia.Peticion{Instrucciones: InstruccionesConsejo, Contexto: Contexto(h, categoria), Esquema: esquemaConsejo, MaxTokens: 200}
	if _, err := s.ia.GenerarValidado(ia.ConUsuario(ctx, owner), pet, &out); err != nil {
		s.log.WarnContext(ctx, "consejos: sin texto de la IA", "evento", "consejo_ia_fallo", "regla", h.Regla, "fuente", FuenteReglas, "error", err.Error())
		return Plantilla(h), FuenteReglas
	}
	if out.Accion != h.Accion {
		s.log.WarnContext(ctx, "consejos: la IA cambió la acción", "evento", "consejo_ia_fallo", "regla", h.Regla, "fuente", FuenteReglas, "error", "accion_distinta")
		return Plantilla(h), FuenteReglas
	}
	return out.Texto, FuenteIA
}

// InstruccionesConsejo son fijas y versionadas (v1); nunca llevan datos del negocio.
const InstruccionesConsejo = `v1. Eres un asesor de negocios locales en una app de turismo en México. Redacta en español, con tuteo y tono amable, un consejo breve (20 a 160 caracteres) para el dueño a partir del hecho del contexto. Usa solo las cifras del contexto y no prometas resultados. Sin emojis, nombres de personas, correos, teléfonos ni enlaces. Devuelve en "accion" exactamente la acción del contexto. Responde solo el JSON del esquema.`

var esquemaConsejo = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"texto":  map[string]any{"type": "string", "minLength": MinTextoIA, "maxLength": MaxTextoIA},
		"accion": map[string]any{"type": "string", "enum": Acciones},
	},
	"required":             []string{"texto", "accion"},
	"additionalProperties": false,
}

// salidaConsejo es la salida del modelo; Validar la revisa antes de aceptarla.
type salidaConsejo struct {
	Texto  string `json:"texto"`
	Accion string `json:"accion"`
}

// Validar exige 20–160 caracteres sin datos personales y una acción de la lista.
func (o *salidaConsejo) Validar() error {
	o.Texto = strings.Join(strings.Fields(o.Texto), " ")
	n := utf8.RuneCountInString(o.Texto)
	if n < MinTextoIA || n > MaxTextoIA {
		return errors.New("el consejo debe tener de 20 a 160 caracteres")
	}
	if ia.Anonimizar(o.Texto) != o.Texto {
		return errors.New("el consejo trae datos personales")
	}
	if !slices.Contains(Acciones, o.Accion) {
		return errors.New("acción fuera de la lista")
	}
	return nil
}

func respuesta(c Consejo) ConsejoResponse {
	return ConsejoResponse{Texto: c.Texto, Accion: c.Accion, Fuente: c.Fuente, Dia: c.Dia}
}

// Observador borra los consejos del negocio al darlo de baja (nv-56); pausa y reanudación no
// cambian nada.
type Observador struct{ repo Store }

// NewObservador recibe el repositorio.
func NewObservador(repo Store) Observador { return Observador{repo: repo} }

// Pausado cumple negocios.Observador.
func (Observador) Pausado(context.Context, bson.ObjectID) error { return nil }

// Reanudado cumple negocios.Observador.
func (Observador) Reanudado(context.Context, bson.ObjectID) error { return nil }

// DadoDeBaja borra los consejos del negocio.
func (o Observador) DadoDeBaja(ctx context.Context, negocio bson.ObjectID) error {
	return o.repo.BorrarNegocio(ctx, negocio)
}
