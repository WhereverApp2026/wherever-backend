package vistas

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// retryAfterHora es el `Retry-After` del 429 por pasar el tope de lugares por hora.
const retryAfterHora = 3600

// Places es lo que vistas lee de negocios.
type Places interface {
	FindVisible(ctx context.Context, id bson.ObjectID) (negocios.Business, bool, error)
	FindCards(ctx context.Context, ids []bson.ObjectID, now time.Time) ([]negocios.PlaceItem, error)
}

// Contador suma la vista contada a las estadísticas del negocio (nv-74). Solo se llama cuando
// la vista se escribe (pasó la ventana de 10 min).
type Contador interface {
	Contar(ctx context.Context, negocio bson.ObjectID, origen string, now time.Time) error
}

// Service maneja las vistas (nv-66).
type Service struct {
	repo     *Repository
	places   Places
	contador Contador
	log      *slog.Logger
	now      func() time.Time
}

// NewService recibe sus dependencias por constructor.
func NewService(repo *Repository, places Places, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, places: places, log: log, now: time.Now}
}

// WithContador conecta las estadísticas del negocio (nv-74).
func (s *Service) WithContador(c Contador) *Service {
	s.contador = c
	return s
}

// VistaRequest es el cuerpo de POST /lugares/{id}/vistas; `origen` es opcional.
type VistaRequest struct {
	Origen string `json:"origen" validate:"omitempty,oneof=busqueda mapa inicio plan_ia cupon"`
}

// RecientesQuery es GET /lugares/recientes.
type RecientesQuery struct {
	Limit int `query:"limite" validate:"min=1,max=50"`
}

// Reciente es la tarjeta de GET /lugares más cuándo la vio.
type Reciente struct {
	negocios.PlaceItem
	VistoEn time.Time `json:"visto_en"`
}

// RecientesResponse es GET /lugares/recientes.
type RecientesResponse struct {
	Items []Reciente `json:"items"`
}

func errStore() error { return apperr.Unavailable("servicio no disponible") }

func self(subject string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return id, apperr.Unauthorized("autenticación requerida")
	}
	return id, nil
}

// Registrar guarda que la persona abrió la ficha de un lugar visible (si no, 404). Antes de 10
// min no escribe; más de 60 lugares distintos en una hora = 429. Los logs nunca juntan persona
// y lugar.
func (s *Service) Registrar(ctx context.Context, subject, rawID string, req VistaRequest) error {
	me, err := self(subject)
	if err != nil {
		return err
	}
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return apperr.NotFound("lugar no encontrado")
	}
	if _, ok, err := s.places.FindVisible(ctx, id); err != nil {
		return errStore()
	} else if !ok {
		return apperr.NotFound("lugar no encontrado")
	}
	now := s.now().UTC()
	n, err := s.repo.OtrosEnLaHora(ctx, me, id, now)
	if err != nil {
		return errStore()
	}
	if n >= MaxPerHour {
		return apperr.RateLimited(retryAfterHora)
	}
	escrita, err := s.repo.Registrar(ctx, me, id, req.Origen, now)
	if err != nil {
		return errStore()
	}
	if escrita {
		s.contar(ctx, id, req.Origen, now)
	}
	return nil
}

// contar suma la vista a las estadísticas del negocio; si falla, la vista ya quedó guardada y
// solo se registra el evento, sin persona ni lugar.
func (s *Service) contar(ctx context.Context, negocio bson.ObjectID, origen string, now time.Time) {
	if s.contador == nil {
		return
	}
	if err := s.contador.Contar(ctx, negocio, origen, now); err != nil {
		s.log.WarnContext(ctx, "vistas: vista sin contar en estadísticas", "evento", "vista_sin_contar")
	}
}

// Recientes devuelve las tarjetas de los lugares vistos, del más reciente al más viejo; los que
// dejaron de ser visibles se omiten.
func (s *Service) Recientes(ctx context.Context, subject string, q RecientesQuery) (RecientesResponse, error) {
	me, err := self(subject)
	if err != nil {
		return RecientesResponse{}, err
	}
	rows, err := s.repo.Recientes(ctx, me, int64(q.Limit))
	if err != nil {
		return RecientesResponse{}, errStore()
	}
	ids := make([]bson.ObjectID, 0, len(rows))
	for _, v := range rows {
		ids = append(ids, v.NegocioID)
	}
	cards, err := s.places.FindCards(ctx, ids, s.now())
	if err != nil {
		return RecientesResponse{}, errStore()
	}
	byID := make(map[string]negocios.PlaceItem, len(cards))
	for _, c := range cards {
		byID[c.ID] = c
	}
	out := RecientesResponse{Items: make([]Reciente, 0, len(rows))}
	for _, v := range rows {
		if c, ok := byID[v.NegocioID.Hex()]; ok {
			out.Items = append(out.Items, Reciente{PlaceItem: c, VistoEn: v.Fecha})
		}
	}
	return out, nil
}

// Borrar borra todas las vistas de la persona (no toca búsquedas ni descartes).
func (s *Service) Borrar(ctx context.Context, subject string) error {
	me, err := self(subject)
	if err != nil {
		return err
	}
	if err := s.repo.BorrarTodo(ctx, me); err != nil {
		return errStore()
	}
	return nil
}
