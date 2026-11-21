package usuarios

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Quién ve la actividad de la persona (nv-82).
const (
	ActivityFriends = "amigos"
	ActivityAll     = "todos"
	ActivityNobody  = "nadie"
)

// Quién puede mandarle solicitudes de amistad (nv-82).
const (
	RequestsAnyone = "cualquiera"
	RequestsCode   = "codigo"
)

// Campos de `privacidad.social` para filtrar en Mongo.
const (
	fieldActivity      = "privacidad.social.actividad"
	fieldSharePlans    = "privacidad.social.compartir_planes"
	fieldInSuggestions = "privacidad.social.en_sugerencias"
)

// Social es `privacidad.social` (nv-82). Todo campo ausente (cuentas previas) vale lo de por
// omisión: actividad solo para amigos, todo compartido, en sugerencias y solicitudes de
// cualquiera. Por eso se lee con los métodos, nunca con los campos.
type Social struct {
	Activity       string `bson:"actividad,omitempty"`
	ShareReviews   *bool  `bson:"compartir_resenas,omitempty"`
	SharePlans     *bool  `bson:"compartir_planes,omitempty"`
	SharePostcards *bool  `bson:"compartir_postales,omitempty"`
	InSuggestions  *bool  `bson:"en_sugerencias,omitempty"`
	Requests       string `bson:"solicitudes,omitempty"`
}

func orTrue(b *bool) bool { return b == nil || *b }

// ActivityLevel es `actividad` con su valor por omisión.
func (s Social) ActivityLevel() string {
	switch s.Activity {
	case ActivityAll, ActivityNobody:
		return s.Activity
	default:
		return ActivityFriends
	}
}

// RequestsFrom es `solicitudes` con su valor por omisión.
func (s Social) RequestsFrom() string {
	if s.Requests == RequestsCode {
		return RequestsCode
	}
	return RequestsAnyone
}

// HidesActivity dice si nadie más ve su actividad (`actividad: nadie`).
func (s Social) HidesActivity() bool { return s.ActivityLevel() == ActivityNobody }

// SharesReviews dice si sus reseñas salen en su perfil y en el feed (nv-81).
func (s Social) SharesReviews() bool { return orTrue(s.ShareReviews) && !s.HidesActivity() }

// SharesPlans dice si sus planes públicos salen fuera de su cuenta (populares, perfil, feed).
func (s Social) SharesPlans() bool { return orTrue(s.SharePlans) && !s.HidesActivity() }

// SharesPostcards dice si alguien más que ella ve sus momentos y postales, sea cual sea la
// visibilidad de cada uno.
func (s Social) SharesPostcards() bool { return orTrue(s.SharePostcards) && !s.HidesActivity() }

// Switches son los interruptores tal como los eligió la persona (con su valor por omisión),
// sin aplicar `actividad`: es lo que devuelve GET /usuarios/yo. El orden es reseñas, planes,
// postales y sugerencias.
func (s Social) Switches() (reviews, plans, postcards, suggestions bool) {
	return orTrue(s.ShareReviews), orTrue(s.SharePlans), orTrue(s.SharePostcards), orTrue(s.InSuggestions)
}

// HidingPlans dice cuáles de ids no comparten sus planes (`compartir_planes: false` o
// `actividad: nadie`): sus planes públicos no salen en populares (nv-82).
func (r *Repository) HidingPlans(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID]bool, error) {
	out := map[bson.ObjectID]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	filter := bson.D{
		{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: fieldSharePlans, Value: false}},
			bson.D{{Key: fieldActivity, Value: ActivityNobody}},
		}},
	}
	cur, err := r.col.Find(ctx, filter, options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var rows []User
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, u := range rows {
		out[u.ID] = true
	}
	return out, nil
}
