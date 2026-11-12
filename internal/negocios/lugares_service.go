package negocios

import (
	"context"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/cache"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// listCursor es el cursor ya decodificado: el valor del campo de orden del último elemento
// (distancia, calificación o precio) y su `_id`.
type listCursor struct {
	ID    bson.ObjectID
	Value any
}

// WithCursor fija la llave del cursor firmado (nv-17).
func (s *Service) WithCursor(signer *pagination.Signer) *Service {
	s.cursor = signer
	return s
}

// listScope ata el cursor al orden y a los filtros de la búsqueda (nv-17): uno de otra
// búsqueda es 422. El hash lleva llave, así la ubicación no se lee del cursor.
func (s *Service) listScope(q ListQuery, p listParams) string {
	parts := []string{p.Order, q.Category, strconv.Itoa(q.MaxPrice), strconv.FormatBool(q.OpenNow), strconv.FormatFloat(p.RadiusM, 'g', -1, 64)}
	if p.Near != nil {
		parts = append(parts, strconv.FormatFloat(p.Near.Lat, 'g', -1, 64), strconv.FormatFloat(p.Near.Lng, 'g', -1, 64))
	}
	return s.cursor.Scope("lugares", parts...)
}

// encodeCursor guarda `orden:valor` en el campo `orden` del cursor compartido.
func (s *Service) encodeCursor(scope, order string, row placeRow) (string, error) {
	var value string
	switch order {
	case OrderDistance:
		if row.DistanceM != nil {
			value = strconv.FormatFloat(*row.DistanceM, 'g', -1, 64)
		}
	case OrderPrice:
		value = strconv.Itoa(row.AveragePrice)
	default:
		value = strconv.FormatFloat(row.Rating.Promedio, 'g', -1, 64)
	}
	return s.cursor.Encode(pagination.Cursor{ID: row.ID.Hex(), Sort: order + ":" + value, Scope: scope})
}

func (s *Service) decodeCursor(scope, raw, order string) (*listCursor, error) {
	invalid := apperr.Validation("cursor inválido", apperr.FieldError{Field: "cursor", Message: "cursor inválido"})
	c, ok, err := validation.Position(s.cursor, scope, raw)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	prefix, value, found := strings.Cut(c.Sort, ":")
	if !found || prefix != order {
		return nil, invalid
	}
	id, err := bson.ObjectIDFromHex(c.ID)
	if err != nil {
		return nil, invalid
	}
	parsed, ok := cursorValue(order, value)
	if !ok {
		return nil, invalid
	}
	return &listCursor{ID: id, Value: parsed}, nil
}

// cursorValue lee el valor con el tipo del campo: entero para precio, flotante finito para
// distancia y calificación.
func cursorValue(order, value string) (any, bool) {
	if order == OrderPrice {
		n, err := strconv.Atoi(value)
		return n, err == nil
	}
	f, err := strconv.ParseFloat(value, 64)
	return f, err == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
}

// WithClock fija el reloj de «abierto ahora» (pruebas de integración).
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

// List resuelve GET /lugares. Todos los filtros van dentro de MongoDB antes del límite.
func (s *Service) List(ctx context.Context, q ListQuery) (pagination.PageResult[PlaceItem], error) {
	params, err := s.listParams(q)
	if err != nil {
		return pagination.PageResult[PlaceItem]{}, err
	}
	rows, err := s.repo.List(ctx, params)
	if err != nil {
		// Sin el texto del error: podría repetir la ubicación de la consulta.
		return pagination.PageResult[PlaceItem]{}, apperr.Unavailable("servicio no disponible")
	}
	next := ""
	if len(rows) > params.Limit {
		rows = rows[:params.Limit]
		if next, err = s.encodeCursor(params.Scope, params.Order, rows[len(rows)-1]); err != nil {
			return pagination.PageResult[PlaceItem]{}, apperr.Unavailable("servicio no disponible")
		}
	}
	now := s.now()
	items := make([]PlaceItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, toItem(row, now))
	}
	return pagination.NewPageResult(items, next, params.Limit), nil
}

func (s *Service) listParams(q ListQuery) (listParams, error) {
	if (q.Lat == nil) != (q.Lng == nil) {
		return listParams{}, apperr.Validation("lat y lng van juntos",
			apperr.FieldError{Field: "lat", Message: "lat y lng van juntos"})
	}
	p := listParams{Order: q.Order, Limit: q.Limit, RadiusM: DefaultRadiusM}
	if q.Lat != nil {
		p.Near = &geoPoint{Lat: *q.Lat, Lng: *q.Lng}
		if q.RadiusM > 0 {
			p.RadiusM = float64(q.RadiusM)
		}
	}
	p.Order = resolveOrder(q.Order, p.Near != nil)
	if p.Order == OrderDistance && p.Near == nil {
		return listParams{}, apperr.Validation("ordenar por distancia requiere lat y lng",
			apperr.FieldError{Field: "orden", Message: "requiere lat y lng"})
	}
	match, err := s.matchConditions(q)
	if err != nil {
		return listParams{}, err
	}
	p.Match = match
	p.Scope = s.listScope(q, p)
	after, err := s.decodeCursor(p.Scope, q.Cursor, p.Order)
	if err != nil {
		return listParams{}, err
	}
	p.After = after
	return p, nil
}

