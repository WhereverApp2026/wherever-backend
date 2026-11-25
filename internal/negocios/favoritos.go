package negocios

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Favorites dice cuáles lugares guardó una persona (nv-92); lo cumple favoritos.Repository.
type Favorites interface {
	MarkedPlaces(ctx context.Context, user bson.ObjectID, places []bson.ObjectID) ([]bson.ObjectID, error)
}

// Marks son los lugares favoritos de quien consulta, sobre una página de tarjetas. nil = sin
// sesión (o sin la dependencia): `es_favorito` se omite.
type Marks map[bson.ObjectID]bool

// LoadMarks lee de una sola vez cuáles de places guardó user. Sin user o sin f = nil.
func LoadMarks(ctx context.Context, f Favorites, user *bson.ObjectID, places []bson.ObjectID) (Marks, error) {
	if f == nil || user == nil {
		return nil, nil
	}
	if len(places) == 0 {
		return Marks{}, nil
	}
	marked, err := f.MarkedPlaces(ctx, *user, places)
	if err != nil {
		return nil, err
	}
	return NewMarks(marked), nil
}

// NewMarks arma las marcas con los ids guardados; el resto de la página queda en false.
func NewMarks(marked []bson.ObjectID) Marks {
	m := make(Marks, len(marked))
	for _, id := range marked {
		m[id] = true
	}
	return m
}

// Of es `es_favorito` de una tarjeta: nil sin sesión; true o false con ella.
func (m Marks) Of(id bson.ObjectID) *bool {
	if m == nil {
		return nil
	}
	v := m[id]
	return &v
}

// WithFavorites conecta los favoritos para `es_favorito` en GET /lugares (nv-92). Sin él, la
// tarjeta va sin el campo.
func (s *Service) WithFavorites(f Favorites) *Service {
	s.favorites = f
	return s
}

// markFavorites pone `es_favorito` en la página de GET /lugares con una sola consulta.
func (s *Service) markFavorites(ctx context.Context, rows []placeRow, items []ExploreItem, v *viewer) error {
	if v == nil {
		return nil
	}
	ids := make([]bson.ObjectID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	marks, err := LoadMarks(ctx, s.favorites, &v.ID, ids)
	if err != nil {
		return err
	}
	for i, r := range rows {
		items[i].IsFavorite = marks.Of(r.ID)
	}
	return nil
}
