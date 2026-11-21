package security

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

const privacidadSocialPath = "/api/v1/usuarios/yo/privacidad"

// Con `actividad: nadie` ninguna ruta social muestra su actividad a nadie más: ni a una amiga,
// ni a una extraña, ni sin sesión (nv-82). La autora sigue viendo lo suyo.
func TestPrivacidadSocialSecurity_NadieEnCadaRuta(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	ctx := context.Background()
	db := env.Mongo.DB()
	beto := env.CreateUser(t, "beto-social82@wherever.test")
	ana := env.CreateUser(t, "ana-social82@wherever.test")
	cleo := env.CreateUser(t, "cleo-social82@wherever.test")
	now := time.Now().UTC()
	f := amistades.Friendship{ID: bson.NewObjectID(), From: ana.ID, To: beto.ID, Status: amistades.StatusAccepted, Pair: amistades.PairKey(ana.ID, beto.ID), CreatedAt: now, UpdatedAt: now}
	require.NoError(t, amistades.NewRepository(db).Insert(ctx, &f))

	lugar := negocios.Business{ID: bson.NewObjectID(), Slug: "social82", QRCheckin: "qr-social82", Name: "Social 82", NormalizedName: "social 82",
		Category: "restaurante", State: negocios.StatusApproved, Location: negocios.Location{Type: "Point", Coordinates: []float64{-99.13, 19.43}}, CreatedAt: now}
	_, err := db.Collection(negocios.Collection).InsertOne(ctx, lugar)
	require.NoError(t, err)
	p := postales.Postal{ID: bson.NewObjectID(), UsuarioID: beto.ID, ImagenID: bson.NewObjectID().Hex(), ImagenEstado: "publicada", NegocioID: &lugar.ID,
		Estilo: postales.EstiloPolaroid, TextoEstado: postales.TextoAprobado, Visibilidad: postales.VisPublica, Estado: postales.EstadoLista, Version: 1, CreadoEn: now, ActualizadoEn: now}
	_, err = db.Collection(postales.Collection).InsertOne(ctx, p)
	require.NoError(t, err)
	m := postales.Momento{ID: bson.NewObjectID(), AutorID: beto.ID, PostalID: p.ID, NegocioID: &lugar.ID, Visibilidad: postales.VisPublica,
		Estado: postales.MomentoPublicado, Etiquetados: []bson.ObjectID{}, Version: 1, CreadoEn: now, ActualizadoEn: now}
	_, err = db.Collection(postales.MomentosCollection).InsertOne(ctx, m)
	require.NoError(t, err)

	tokBeto := testutil.ValidTokenFor(t, beto.ID.Hex(), "jti-beto-social82", middleware.RoleConsumer)
	tokAna := testutil.ValidTokenFor(t, ana.ID.Hex(), "jti-ana-social82", middleware.RoleConsumer)
	tokCleo := testutil.ValidTokenFor(t, cleo.ID.Hex(), "jti-cleo-social82", middleware.RoleConsumer)

	// Antes, la amiga ve el momento: la prueba no pasa por un dato mal sembrado.
	before := send(t, app, http.MethodGet, "/api/v1/momentos/feed", nil, tokAna)
	require.Equal(t, http.StatusOK, before.status, before.body)
	require.Contains(t, before.body, m.ID.Hex())

	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodPatch, privacidadSocialPath, map[string]any{"social": map[string]any{"actividad": "nadie"}}, "").status)
	set := send(t, app, http.MethodPatch, privacidadSocialPath, map[string]any{"social": map[string]any{"actividad": "nadie"}}, tokBeto)
	require.Equal(t, http.StatusOK, set.status, set.body)

	galeria := "/api/v1/lugares/" + lugar.ID.Hex() + "/postales"
	for _, viewer := range []string{tokAna, tokCleo, ""} {
		if viewer != "" {
			prof := send(t, app, http.MethodGet, "/api/v1/personas/"+beto.ID.Hex(), nil, viewer)
			require.Equal(t, http.StatusOK, prof.status, prof.body)
			for _, hidden := range []string{"nivel", "amigos_en_comun", "grupos_en_comun", "silenciado", "social", "privacidad"} {
				assert.NotContains(t, prof.body, hidden)
			}
			ins := send(t, app, http.MethodGet, "/api/v1/personas/"+beto.ID.Hex()+"/insignias", nil, viewer)
			require.Equal(t, http.StatusOK, ins.status, ins.body)
			assert.JSONEq(t, `{"insignias":[]}`, ins.body)
			feed := send(t, app, http.MethodGet, "/api/v1/momentos/feed", nil, viewer)
			require.Equal(t, http.StatusOK, feed.status, feed.body)
			assert.NotContains(t, feed.body, m.ID.Hex())
			assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, "/api/v1/postales/"+p.ID.Hex(), nil, viewer).status)
		}
		assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, "/api/v1/momentos/"+m.ID.Hex(), nil, viewer).status)
		gal := send(t, app, http.MethodGet, galeria, nil, viewer)
		require.Equal(t, http.StatusOK, gal.status, gal.body)
		assert.NotContains(t, gal.body, p.ID.Hex())
	}
	assert.Equal(t, http.StatusOK, send(t, app, http.MethodGet, "/api/v1/momentos/"+m.ID.Hex(), nil, tokBeto).status)
	assert.Equal(t, http.StatusOK, send(t, app, http.MethodGet, "/api/v1/postales/"+p.ID.Hex(), nil, tokBeto).status)
}

