package planes

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// Service resuelve los endpoints de planes (nv-30). Solo el dueño ve y edita su plan: toda
// lectura pasa por FindOwned y un plan ajeno responde 404, igual que uno que no existe.
type Service struct {
	repo  *Repository
	calc  *Calculadora
	graph Graph
	now   func() time.Time
}

// Graph recibe al dueño de un plan que cambió para reflejarlo en Neo4j (nv-52). Es
// eventual: un fallo no cambia la respuesta.
type Graph interface {
	Touch(ctx context.Context, userID bson.ObjectID) error
}

type noGraph struct{}

func (noGraph) Touch(context.Context, bson.ObjectID) error { return nil }

// NewService recibe el repositorio y la calculadora por constructor.
func NewService(repo *Repository, calc *Calculadora) *Service {
	return &Service{repo: repo, calc: calc, graph: noGraph{}, now: time.Now}
}

// WithGraph refleja los planes guardados en el grafo (nv-52).
func (s *Service) WithGraph(g Graph) *Service {
	s.graph = g
	return s
}

// Errores públicos: se crean en cada llamada porque la etapa de errores puede anotarlos.
func errPlanNotFound() error { return apperr.NotFound("plan no encontrado") }
func errStopNotFound() error { return apperr.NotFound("parada no encontrada") }
func errStale() error {
	return apperr.Conflict("el plan cambió en otro dispositivo; vuelve a cargarlo")
}

// errStore no lleva el texto de MongoDB: podría repetir el nombre o las notas del plan.
func errStore() error { return apperr.Unavailable("servicio no disponible") }

func owner(subject string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("autenticación requerida")
	}
	return id, nil
}

// storeError traduce los errores del repositorio.
func storeError(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return errPlanNotFound()
	case errors.Is(err, ErrVersionConflict):
		return errStale()
	default:
		return errStore()
	}
}

// planError traduce lo que rechaza Recalcular a 422 con el campo en falta.
func planError(err error) error {
	var stop *StopError
	switch {
	case errors.As(err, &stop):
		return invalid("paradas", "parada "+stop.StopID.Hex()+": "+stop.Reason)
	case errors.Is(err, ErrStopCount):
		return invalid("paradas", ErrStopCount.Error())
	case errors.Is(err, ErrMobility):
		return invalid("movilidad", ErrMobility.Error())
	case errors.Is(err, ErrPeople):
		return invalid("personas", ErrPeople.Error())
	default:
		return errStore()
	}
}

// Create guarda un plan nuevo ya calculado (201).
func (s *Service) Create(ctx context.Context, subject string, req CreateRequest) (Plan, error) {
	ownerID, err := owner(subject)
	if err != nil {
		return Plan{}, err
	}
	p, err := req.toPlan(ownerID)
	if err != nil {
		return Plan{}, err
	}
	return s.insert(ctx, &p)
}

func (s *Service) insert(ctx context.Context, p *Plan) (Plan, error) {
	if err := s.calc.Aplicar(ctx, p); err != nil {
		return Plan{}, planError(err)
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return Plan{}, errStore()
	}
	_ = s.graph.Touch(ctx, p.OwnerID)
	return *p, nil
}

// Get trae el plan del dueño; ajeno, borrado o id mal formado = 404.
func (s *Service) Get(ctx context.Context, subject, rawID string) (Plan, error) {
	ownerID, err := owner(subject)
	if err != nil {
		return Plan{}, err
	}
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return Plan{}, errPlanNotFound()
	}
	p, err := s.repo.FindOwned(ctx, ownerID, id)
	if err != nil {
		return Plan{}, storeError(err)
	}
	return p, nil
}

// edit carga el plan del dueño, comprueba la versión, aplica el cambio, recalcula y guarda
// con If-Match. Es el único camino de las ediciones.
func (s *Service) edit(ctx context.Context, subject, rawID string, version int, change func(*Plan) error) (Plan, error) {
	p, err := s.Get(ctx, subject, rawID)
	if err != nil {
		return Plan{}, err
	}
	if p.Version != version {
		return Plan{}, errStale()
	}
	if err := change(&p); err != nil {
		return Plan{}, err
	}
	if err := s.calc.Aplicar(ctx, &p); err != nil {
		return Plan{}, planError(err)
	}
	if err := s.repo.UpdateIfVersion(ctx, &p, version); err != nil {
		return Plan{}, storeError(err)
	}
	_ = s.graph.Touch(ctx, p.OwnerID)
	return p, nil
}

// Patch cambia nombre, fecha, personas, presupuesto o movilidad.
func (s *Service) Patch(ctx context.Context, subject, rawID string, version int, req PatchRequest) (Plan, error) {
	return s.edit(ctx, subject, rawID, version, req.apply)
}

// AddStop agrega una parada; el plan sigue en orden de llegada.
func (s *Service) AddStop(ctx context.Context, subject, rawID string, version int, req StopRequest) (Plan, error) {
	return s.edit(ctx, subject, rawID, version, func(p *Plan) error {
		stop, err := req.toStop("parada")
		if err != nil {
			return err
		}
		p.Paradas = append(p.Paradas, stop)
		return nil
	})
}

