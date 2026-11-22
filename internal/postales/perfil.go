package postales

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/personas"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// Perfiles aplica las reglas del perfil de otra persona a su pestaña (lo cumple
// *personas.Service, nv-81).
type Perfiles interface {
	Pestana(ctx context.Context, subject, rawID, pestana string) (personas.Vista, error)
}

// WithPerfiles conecta la pestaña de momentos del perfil; sin ella la ruta responde 503.
func (s *Service) WithPerfiles(p Perfiles) *Service {
	s.perfiles = p
	return s
}

// visiblesPara son las visibilidades de momento que ve quien mira: amigos, `amigos` y
// `publica`; los demás, solo `publica`.
func visiblesPara(amigo bool) []string {
	if amigo {
		return []string{VisPublica, VisAmigos}
	}
	return []string{VisPublica}
}

// MomentosDePersona pagina los momentos publicados de otra persona que quien mira puede ver,
// del más nuevo al más viejo. Su privacidad (nv-82) puede dejar la página vacía; las reglas
// del perfil dan 404.
func (s *Service) MomentosDePersona(ctx context.Context, cl *middleware.Claims, rawID string, q ListaQuery) (pagination.PageResult[MomentoResponse], error) {
	var vacio pagination.PageResult[MomentoResponse]
	if s.perfiles == nil {
		return vacio, apperr.Unavailable("servicio no disponible")
	}
	subject := ""
	if cl != nil {
		subject = cl.Subject
	}
	v, err := s.perfiles.Pestana(ctx, subject, rawID, personas.PestanaMomentos)
	if err != nil {
		return vacio, err
	}
	me, _ := bson.ObjectIDFromHex(subject) // Pestana ya lo validó
	scope := s.Cursor.Scope("momentos_persona", me.Hex(), v.Autor.Hex())
	after, err := s.posicion(scope, q.Cursor)
	if err != nil {
		return vacio, err
	}
	if !v.Visible {
		return pagination.NewPageResult([]MomentoResponse{}, "", q.Limite), nil
	}
	autores, err := s.autores(ctx, []bson.ObjectID{v.Autor}, &me)
	if err != nil {
		return vacio, err
	}
	rows, err := s.Repo.DePersona(ctx, v.Autor, visiblesPara(v.Amigo), after, q.Limite)
	if err != nil {
		return vacio, errStore()
	}
	return s.paginaMomentos(ctx, rows, q.Limite, scope, autores, me)
}

// DePersona pagina los momentos publicados del autor con esas visibilidades (ix_feed).
func (r *Repository) DePersona(ctx context.Context, autor bson.ObjectID, visibilidades []string, after *Pos, limit int) ([]Momento, error) {
	filter := bson.D{
		{Key: "autor_id", Value: autor},
		{Key: "estado", Value: MomentoPublicado},
		{Key: "visibilidad", Value: bson.D{{Key: "$in", Value: visibilidades}}},
	}
	if after != nil {
		filter = append(filter, despues("creado_en", after))
	}
	return listar[Momento](ctx, r.momentos, filter, porFecha("creado_en"), limit+1)
}

// personaRoutes registra GET /personas/{id}/momentos con el límite del perfil (nv-70).
func (h *Handler) personaRoutes(v1 fiber.Router, limiter *middleware.Limiter) {
	chain := []fiber.Handler{middleware.RequireAuth()}
	if h.active != nil {
		chain = append(chain, h.active)
	}
	if limiter != nil {
		chain = append(chain, limiter.WindowUser(personas.PoliticaPerfil, personas.LimitePerfil, personas.VentanaPerfil))
	}
	v1.Get("/personas/:id/momentos", append(chain, h.momentosDePersona)...)
}

func (h *Handler) momentosDePersona(c *fiber.Ctx) error {
	q := NewListaQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.MomentosDePersona(c.UserContext(), claims(c), c.Params("id"), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}
