package insignias

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Progreso es el avance hacia la meta de una insignia.
type Progreso struct {
	Actual int `json:"actual"`
	Meta   int `json:"meta"`
}

// Insignia es una insignia del catálogo vista por su dueña: ganada o bloqueada.
type Insignia struct {
	ID          string     `json:"id"`
	Nombre      string     `json:"nombre"`
	Descripcion string     `json:"descripcion"`
	Ganada      bool       `json:"ganada"`
	GanadaEn    *time.Time `json:"ganada_en"`
	Progreso    Progreso   `json:"progreso"`
}

// Ganada es una insignia que otra persona puede ver: solo las ganadas, sin progreso.
type Ganada struct {
	ID       string    `json:"id"`
	Nombre   string    `json:"nombre"`
	GanadaEn time.Time `json:"ganada_en"`
}

// NivelResponse es el nivel con sus puntos y el siguiente escalón.
type NivelResponse struct {
	Numero    int    `json:"numero"`
	Nombre    string `json:"nombre"`
	Puntos    int    `json:"puntos"`
	Siguiente *Nivel `json:"siguiente,omitempty"`
}

// MiasResponse es GET /usuarios/yo/insignias.
type MiasResponse struct {
	Insignias    []Insignia    `json:"insignias"`
	Ganadas      int           `json:"ganadas"`
	Total        int           `json:"total"`
	Nivel        NivelResponse `json:"nivel"`
	RachaSemanas int           `json:"racha_semanas"`
}

// Service concilia insignias, puntos, nivel y racha.
type Service struct {
	repo *Repository
	now  func() time.Time
	log  *slog.Logger
}

// NewService recibe el repositorio por constructor.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, now: time.Now, log: log}
}

// WithClock fija el reloj (pruebas de racha y de planes por semana).
func (s *Service) WithClock(now func() time.Time) *Service {
	cp := *s
	cp.now = now
	return &cp
}

func errStore() error { return apperr.Unavailable("servicio no disponible") }

// insigniasDe lee la actividad, cuenta, guarda las insignias recién ganadas y devuelve todo.
func (s *Service) insigniasDe(ctx context.Context, userID bson.ObjectID) (Actividad, map[string]int, map[string]time.Time, error) {
	a, err := s.repo.Actividad(ctx, userID)
	if err != nil {
		return a, nil, nil, err
	}
	fechas, err := s.repo.Ganadas(ctx, userID)
	if err != nil {
		return a, nil, nil, err
	}
	dias := make([]string, len(a.Visitas))
	lugares := map[bson.ObjectID]bool{}
	for i, v := range a.Visitas {
		dias[i] = v.Dia
		lugares[v.NegocioID] = true
	}
	conteos := map[string]int{
		FuenteCafeterias: a.Cafeterias, FuenteResenas: len(a.Resenas), FuenteGrupos: a.Grupos,
		FuenteLugares: len(lugares), FuentePostales: len(a.Postales), FuenteRacha: Racha(dias, s.now()),
		FuenteMeGusta: 0, // llega con nv-83
	}
	ahora := s.now().UTC().Truncate(time.Millisecond)
	for _, d := range Catalogo {
		if _, ya := fechas[d.ID]; ya || !d.Ganada(conteos) {
			continue
		}
		if err := s.repo.Guardar(ctx, userID, d.ID, ahora); err != nil {
			return a, nil, nil, err
		}
		fechas[d.ID] = ahora
		s.log.InfoContext(ctx, "insignia ganada", "evento", "insignia", "insignia", d.ID)
	}
	return a, conteos, fechas, nil
}

// Estado es lo conciliado de una persona.
type Estado struct {
	Conteos map[string]int
	Fechas  map[string]time.Time
	Puntos  int
	Nivel   Nivel
	Racha   int
}