func stopIndex(p *Plan, rawStopID string) (int, error) {
	for i, s := range p.Paradas {
		if strings.EqualFold(s.ID.Hex(), rawStopID) {
			return i, nil
		}
	}
	return 0, errStopNotFound()
}

// PatchStop mueve una parada o cambia su duración, precio o nota.
func (s *Service) PatchStop(ctx context.Context, subject, rawID, rawStopID string, version int, req PatchStopRequest) (Plan, error) {
	return s.edit(ctx, subject, rawID, version, func(p *Plan) error {
		i, err := stopIndex(p, rawStopID)
		if err != nil {
			return err
		}
		return req.apply(&p.Paradas[i])
	})
}

// RemoveStop quita una parada; la última no se puede quitar (422).
func (s *Service) RemoveStop(ctx context.Context, subject, rawID, rawStopID string, version int) (Plan, error) {
	return s.edit(ctx, subject, rawID, version, func(p *Plan) error {
		i, err := stopIndex(p, rawStopID)
		if err != nil {
			return err
		}
		p.Paradas = append(p.Paradas[:i], p.Paradas[i+1:]...)
		return nil
	})
}

// Reorder reparte las horas de llegada en el orden pedido.
func (s *Service) Reorder(ctx context.Context, subject, rawID string, version int, req OrderRequest) (Plan, error) {
	return s.edit(ctx, subject, rawID, version, func(p *Plan) error {
		return reorder(p.Paradas, req.Orden)
	})
}

// Delete hace el borrado lógico con If-Match.
func (s *Service) Delete(ctx context.Context, subject, rawID string, version int) error {
	p, err := s.Get(ctx, subject, rawID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteIfVersion(ctx, p.OwnerID, p.ID, version); err != nil {
		return storeError(err)
	}
	_ = s.graph.Touch(ctx, p.OwnerID)
	return nil
}

// Clone repite un plan propio en otra fecha (US-058): paradas con ids nuevos y las mismas
// horas en el día nuevo. No copia reservas ni integrantes.
func (s *Service) Clone(ctx context.Context, subject, rawID string, req CloneRequest) (Plan, error) {
	src, err := s.Get(ctx, subject, rawID)
	if err != nil {
		return Plan{}, err
	}
	day, err := parseDay("fecha", req.Fecha)
	if err != nil {
		return Plan{}, err
	}
	p := Plan{
		OwnerID: src.OwnerID, Nombre: src.Nombre, Fecha: src.Fecha, Personas: src.Personas, Presupuesto: src.Presupuesto,
		Movilidad: src.Movilidad, Origen: src.Origen, Paradas: make([]Parada, len(src.Paradas)),
	}
	copy(p.Paradas, src.Paradas)
	for i := range p.Paradas {
		p.Paradas[i].ID = bson.NewObjectID()
	}
	if req.Nombre != "" {
		p.Nombre = req.Nombre
	}
	moveToDay(&p, day)
	return s.insert(ctx, &p)
}

// List devuelve una página de la pestaña pedida.
func (s *Service) List(ctx context.Context, subject string, q ListQuery) (pagination.PageResult[Plan], error) {
	ownerID, err := owner(subject)
	if err != nil {
		return pagination.PageResult[Plan]{}, err
	}
	page := s.tab(q.Pestana)
	page.Owner, page.Limit = ownerID, q.Limit
	page.After, err = position(q.Pestana, q.Cursor)
	if err != nil {
		return pagination.PageResult[Plan]{}, err
	}
	rows, err := s.repo.List(ctx, page)
	if err != nil {
		return pagination.PageResult[Plan]{}, errStore()
	}
	next := ""
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		last := rows[len(rows)-1]
		next = pagination.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: q.Pestana + ":" + last.Fecha.UTC().Format(time.RFC3339Nano)})
	}
	return pagination.NewPageResult(rows, next, q.Limit), nil
}

// tab fija el rango y el orden de cada pestaña con el día de hoy en México.
func (s *Service) tab(name string) PageQuery {
	now := s.now().In(negocios.MexicoCity())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch name {
	case TabPast:
		return PageQuery{Before: today, Desc: true}
	case TabSaved:
		return PageQuery{Desc: true}
	default:
		return PageQuery{Since: today}
	}
}

// position lee el cursor; uno de otra pestaña o manipulado es 422.
func position(tab, raw string) (*Position, error) {
	bad := invalid("cursor", "cursor inválido")
	c, ok, err := pagination.Decode(raw)
	if err != nil {
		return nil, bad
	}
	if !ok {
		return nil, nil
	}
	value, found := strings.CutPrefix(c.Sort, tab+":")
	if !found {
		return nil, bad
	}
	at, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, bad
	}
	id, err := bson.ObjectIDFromHex(c.ID)
	if err != nil {
		return nil, bad
	}
	return &Position{Fecha: at, ID: id}, nil
}
