package resenas

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// visitsFilter son los check-ins válidos de users desde since (ix_mis_checkins).
func visitsFilter(users []bson.ObjectID, since time.Time) bson.D {
	return bson.D{
		{Key: "usuario_id", Value: bson.D{{Key: "$in", Value: users}}},
		{Key: "creado_en", Value: bson.D{{Key: "$gte", Value: since}}},
		{Key: "valido", Value: true},
	}
}

// VisitedPlaces son los negocios distintos con check-in válido de users desde since: el
// filtro «De amigos» de Explorar (nv-85). Cumple negocios.Visits.
func (r *Repository) VisitedPlaces(ctx context.Context, users []bson.ObjectID, since time.Time) ([]bson.ObjectID, error) {
	if len(users) == 0 {
		return []bson.ObjectID{}, nil
	}
	res := r.checkins.Distinct(ctx, "negocio_id", visitsFilter(users, since))
	var out []bson.ObjectID
	if err := res.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// VisitsAt es el check-in válido más reciente de cada persona de users en cada negocio de
// places desde since (tarjeta de Explorar, nv-85). Cumple negocios.Visits.
func (r *Repository) VisitsAt(ctx context.Context, users, places []bson.ObjectID, since time.Time) ([]negocios.FriendVisit, error) {
	if len(users) == 0 || len(places) == 0 {
		return []negocios.FriendVisit{}, nil
	}
	match := append(visitsFilter(users, since), bson.E{Key: "negocio_id", Value: bson.D{{Key: "$in", Value: places}}})
	cur, err := r.checkins.Aggregate(ctx, mongo.Pipeline{
		bson.D{{Key: "$match", Value: match}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{{Key: "negocio_id", Value: "$negocio_id"}, {Key: "usuario_id", Value: "$usuario_id"}}},
			{Key: "ultimo", Value: bson.D{{Key: "$max", Value: "$creado_en"}}},
		}}},
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0}, {Key: "negocio_id", Value: "$_id.negocio_id"},
			{Key: "usuario_id", Value: "$_id.usuario_id"}, {Key: "ultimo", Value: 1},
		}}},
	})
	if err != nil {
		return nil, err
	}
	out := []negocios.FriendVisit{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
