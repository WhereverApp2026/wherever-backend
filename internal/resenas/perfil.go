package resenas

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/contadores"
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

// WithContadores invalida los contadores del perfil al crear o borrar una reseña (nv-81).
func (s *Service) WithContadores(c *contadores.Service) *Service {
	s.contadores = c
	return s
}

// WithPerfiles conecta la pestaña de reseñas del perfil; sin ella la ruta responde 503.
func (s *Service) WithPerfiles(p Perfiles) *Service {
	s.perfiles = p
	return s
}

// NegocioView es el negocio de una reseña en el perfil.
type NegocioView struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
	Slug   string `json:"slug"`
}

// PersonaResenaResponse es una reseña en el perfil de su autora: el negocio en lugar del
// autor, sin `checkin_id` ni datos de contacto.
type PersonaResenaResponse struct {
	ID               string         `json:"id"`
	Negocio          NegocioView    `json:"negocio"`
	Calificacion     int            `json:"calificacion"`
	Aspectos         Aspectos       `json:"aspectos"`
	Chips            []string       `json:"chips"`
	Comentario       string         `json:"comentario"`
	Fotos            []Foto         `json:"fotos"`
	Util             int            `json:"util"`
	RespuestaNegocio *RespuestaView `json:"respuesta_negocio"`
	CreadoEn         time.Time      `json:"creado_en"`
}

// DePersona pagina las reseñas publicadas de otra persona, de la más nueva a la más vieja, con
// el negocio y su respuesta publicada. Su privacidad (nv-82) puede dejar la página vacía; las
// reglas del perfil dan 404. Una reseña de un negocio que ya no está visible no sale.
func (s *Service) DePersona(ctx context.Context, subject, rawID string, q ListaQuery) (pagination.PageResult[PersonaResenaResponse], error) {
	var vacio pagination.PageResult[PersonaResenaResponse]
	if s.perfiles == nil {
		return vacio, apperr.Unavailable("servicio no disponible")
	}
	v, err := s.perfiles.Pestana(ctx, subject, rawID, personas.PestanaResenas)
	if err != nil {
		return vacio, err
	}
	me, _ := bson.ObjectIDFromHex(subject) // Pestana ya lo validó
	scope := s.cursor.Scope("resenas_persona", me.Hex(), v.Autor.Hex())
	after, err := s.posicion(scope, q.Cursor)
	if err != nil {
		return vacio, err
	}
	if !v.Visible {
		return pagination.NewPageResult([]PersonaResenaResponse{}, "", q.Limite), nil
	}
	rs, err := s.repo.DePersona(ctx, v.Autor, after, q.Limite)
	if err != nil {
		return vacio, errStore()
	}
	next := ""
	if len(rs) > q.Limite {
		rs = rs[:q.Limite]
		last := rs[len(rs)-1]
		if next, err = s.siguiente(scope, last.ID, fechaCursor(last.CreadoEn)); err != nil {
			return vacio, err
		}
	}
	items, err := s.conNegocio(ctx, rs)
	if err != nil {
		return vacio, err
	}
	return pagination.NewPageResult(items, next, q.Limite), nil
}

// conNegocio arma cada reseña con su negocio visible.
func (s *Service) conNegocio(ctx context.Context, rs []Resena) ([]PersonaResenaResponse, error) {
	ids := make([]bson.ObjectID, 0, len(rs))
	for _, r := range rs {
		ids = append(ids, r.NegocioID)
	}
	out := make([]PersonaResenaResponse, 0, len(rs))
	if len(ids) == 0 {
		return out, nil
	}
	bs, err := s.negocios.FindVisibleMany(ctx, ids)
	if err != nil {
		return nil, errStore()
	}
	byID := make(map[bson.ObjectID]NegocioView, len(bs))
	for _, b := range bs {
		byID[b.ID] = NegocioView{ID: b.ID.Hex(), Nombre: b.Name, Slug: b.Slug}
	}
	for _, r := range rs {
		n, ok := byID[r.NegocioID]
		if !ok {
			continue
		}
		item := PersonaResenaResponse{
			ID: r.ID.Hex(), Negocio: n, Calificacion: r.Calificacion, Aspectos: r.Aspectos, Chips: noNil(r.Chips),
			Comentario: r.Comentario, Fotos: fotosNoNil(r.Fotos), Util: r.Util, CreadoEn: r.CreadoEn.UTC(),
		}
		if rn := r.RespuestaNegocio; rn != nil && rn.Estado == EstadoPublicada {
			item.RespuestaNegocio = &RespuestaView{Texto: rn.Texto, Fecha: rn.Fecha.UTC()}
		}
		out = append(out, item)
	}
	return out, nil
}

// DePersona trae hasta limit+1 reseñas publicadas de la persona (ix_mis_resenas).
func (r *Repository) DePersona(ctx context.Context, usuario bson.ObjectID, after *Pos, limit int) ([]Resena, error) {
	filter := bson.D{{Key: "usuario_id", Value: usuario}, {Key: "estado", Value: EstadoPublicada}}
	if after != nil {
		filter = append(filter, despues("creado_en", after.Fecha, after.ID)...)
	}
	return listar[Resena](ctx, r.resenas, filter, bson.D{{Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}}, limit+1)
}

// personaRoutes registra GET /personas/{id}/resenas con el límite del perfil (nv-70).
func (h *Handler) personaRoutes(v1 fiber.Router, limiter *middleware.Limiter) {
	chain := []fiber.Handler{middleware.RequireAuth()}
	if h.active != nil {
		chain = append(chain, h.active)
	}
	if limiter != nil {
		chain = append(chain, limiter.WindowUser(personas.PoliticaPerfil, personas.LimitePerfil, personas.VentanaPerfil))
	}
	v1.Get("/personas/:id/resenas", append(chain, h.dePersona)...)
}

func (h *Handler) dePersona(c *fiber.Ctx) error {
	q := NewListaQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	subject := ""
	if cl := claims(c); cl != nil {
		subject = cl.Subject
	}
	out, err := h.svc.DePersona(c.UserContext(), subject, c.Params("id"), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}