// Conciliar registra los movimientos que falten, suma con `$inc` solo los nuevos y guarda
// nivel y racha en `usuarios`. Es idempotente y seguro con peticiones a la vez: el índice
// `ux_movimiento` deja pasar un solo movimiento por acción.
func (s *Service) Conciliar(ctx context.Context, userID bson.ObjectID) (Estado, error) {
	a, conteos, fechas, err := s.insigniasDe(ctx, userID)
	if err != nil {
		return Estado{}, err
	}
	suma, err := s.registrar(ctx, userID, Esperados(userID, a, fechas))
	if err != nil {
		return Estado{}, err
	}
	perfil, err := s.repo.SumarPuntos(ctx, userID, suma)
	if err != nil {
		return Estado{}, err
	}
	nivel := NivelDe(perfil.Puntos)
	racha := conteos[FuenteRacha]
	if perfil.Nivel != nivel.Nombre || perfil.NivelNumero != nivel.Numero || perfil.Racha != racha {
		perfil.Nivel, perfil.NivelNumero, perfil.Racha = nivel.Nombre, nivel.Numero, racha
		if err := s.repo.GuardarNivel(ctx, userID, perfil); err != nil {
			return Estado{}, err
		}
	}
	if suma > 0 {
		s.log.InfoContext(ctx, "puntos sumados", "evento", "puntos", "puntos", suma, "nivel", nivel.Numero)
	}
	return Estado{Conteos: conteos, Fechas: fechas, Puntos: perfil.Puntos, Nivel: nivel, Racha: racha}, nil
}

// registrar inserta los movimientos que aún no existen y devuelve cuánto suman los que esta
// petición sí insertó (los que otra ganó por el índice único no cuentan).
func (s *Service) registrar(ctx context.Context, userID bson.ObjectID, esperados []Pendiente) (int, error) {
	hechos, err := s.repo.Movimientos(ctx, userID)
	if err != nil {
		return 0, err
	}
	ya := make(map[clave]bool, len(hechos))
	for _, m := range hechos {
		ya[clave{m.Motivo, m.RefID}] = true
	}
	ahora := s.now().UTC().Truncate(time.Millisecond)
	suma := 0
	for _, p := range esperados {
		if ya[clave{p.Motivo, p.RefID}] {
			continue
		}
		nuevo, err := s.repo.Registrar(ctx, Movimiento{ID: bson.NewObjectID(), UsuarioID: userID, Motivo: p.Motivo, RefID: p.RefID, Puntos: PuntosPor[p.Motivo], CreadoEn: ahora})
		if err != nil {
			return 0, err
		}
		if nuevo && sumaAlCrear(p.Motivo) {
			suma += PuntosPor[p.Motivo]
		}
	}
	return suma, nil
}

// ConciliarPerfil es la conciliación de GET /usuarios/yo (account.Conciliador).
func (s *Service) ConciliarPerfil(ctx context.Context, userID bson.ObjectID) error {
	_, err := s.Conciliar(ctx, userID)
	return err
}

// Mias da el catálogo completo para la dueña: ganadas con su fecha y bloqueadas con su
// avance, más nivel y racha. Una insignia ya ganada se conserva aunque el conteo baje.
func (s *Service) Mias(ctx context.Context, subject string) (MiasResponse, error) {
	userID, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return MiasResponse{}, apperr.Unauthorized("autenticación requerida")
	}
	e, err := s.Conciliar(ctx, userID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return MiasResponse{}, apperr.Unauthorized("autenticación requerida")
		}
		return MiasResponse{}, errStore()
	}
	sig := NivelNumero(e.Nivel.Numero + 1)
	out := MiasResponse{
		Insignias: make([]Insignia, 0, len(Catalogo)), Total: len(Catalogo), RachaSemanas: e.Racha,
		Nivel: NivelResponse{Numero: e.Nivel.Numero, Nombre: e.Nivel.Nombre, Puntos: e.Puntos, Siguiente: &sig},
	}
	for _, d := range Catalogo {
		it := Insignia{ID: d.ID, Nombre: d.Nombre, Descripcion: d.Descripcion, Progreso: Progreso{Actual: d.Progreso(e.Conteos), Meta: d.Meta}}
		if f, ok := e.Fechas[d.ID]; ok {
			f := f.UTC()
			it.Ganada, it.GanadaEn, it.Progreso.Actual = true, &f, d.Meta
			out.Ganadas++
		}
		out.Insignias = append(out.Insignias, it)
	}
	return out, nil
}

// GanadasDe da solo las insignias ganadas de otra persona, en el orden del catálogo. Quien
// llama ya comprobó que puede ver su perfil (personas, nv-70). No mueve puntos: eso solo pasa
// cuando la dueña los pide o en el trabajo nocturno.
func (s *Service) GanadasDe(ctx context.Context, userID bson.ObjectID) ([]Ganada, error) {
	_, _, fechas, err := s.insigniasDe(ctx, userID)
	if err != nil {
		return nil, errStore()
	}
	out := []Ganada{}
	for _, d := range Catalogo {
		if f, ok := fechas[d.ID]; ok {
			out = append(out, Ganada{ID: d.ID, Nombre: d.Nombre, GanadaEn: f.UTC()})
		}
	}
	return out, nil
}