// Con `solicitudes: codigo` un `origen` falso no salta la regla: el código y el teléfono se
// validan contra la cuenta, y el 403 no revela ni el código ni el teléfono (nv-82).
func TestPrivacidadSocialSecurity_OrigenFalsoNoSaltaElCodigo(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	ctx := context.Background()
	cerrada := env.CreateUser(t, "cerrada-social82@wherever.test")
	cerrada, err := env.Users.Update(ctx, cerrada.ID, bson.D{
		{Key: "telefono_hash", Value: usuarios.HashPhone(env.Cfg.PhoneHMACKey, "+525533334444")},
		{Key: "privacidad.buscable_por_telefono", Value: true},
		{Key: "privacidad.social.solicitudes", Value: usuarios.RequestsCode},
	})
	require.NoError(t, err)
	otra := env.CreateUser(t, "otra-social82@wherever.test")
	tok := testutil.ValidTokenFor(t, otra.ID.Hex(), "jti-otra-social82", middleware.RoleConsumer)
	id := cerrada.ID.Hex()

	for _, body := range []map[string]any{
		{"usuario_id": id, "origen": "codigo"},
		{"usuario_id": id, "origen": "codigo", "codigo": otra.FriendCode},
		{"usuario_id": id, "origen": "codigo", "telefono": "5533334444"},
		{"usuario_id": id, "origen": "contactos"},
		{"usuario_id": id, "origen": "contactos", "codigo": cerrada.FriendCode},
		{"usuario_id": id, "origen": "contactos", "telefono": "5500001111"},
		{"usuario_id": id, "origen": "busqueda", "codigo": cerrada.FriendCode},
		{"usuario_id": id, "origen": "sugerencia", "telefono": "5533334444"},
	} {
		r := send(t, app, http.MethodPost, "/api/v1/amistades", body, tok)
		require.Equal(t, http.StatusForbidden, r.status, r.body)
		assert.Contains(t, r.body, `"codigo":"solicitud_restringida"`)
		assert.NotContains(t, r.body, cerrada.FriendCode)
		assert.NotContains(t, r.body, "5533334444")
	}
	assert.Equal(t, http.StatusUnprocessableEntity, send(t, app, http.MethodPost, "/api/v1/amistades", map[string]any{"usuario_id": id, "origen": "admin"}, tok).status)
	assert.NotContains(t, env.Logs.String(), "5533334444", "el teléfono no llega a los logs")
	ok := send(t, app, http.MethodPost, "/api/v1/amistades", map[string]any{"usuario_id": id, "origen": "codigo", "codigo": cerrada.FriendCode}, tok)
	assert.Equal(t, http.StatusCreated, ok.status, ok.body)
}
