package account

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

func strPtr(s string) *string { return &s }

// `actividad` y `solicitudes` solo aceptan sus valores; los demás campos son interruptores.
func TestSocialInput_Enums(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	for _, ok := range []PrivacyRequest{
		{}, {Social: &SocialInput{}},
		{Social: &SocialInput{Activity: strPtr("amigos"), Requests: strPtr("cualquiera")}},
		{Social: &SocialInput{Activity: strPtr("todos")}},
		{Social: &SocialInput{Activity: strPtr("nadie"), Requests: strPtr("codigo")}},
	} {
		assert.NoError(t, v.Struct(ok))
	}
	for field, bad := range map[string]PrivacyRequest{
		"actividad":   {Social: &SocialInput{Activity: strPtr("publico")}},
		"solicitudes": {Social: &SocialInput{Requests: strPtr("nadie")}},
		"vacía":       {Social: &SocialInput{Activity: strPtr("")}},
	} {
		var appErr *apperr.Error
		require.ErrorAs(t, v.Struct(bad), &appErr, field)
		assert.Equal(t, apperr.KindValidation, appErr.Kind, field)
	}
}

// Sin `privacidad.social` la respuesta trae los valores por omisión.
func TestSocialResponse_ValoresPorOmision(t *testing.T) {
	got := privacyResponse(usuarios.Privacy{}).Social
	assert.Equal(t, SocialResponse{Activity: "amigos", ShareReviews: true, SharePlans: true, SharePostcards: true, InSuggestions: true, Requests: "cualquiera"}, got)
	off := false
	got = privacyResponse(usuarios.Privacy{Social: usuarios.Social{Activity: "nadie", SharePlans: &off, Requests: "codigo"}}).Social
	assert.Equal(t, SocialResponse{Activity: "nadie", ShareReviews: true, SharePlans: false, SharePostcards: true, InSuggestions: true, Requests: "codigo"}, got)
}

// Solo lo que llega entra al `$set`, con la ruta `privacidad.social.*`.
func TestSetSocial_Parcial(t *testing.T) {
	set := bson.D{}
	setSocial(&set, nil)
	assert.Empty(t, set)
	off := false
	setSocial(&set, &SocialInput{Activity: strPtr("todos"), InSuggestions: &off})
	assert.Equal(t, bson.D{{Key: "privacidad.social.actividad", Value: "todos"}, {Key: "privacidad.social.en_sugerencias", Value: false}}, set)
}
