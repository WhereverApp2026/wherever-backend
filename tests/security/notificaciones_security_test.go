package security

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// Las siete rutas de nv-40 piden sesión; el token del teléfono no llega a los logs.
func TestNotificacionesSecurity_SesionYTokenFueraDeLogs(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	id := bson.NewObjectID().Hex()
	device := strings.Repeat("f", 150)
	routes := []struct {
		method, path string
		body         any
	}{
		{http.MethodPut, "/api/v1/dispositivos", map[string]string{"token": device}},
		{http.MethodDelete, "/api/v1/dispositivos", map[string]string{"token": device}},
		{http.MethodGet, "/api/v1/notificaciones", nil},
		{http.MethodGet, "/api/v1/notificaciones/no-leidas", nil},
		{http.MethodPost, "/api/v1/notificaciones/leer-todas", nil},
		{http.MethodPatch, "/api/v1/notificaciones/" + id, map[string]bool{"leida": true}},
		{http.MethodDelete, "/api/v1/notificaciones/" + id, nil},
	}
	for _, r := range routes {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, r.body, "").status, r.method+" "+r.path)
	}

	u := env.CreateUser(t, "telefono-seguro@wherever.test")
	token := testutil.ValidTokenFor(t, u.ID.Hex(), "jti-telefono", middleware.RoleConsumer)
	assert.Equal(t, http.StatusNoContent, send(t, app, http.MethodPut, "/api/v1/dispositivos", map[string]string{"token": device}, token).status)
	assert.NotContains(t, env.Logs.String(), device)
}

// nv-86: la pestaña y los conteos son solo de quien pregunta; el aviso de nv-86 lleva solo ids
// (sin texto de terceros ni ubicación); sin sesión, 401.
func TestNotificacionesSecurity_CategoriasSoloMiasYSoloIDs(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	for _, path := range []string{"/api/v1/notificaciones?categoria=social", "/api/v1/notificaciones?categoria=planes"} {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, path, nil, "").status, path)
	}
	a := env.CreateUser(t, "categoria-a@wherever.test")
	b := env.CreateUser(t, "categoria-b@wherever.test")
	svc := notificaciones.NewService(notificaciones.NewRepository(env.Mongo.DB()), nil, env.Log)
	svc.Emit(context.Background(), a.ID, notificaciones.KindLike, map[string]string{"momento_id": bson.NewObjectID().Hex()})
	tokenB := testutil.ValidTokenFor(t, b.ID.Hex(), "jti-categoria-b", middleware.RoleConsumer)
	r := send(t, app, http.MethodGet, "/api/v1/notificaciones?categoria=social", nil, tokenB)
	assert.Equal(t, http.StatusOK, r.status)
	assert.Empty(t, jsonObj(t, r.body)["items"], "el me gusta de otra persona no sale")
	counts := jsonObj(t, send(t, app, http.MethodGet, "/api/v1/notificaciones/no-leidas", nil, tokenB).body)
	assert.Equal(t, float64(0), counts["total"])

	tokenA := testutil.ValidTokenFor(t, a.ID.Hex(), "jti-categoria-a", middleware.RoleConsumer)
	r = send(t, app, http.MethodGet, "/api/v1/notificaciones?categoria=social", nil, tokenA)
	items, ok := jsonObj(t, r.body)["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	datos, ok := item["datos"].(map[string]any)
	require.True(t, ok)
	for k := range datos {
		assert.True(t, strings.HasSuffix(k, "_id"), "datos solo lleva ids: "+k)
	}
}
