package security

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/resenas"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

var rutasExplorar85 = []string{
	"/api/v1/lugares?",
	"/api/v1/lugares/conteo?",
	"/api/v1/lugares/mapa?min_lat=19&max_lat=20&min_lng=-100&max_lng=-99&zoom=14&",
}

// `de_amigos` sin sesión = 401, con token inválido = 401 y con la cuenta en eliminación
// pendiente = 403, en las tres rutas.
func TestExplorarSecurity_DeAmigosPideSesionActiva(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	pending := env.CreateUser(t, "pendiente-exp85@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	pendiente := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-exp85", middleware.RoleConsumer)
	for _, r := range rutasExplorar85 {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, r+"de_amigos=true", nil, "").status, r)
		assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, r+"de_amigos=true", nil, "no-es-un-token").status, r)
		assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodGet, r+"de_amigos=true", nil, pendiente).status, r)
		assert.Equal(t, http.StatusOK, send(t, app, http.MethodGet, r+"con_sello=true", nil, "").status, r)
	}
}

// Una categoría o un precio que no son del catálogo (inyección incluida) = 422.
func TestExplorarSecurity_CategoriasYPrecioInvalidos(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	malos := []string{
		"categorias=" + url.QueryEscape(`cafeteria,{"$ne":1}`),
		"categorias=" + url.QueryEscape("bar,cafeteria,restaurante,comercio,servicios,entretenimiento,otro"),
		"categorias=" + url.QueryEscape("bar|.*"),
		"precio=" + url.QueryEscape("1;2"),
		"precio=5",
	}
	for _, r := range rutasExplorar85 {
		for _, q := range malos {
			assert.Equal(t, http.StatusUnprocessableEntity, send(t, app, http.MethodGet, r+q, nil, "").status, r+q)
		}
	}
}

// La tarjeta nunca trae nombres completos ni a quien no es mi amigo o oculta su actividad.
func TestExplorarSecurity_SoloInicialesDeAmigosQueComparten(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	db := env.Mongo.DB()
	ctx := context.Background()
	now := time.Now().UTC()
	lugar := negocios.Business{
		ID: bson.NewObjectID(), Slug: "exp85-seg", QRCheckin: "qr-exp85-seg", Name: "Lugar Seguro", Category: "restaurante",
		PriceLevel: 2, State: negocios.StatusApproved, Location: negocios.Location{Type: "Point", Coordinates: []float64{-99.1, 19.4}},
	}
	_, err := db.Collection(negocios.Collection).InsertOne(ctx, lugar)
	require.NoError(t, err)
	yo := env.CreateUser(t, "yo-exp85@wherever.test")
	ana := env.CreateUser(t, "ana-exp85@wherever.test")
	oculto := env.CreateUser(t, "oculto-exp85@wherever.test")
	extrana := env.CreateUser(t, "extrana-exp85@wherever.test")
	_, err = env.Users.Update(ctx, ana.ID, bson.D{{Key: "nombre", Value: "Ana Secreta Pérez"}})
	require.NoError(t, err)
	_, err = env.Users.Update(ctx, oculto.ID, bson.D{{Key: "privacidad.social.actividad", Value: usuarios.ActivityNobody}})
	require.NoError(t, err)
	for _, other := range []usuarios.User{ana, oculto} {
		f := amistades.Friendship{ID: bson.NewObjectID(), From: yo.ID, To: other.ID, Status: amistades.StatusAccepted,
			Pair: amistades.PairKey(yo.ID, other.ID), CreatedAt: now, UpdatedAt: now}
		require.NoError(t, amistades.NewRepository(db).Insert(ctx, &f))
	}
	for _, who := range []usuarios.User{ana, oculto, extrana} {
		_, err := db.Collection(resenas.CheckinsCollection).InsertOne(ctx, resenas.Checkin{ID: bson.NewObjectID(), UsuarioID: who.ID,
			NegocioID: lugar.ID, Metodo: resenas.MetodoQR, Valido: true, Dia: now.Format(time.DateOnly), CreadoEn: now.Add(-time.Hour)})
		require.NoError(t, err)
	}
	token := testutil.ValidTokenFor(t, yo.ID.Hex(), "jti-yo-exp85", middleware.RoleConsumer)
	for _, q := range []string{"", "de_amigos=true"} {
		r := send(t, app, http.MethodGet, "/api/v1/lugares?"+q, nil, token)
		require.Equal(t, http.StatusOK, r.status, r.body)
		assert.Contains(t, r.body, `"iniciales":"AP"`, q)
		assert.Contains(t, r.body, `"total":1`, q)
		for _, leak := range []string{"Ana Secreta", "Pérez", oculto.ID.Hex(), extrana.ID.Hex(), "ana-exp85@"} {
			assert.NotContains(t, r.body, leak, q)
		}
	}
	// Sin sesión no hay amigos en la tarjeta.
	r := send(t, app, http.MethodGet, "/api/v1/lugares", nil, "")
	require.Equal(t, http.StatusOK, r.status)
	assert.NotContains(t, r.body, "amigos_fueron")
	assert.NotContains(t, r.body, ana.ID.Hex())
}