// resolveOrder: sin `orden`, por distancia si hay ubicación y por calificación si no.
func resolveOrder(order string, hasLocation bool) string {
	switch {
	case order != "":
		return order
	case hasLocation:
		return OrderDistance
	default:
		return OrderRating
	}
}

// matchConditions traduce categoría, precio máximo y abierto ahora a condiciones de MongoDB.
func (s *Service) matchConditions(q ListQuery) (bson.A, error) {
	var match bson.A
	if q.Category != "" {
		if !knownCategory(q.Category) {
			return nil, unknownCategory()
		}
		match = append(match, bson.D{{Key: "categoria", Value: q.Category}})
	}
	if q.MaxPrice > 0 {
		match = append(match, bson.D{{Key: "nivel_precio", Value: bson.D{{Key: "$lte", Value: q.MaxPrice}}}})
	}
	if q.OpenNow {
		match = append(match, bson.D{openAtFilter(s.now())})
	}
	return match, nil
}

func knownCategory(id string) bool {
	return slices.ContainsFunc(categoryCatalog, func(c CatalogItem) bool { return c.ID == id })
}

func unknownCategory() error {
	return apperr.Validation("categoría desconocida",
		apperr.FieldError{Field: "categoria", Message: "categoría desconocida"})
}

// Detail resuelve GET /lugares/{id}. Un id mal formado o un negocio no público es 404.
func (s *Service) Detail(ctx context.Context, rawID string) (PlaceDetail, error) {
	notFound := apperr.NotFound("lugar no encontrado")
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return PlaceDetail{}, notFound
	}
	f, err := cache.Obtener(ctx, s.cache, claveFicha(id.Hex()), VidaFicha, func(ctx context.Context) (fichaGuardada, error) {
		b, ok, err := s.repo.FindVisible(ctx, id)
		if err != nil {
			return fichaGuardada{}, apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
		}
		if !ok {
			return fichaGuardada{}, notFound
		}
		return fichaGuardada{Detalle: toDetail(b, s.now()), Ventanas: b.ScheduleMin}, nil
	})
	if err != nil {
		return PlaceDetail{}, err
	}
	// «Abierto ahora» se calcula en cada petición, no al guardar.
	detalle := f.Detalle
	detalle.OpenNow = OpenAt(f.Ventanas, s.now())
	return detalle, nil
}

// fichaGuardada es lo que vive en `cache:negocio:{id}`: la ficha y las ventanas de horario
// para recalcular «abierto ahora».
type fichaGuardada struct {
	Detalle  PlaceDetail  `json:"detalle"`
	Ventanas []VentanaMin `json:"ventanas"`
}

// CellDegrees es el lado de la celda para un zoom: un mosaico de 256 px cubre 360/2^zoom
// grados de longitud y se parte en cellsPerTile celdas por lado.
func CellDegrees(zoom int) float64 {
	return 360 / math.Pow(2, float64(zoom)) / cellsPerTile
}

// Map resuelve GET /lugares/mapa: pines sueltos hasta MaxMapPins; si hay más, celdas.
func (s *Service) Map(ctx context.Context, q MapQuery) (MapResponse, error) {
	if *q.MinLat >= *q.MaxLat || *q.MinLng >= *q.MaxLng || *q.MaxLng-*q.MinLng >= 180 {
		return MapResponse{}, apperr.Validation("área inválida",
			apperr.FieldError{Field: "min_lng", Message: "el área debe tener mínimo < máximo y menos de 180° de ancho"})
	}
	if q.Category != "" && !knownCategory(q.Category) {
		return MapResponse{}, unknownCategory()
	}
	area := mapArea{
		MinLat: *q.MinLat, MinLng: *q.MinLng, MaxLat: *q.MaxLat, MaxLng: *q.MaxLng,
		CellDeg: CellDegrees(q.Zoom), Category: q.Category,
	}
	unavailable := func(err error) error {
		return apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
	}
	pins, err := s.repo.MapPins(ctx, area, MaxMapPins+1)
	if err != nil {
		return MapResponse{}, unavailable(err)
	}
	if len(pins) <= MaxMapPins {
		return pinsResponse(pins), nil
	}
	cells, err := s.repo.MapCells(ctx, area, MaxMapCells)
	if err != nil {
		return MapResponse{}, unavailable(err)
	}
	return cellsResponse(cells), nil
}

func pinsResponse(pins []Business) MapResponse {
	out := MapResponse{Pins: make([]MapPin, 0, len(pins)), Cells: []MapCell{}}
	for _, b := range pins {
		out.Pins = append(out.Pins, MapPin{
			ID: b.ID.Hex(), Name: b.Name, Category: b.Category,
			Rating: b.Rating.Promedio, Location: pointOf(b.Location),
		})
	}
	return out
}

func cellsResponse(cells []cellRow) MapResponse {
	out := MapResponse{Grouped: true, Pins: []MapPin{}, Cells: make([]MapCell, 0, len(cells))}
	for _, c := range cells {
		out.Cells = append(out.Cells, MapCell{Center: Point{Lat: c.Lat, Lng: c.Lng}, Count: c.Count})
	}
	return out
}
