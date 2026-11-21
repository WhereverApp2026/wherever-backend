package usuarios

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
)

// NewConsumer arma la cuenta con la que nace un consumidor (hoja A1): rol, modo, preferencias,
// puntos y nivel iniciales. La edad y los términos ya se validaron; lo usan el registro (nv-05)
// y la entrada con Google (nv-72).
func NewConsumer(name, email string, birth time.Time, terms string, promoEmail bool, now time.Time) User {
	name = strings.TrimSpace(name)
	return User{
		Name: name, NormalizedName: text.Normalize(name), Email: email,
		BirthDate: &birth, Roles: []string{middleware.RoleConsumer}, Mode: ModeExplore,
		Preferences: DefaultPreferences(),
		Privacy:     Privacy{TermsVersion: terms, AcceptedAt: now, PromoEmail: promoEmail},
		Points:      WelcomePoints, Level: InitialLevel, LevelNumber: InitialLevelNumber, Status: StatusActive,
	}
}

// FindByGoogleID busca por `google_id` con ux_google_id (nv-72).
func (r *Repository) FindByGoogleID(ctx context.Context, sub string) (User, error) {
	return r.findOne(ctx, bson.D{{Key: "google_id", Value: sub}})
}

// LinkGoogle vincula la cuenta a Google solo si aún no tiene `google_id`, y marca el correo
// verificado (Google ya lo verificó). Con dropPassword borra `password_hash`: la contraseña
// la puso alguien que nunca probó ser dueño del correo (nv-72). ErrNotFound = otra petición la
// vinculó antes; ErrGoogleTaken = ese `sub` ya es de otra cuenta.
func (r *Repository) LinkGoogle(ctx context.Context, id bson.ObjectID, sub string, dropPassword bool) (User, error) {
	filter := bson.D{{Key: "_id", Value: id}, {Key: "google_id", Value: bson.D{{Key: "$exists", Value: false}}}}
	set := bson.D{{Key: "google_id", Value: sub}, {Key: "correo_verificado", Value: true}}
	var unset bson.D
	if dropPassword {
		unset = bson.D{{Key: "password_hash", Value: ""}}
	}
	return r.apply(ctx, filter, set, unset)
}
