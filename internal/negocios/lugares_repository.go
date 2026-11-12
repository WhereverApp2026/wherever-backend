package negocios

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// placeRow es un negocio de la lista con la distancia que calcula `$geoNear`.
type placeRow struct {
	Business  `bson:",inline"`
	DistanceM *float64 `bson:"distancia_m,omitempty"`
}

// geoPoint es la ubicación de la consulta `[lng, lat]`; nunca se guarda.
type geoPoint struct {
	Lat float64
	Lng float64
}

// listParams son los filtros ya validados de GET /lugares.
type listParams struct {
	Match   bson.A // condiciones que van en `$and` junto con VisibleFilter
	Near    *geoPoint
	RadiusM float64
	Order   string
	After   *listCursor
	Limit   int
	Scope   string // ámbito del cursor firmado (nv-17); no va a MongoDB
}

// sortKey es el campo de cada orden y su dirección; `_id` desempata siempre ascendente.
func sortKey(order string) (string, int) {
	switch order {
	case OrderDistance:
		return "distancia_m", 1
	case OrderPrice:
		return "precio_promedio", 1
	default:
		return "calificacion.promedio", -1
	}
}

// visibleAnd une el filtro de visibilidad con las condiciones extra en un `$and`, así dos
// `$or` (abierto ahora y cursor) no chocan en la misma llave.
func visibleAnd(conditions bson.A) bson.D {
	filter := VisibleFilter()
	if len(conditions) > 0 {
		filter = append(filter, bson.E{Key: "$and", Value: conditions})
	}
	return filter
}

// afterCondition reanuda después del último elemento: valor estrictamente "después" o el
// mismo valor con un `_id` mayor.
func afterCondition(order string, after *listCursor) bson.D {
	field, dir := sortKey(order)
	op := "$gt"
	if dir < 0 {
		op = "$lt"
	}
	return bson.D{{Key: "$or", Value: bson.A{
		bson.D{{Key: field, Value: bson.D{{Key: op, Value: after.Value}}}},
		bson.D{{Key: field, Value: after.Value}, {Key: "_id", Value: bson.D{{Key: "$gt", Value: after.ID}}}},
	}}}
}

// List arma la consulta. Con ubicación, `$geoNear` va primero (radio y distancia); ordenado
// por distancia la página siguiente arranca en `minDistance`, porque `$geoNear` no se
// reanuda por `_id`, y el `$match` posterior quita los empates ya entregados.
func (r *Repository) List(ctx context.Context, p listParams) ([]placeRow, error) {
	field, dir := sortKey(p.Order)
	var pipeline mongo.Pipeline
	if p.Near != nil {
		near := bson.D{
			{Key: "near", Value: bson.D{{Key: "type", Value: "Point"}, {Key: "coordinates", Value: bson.A{p.Near.Lng, p.Near.Lat}}}},
			{Key: "distanceField", Value: "distancia_m"},
			{Key: "maxDistance", Value: p.RadiusM},
			{Key: "spherical", Value: true},
			{Key: "key", Value: "ubicacion"},
			{Key: "query", Value: visibleAnd(p.Match)},
		}
		if p.After != nil && p.Order == OrderDistance {
			near = append(near, bson.E{Key: "minDistance", Value: p.After.Value})
		}
		pipeline = append(pipeline, bson.D{{Key: "$geoNear", Value: near}})
		if p.After != nil {
			pipeline = append(pipeline, bson.D{{Key: "$match", Value: afterCondition(p.Order, p.After)}})
		}
	} else {
		conditions := p.Match
		if p.After != nil {
			conditions = append(append(bson.A{}, conditions...), afterCondition(p.Order, p.After))
		}
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: visibleAnd(conditions)}})
	}
	pipeline = append(pipeline,
		bson.D{{Key: "$sort", Value: bson.D{{Key: field, Value: dir}, {Key: "_id", Value: 1}}}},
		bson.D{{Key: "$limit", Value: p.Limit + 1}},
	)
	cur, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	var rows []placeRow
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// FindVisible devuelve un negocio aprobado y no eliminado; ok=false si no existe o no es
// público.
func (r *Repository) FindVisible(ctx context.Context, id bson.ObjectID) (Business, bool, error) {
	filter := append(VisibleFilter(), bson.E{Key: "_id", Value: id})
	var b Business
	err := r.col.FindOne(ctx, filter).Decode(&b)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Business{}, false, nil
	}
	if err != nil {
		return Business{}, false, err
	}
	return b, true, nil
}

// mapArea es el rectángulo visible y la rejilla del zoom.
type mapArea struct {
	MinLat, MinLng, MaxLat, MaxLng float64
	CellDeg                        float64
	Category                       string
}

func (a mapArea) filter() bson.D {
	ring := bson.A{
		bson.A{a.MinLng, a.MinLat}, bson.A{a.MaxLng, a.MinLat},
		bson.A{a.MaxLng, a.MaxLat}, bson.A{a.MinLng, a.MaxLat},
		bson.A{a.MinLng, a.MinLat},
	}
	conditions := bson.A{bson.D{{Key: "ubicacion", Value: bson.D{{Key: "$geoWithin", Value: bson.D{
		{Key: "$geometry", Value: bson.D{{Key: "type", Value: "Polygon"}, {Key: "coordinates", Value: bson.A{ring}}}},
	}}}}}}
	if a.Category != "" {
		conditions = append(conditions, bson.D{{Key: "categoria", Value: a.Category}})
	}
	return visibleAnd(conditions)
}

// MapPins trae hasta `limit` negocios del área con lo mínimo para el pin.
func (r *Repository) MapPins(ctx context.Context, a mapArea, limit int) ([]Business, error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: a.filter()}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "calificacion.promedio", Value: -1}, {Key: "_id", Value: 1}}}},
		bson.D{{Key: "$limit", Value: limit}},
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "nombre", Value: 1}, {Key: "categoria", Value: 1},
			{Key: "calificacion", Value: 1}, {Key: "ubicacion", Value: 1},
		}}},
	}
	cur, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	var out []Business
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// cellRow es una celda agregada.
type cellRow struct {
	Lat   float64 `bson:"lat"`
	Lng   float64 `bson:"lng"`
	Count int     `bson:"conteo"`
}

// MapCells agrupa el área en celdas de `CellDeg` grados con conteo y centro promedio. Las
// más pobladas primero, hasta `limit`.
func (r *Repository) MapCells(ctx context.Context, a mapArea, limit int) ([]cellRow, error) {
	lng := bson.D{{Key: "$arrayElemAt", Value: bson.A{"$ubicacion.coordinates", 0}}}
	lat := bson.D{{Key: "$arrayElemAt", Value: bson.A{"$ubicacion.coordinates", 1}}}
	cell := func(value bson.D, origin float64) bson.D {
		return bson.D{{Key: "$floor", Value: bson.D{{Key: "$divide", Value: bson.A{
			bson.D{{Key: "$subtract", Value: bson.A{value, origin}}}, a.CellDeg,
		}}}}}
	}
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: a.filter()}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{{Key: "x", Value: cell(lng, a.MinLng)}, {Key: "y", Value: cell(lat, a.MinLat)}}},
			{Key: "lng", Value: bson.D{{Key: "$avg", Value: lng}}},
			{Key: "lat", Value: bson.D{{Key: "$avg", Value: lat}}},
			{Key: "conteo", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "conteo", Value: -1}, {Key: "_id", Value: 1}}}},
		bson.D{{Key: "$limit", Value: limit}},
	}
	cur, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	var out []cellRow
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
