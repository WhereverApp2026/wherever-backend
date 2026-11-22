package planes

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/contadores"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/personas"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// Perfiles aplica las reglas del perfil de otra persona a su pestaña (lo cumple
// *personas.Service, nv-81).
type Perfiles interface {
	Pestana(ctx context.Context, subject, rawID, pestana string) (personas.Vista, error)
}

// WithContadores invalida los contadores del perfil con cada cambio de un plan (nv-81).
func (s *Service) WithContadores(c *contadores.Service) *Service {
	s.contadores = c
	return s
}

// WithPerfiles conecta la pestaña de planes del perfil; sin ella la ruta responde 503.
func (s *Service) WithPerfiles(p Perfiles) *Service {
	s.perfiles = p
	return s
}

// PersonaQuery es `?cursor=&limite=` de GET /personas/{id}/planes.
type PersonaQuery struct {
	Cursor string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limit  int    `query:"limite" validate:"min=1,max=50"`
}

// NewPersonaQuery trae 20 por página.
func NewPersonaQuery() PersonaQuery { return PersonaQuery{Limit: pagination.DefaultLimit} }

// tabPersona es la pestaña con que se firma el cursor de esta lista.
const tabPersona = "persona"

// DePersona pagina los planes `publico` activos de otra persona, de la fecha más nueva a la
// más vieja, sin dueño, integrantes, notas ni grupo. Su privacidad (nv-82) puede dejar la
// página vacía; las reglas del perfil dan 404.
func (s *Service) DePersona(ctx context.Context, subject, rawID string, q PersonaQuery) (pagination.PageResult[PublicPlan], error) {
	var vacio pagination.PageResult[PublicPlan]
	if s.perfiles == nil || s.extras.places == nil {
		return vacio, apperr.Unavailable("servicio no disponible")
	}
	v, err := s.perfiles.Pestana(ctx, subject, rawID, personas.PestanaPlanes)
	if err != nil {
		return vacio, err
	}
	me, _ := owner(subject) // Pestana ya lo validó
	scope := s.cursor.Scope("planes_persona", me.Hex(), v.Autor.Hex())
	after, err := position(s.cursor, scope, tabPersona, q.Cursor)
	if err != nil {
		return vacio, err
	}
	if !v.Visible {
		return pagination.NewPageResult([]PublicPlan{}, "", q.Limit), nil
	}
	rows, err := s.repo.List(ctx, PageQuery{Owner: v.Autor, Public: true, Desc: true, After: after, Limit: q.Limit})
	if err != nil {
		return vacio, errStore()
	}
	next := ""
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		last := rows[len(rows)-1]
		next, err = s.cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: tabPersona + ":" + last.Fecha.UTC().Format(time.RFC3339Nano), Scope: scope})
		if err != nil {
			return vacio, errStore()
		}
	}
	items, err := s.publicPlans(ctx, rows)
	if err != nil {
		return vacio, err
	}
	return pagination.NewPageResult(items, next, q.Limit), nil
}

// publicPlans arma los planes públicos con la ficha corta de cada lugar.
func (s *Service) publicPlans(ctx context.Context, rows []Plan) ([]PublicPlan, error) {
	ids := []bson.ObjectID{}
	for _, p := range rows {
		for _, st := range p.Paradas {
			ids = append(ids, st.NegocioID)
		}
	}
	cards, err := s.extras.places.FindCards(ctx, ids, s.now())
	if err != nil {
		return nil, errStore()
	}
	byID := make(map[bson.ObjectID]negocios.PlaceItem, len(cards))
	for _, c := range cards {
		if id, err := bson.ObjectIDFromHex(c.ID); err == nil {
			byID[id] = c
		}
	}
	out := make([]PublicPlan, 0, len(rows))
	for _, p := range rows {
		out = append(out, newPublicPlan(p, byID))
	}
	return out, nil
}

// personaRoutes registra GET /personas/{id}/planes con el límite del perfil (nv-70).
func (h *Handler) personaRoutes(v1 fiber.Router, limiter *middleware.Limiter) {
	chain := []fiber.Handler{middleware.RequireAuth()}
	if h.active != nil {
		chain = append(chain, h.active)
	}
	if limiter != nil {
		chain = append(chain, limiter.WindowUser(personas.PoliticaPerfil, personas.LimitePerfil, personas.VentanaPerfil))
	}
	v1.Get("/personas/:id/planes", append(chain, h.dePersona)...)
}

func (h *Handler) dePersona(c *fiber.Ctx) error {
	q := NewPersonaQuery()
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.DePersona(c.UserContext(), subject(c), c.Params("id"), q)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}
