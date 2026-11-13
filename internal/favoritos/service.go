package favoritos

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Businesses es lo que favoritos lee de negocios.
type Businesses interface {
	FindVisible(ctx context.Context, id bson.ObjectID) (negocios.Business, bool, error)
	FindFavoriteCards(ctx context.Context, ids []bson.ObjectID) ([]negocios.Business, error)
}

// Plans es lo que favoritos lee de planes: el plan propio y cuáles siguen activos.
type Plans interface {
	FindOwned(ctx context.Context, owner, id bson.ObjectID) (planes.Plan, error)
	ActiveIDs(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID]bool, error)
}

// Service maneja los favoritos (nv-54).
type Service struct {
	repo       *Repository
	businesses Businesses
	plans      Plans
	cursor     *pagination.Signer
	now        func() time.Time
}

// NewService recibe sus dependencias por constructor; sin cursor la lista falla cerrada.
func NewService(repo *Repository, businesses Businesses, plans Plans, cursor *pagination.Signer) *Service {
	return &Service{repo: repo, businesses: businesses, plans: plans, cursor: cursor, now: time.Now}
}

// Errores públicos: se crean en cada llamada porque la etapa de errores puede anotarlos.
func errTargetNotFound() error { return apperr.NotFound("objetivo no encontrado") }
func errLimit() error          { return apperr.Conflict("límite de favoritos: máximo 500 por tipo") }

// errStore no lleva el texto de MongoDB: podría repetir ids o nombres.
func errStore() error { return apperr.Unavailable("servicio no disponible") }

func self(subject string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("autenticación requerida")
	}
	return id, nil
}

// Mark marca un negocio aprobado o un plan propio. Es idempotente: repetirlo devuelve el
// mismo favorito. Un objetivo no válido da 404 sin decir por qué.
func (s *Service) Mark(ctx context.Context, subject string, p TargetParams) (View, error) {
	me, err := self(subject)
	if err != nil {
		return View{}, err
	}
	target, err := bson.ObjectIDFromHex(p.TargetID)
	if err != nil {
		return View{}, errTargetNotFound()
	}
	summary, card, err := s.target(ctx, me, p.Type, target)
	if err != nil {
		return View{}, err
	}
	existing, ok, err := s.repo.Find(ctx, me, p.Type, target)
	if err != nil {
		return View{}, errStore()
	}
	if ok {
		return s.view(existing, card), nil
	}
	n, err := s.repo.Count(ctx, me, p.Type)
	if err != nil {
		return View{}, errStore()
	}
	if n >= MaxPerType {
		return View{}, errLimit()
	}
	f, err := s.repo.Upsert(ctx, Favorite{
		ID: bson.NewObjectID(), UserID: me, Type: p.Type, TargetID: target, Summary: summary,
		CreatedAt: s.now().UTC().Truncate(time.Millisecond),
	})
	if err != nil {
		return View{}, errStore()
	}
	return s.view(f, card), nil
}

// target valida el objetivo: negocio aprobado sin `eliminado_en` o plan activo propio.
func (s *Service) target(ctx context.Context, me bson.ObjectID, kind string, id bson.ObjectID) (Summary, *negocios.Business, error) {
	if kind == TypePlan {
		p, err := s.plans.FindOwned(ctx, me, id)
		if errors.Is(err, planes.ErrNotFound) {
			return Summary{}, nil, errTargetNotFound()
		}
		if err != nil {
			return Summary{}, nil, errStore()
		}
		return Summary{Name: p.Nombre}, nil, nil
	}
	b, ok, err := s.businesses.FindVisible(ctx, id)
	if err != nil {
		return Summary{}, nil, errStore()
	}
	if !ok {
		return Summary{}, nil, errTargetNotFound()
	}
	return businessSummary(b), &b, nil
}

func businessSummary(b negocios.Business) Summary {
	return Summary{Name: b.Name, PhotoURL: negocios.Cover(b.Photos), Category: b.Category}
}

// view arma la vista; con el negocio leído refresca el resumen y agrega cupón y horario.
func (s *Service) view(f Favorite, b *negocios.Business) View {
	sum := f.Summary
	v := View{Type: f.Type, TargetID: f.TargetID.Hex(), CreatedAt: f.CreatedAt}
	if b != nil {
		sum = businessSummary(*b)
		coupon, open := b.Coupons > 0, negocios.OpenAt(b.ScheduleMin, s.now())
		v.CouponToday, v.OpenNow = &coupon, &open
	}
	v.Summary = SummaryView(sum)
	return v
}

// Unmark quita el favorito; si no estaba también responde bien (204).
func (s *Service) Unmark(ctx context.Context, subject string, p TargetParams) error {
	me, err := self(subject)
	if err != nil {
		return err
	}
	target, err := bson.ObjectIDFromHex(p.TargetID)
	if err != nil {
		return nil
	}
	if err := s.repo.Delete(ctx, me, p.Type, target); err != nil {
		return errStore()
	}
	return nil
}

// List devuelve una pestaña del más reciente al más viejo. Oculta los negocios no aprobados
// y borra los favoritos cuyo objetivo ya no existe. Con `con_cupon` la página puede traer
// menos de `limite`; el cursor sale de la última fila leída, así que sigue siendo correcto.
func (s *Service) List(ctx context.Context, subject string, q ListQuery) (pagination.PageResult[View], error) {
	me, err := self(subject)
	if err != nil {
		return pagination.PageResult[View]{}, err
	}
	scope := s.cursor.Scope("favoritos", me.Hex(), q.Type)
	after, err := position(s.cursor, scope, q.Cursor)
	if err != nil {
		return pagination.PageResult[View]{}, err
	}
	rows, err := s.repo.List(ctx, me, q.Type, after, q.Limit)
	if err != nil {
		return pagination.PageResult[View]{}, errStore()
	}
	next := ""
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		last := rows[len(rows)-1]
		next, err = s.cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: last.CreatedAt.UTC().Format(time.RFC3339Nano), Scope: scope})
		if err != nil {
			return pagination.PageResult[View]{}, errStore()
		}
	}
	views, err := s.visible(ctx, me, rows)
	if err != nil {
		return pagination.PageResult[View]{}, err
	}
	if q.WithCoupon {
		views = withCoupon(views)
	}
	return pagination.NewPageResult(views, next, q.Limit), nil
}

func withCoupon(views []View) []View {
	out := []View{}
	for _, v := range views {
		if v.CouponToday != nil && *v.CouponToday {
			out = append(out, v)
		}
	}
	return out
}

// IDs devuelve los objetivos de un tipo (hasta 500), sin paginar ni caché.
func (s *Service) IDs(ctx context.Context, subject string, q IDsQuery) (IDsView, error) {
	me, err := self(subject)
	if err != nil {
		return IDsView{}, err
	}
	ids, err := s.repo.IDs(ctx, me, q.Type)
	if err != nil {
		return IDsView{}, errStore()
	}
	out := IDsView{Type: q.Type, IDs: make([]string, 0, len(ids))}
	for _, id := range ids {
		out.IDs = append(out.IDs, id.Hex())
	}
	return out, nil
}

// Search busca por prefijo del nombre solo entre mis favoritos (hasta 500 por tipo, en
// memoria) y muestra lo mismo que la lista.
func (s *Service) Search(ctx context.Context, subject string, q SearchQuery) (SearchView, error) {
	me, err := self(subject)
	if err != nil {
		return SearchView{}, err
	}
	rows, err := s.repo.All(ctx, me, q.Kinds()...)
	if err != nil {
		return SearchView{}, errStore()
	}
	matches := make([]Favorite, 0, len(rows))
	for _, f := range rows {
		if MatchesPrefix(f.Summary.Name, q.Q) {
			matches = append(matches, f)
		}
	}
	views, err := s.visible(ctx, me, matches)
	if err != nil {
		return SearchView{}, err
	}
	return SearchView{Items: views}, nil
}

// visible arma las vistas en el orden dado: un negocio no aprobado se omite sin borrar el
// favorito; un objetivo que ya no existe (plan borrado, negocio con `eliminado_en` o id
// ausente) se omite y su favorito se borra.
func (s *Service) visible(ctx context.Context, me bson.ObjectID, rows []Favorite) ([]View, error) {
	shops, activePlans, err := s.targets(ctx, rows)
	if err != nil {
		return nil, err
	}
	views := []View{}
	gone := map[string][]bson.ObjectID{}
	for _, f := range rows {
		if f.Type == TypePlan {
			if activePlans[f.TargetID] {
				views = append(views, s.view(f, nil))
			} else {
				gone[TypePlan] = append(gone[TypePlan], f.TargetID)
			}
			continue
		}
		b, ok := shops[f.TargetID]
		switch {
		case !ok || b.DeletedAt != nil:
			gone[TypeBusiness] = append(gone[TypeBusiness], f.TargetID)
		case b.State == negocios.StatusApproved:
			views = append(views, s.view(f, &b))
		}
	}
	for _, kind := range []string{TypeBusiness, TypePlan} {
		if err := s.repo.DeleteTargets(ctx, me, kind, gone[kind]); err != nil {
			return nil, errStore()
		}
	}
	return views, nil
}

// targets lee de una vez los negocios y los planes activos de las filas.
func (s *Service) targets(ctx context.Context, rows []Favorite) (map[bson.ObjectID]negocios.Business, map[bson.ObjectID]bool, error) {
	var businessIDs, planIDs []bson.ObjectID
	for _, f := range rows {
		if f.Type == TypePlan {
			planIDs = append(planIDs, f.TargetID)
		} else {
			businessIDs = append(businessIDs, f.TargetID)
		}
	}
	cards, err := s.businesses.FindFavoriteCards(ctx, businessIDs)
	if err != nil {
		return nil, nil, errStore()
	}
	shops := make(map[bson.ObjectID]negocios.Business, len(cards))
	for _, b := range cards {
		shops[b.ID] = b
	}
	activePlans, err := s.plans.ActiveIDs(ctx, planIDs)
	if err != nil {
		return nil, nil, errStore()
	}
	return shops, activePlans, nil
}

func position(signer *pagination.Signer, scope, raw string) (*Position, error) {
	c, ok, err := validation.Position(signer, scope, raw)
	if err != nil || !ok {
		return nil, err
	}
	bad := apperr.Validation("cursor inválido", apperr.FieldError{Field: "cursor", Message: "cursor inválido"})
	at, err := time.Parse(time.RFC3339Nano, c.Sort)
	if err != nil {
		return nil, bad
	}
	id, err := bson.ObjectIDFromHex(c.ID)
	if err != nil {
		return nil, bad
	}
	return &Position{CreatedAt: at, ID: id}, nil
}
