package security

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

func importPayload(slug string) []map[string]any {
	return []map[string]any{{
		"slug": slug, "nombre": "Negocio Seguridad", "categoria": "restaurante",
		"descripcion": "Negocio de prueba de seguridad", "nivel_precio": 2, "precio_promedio": 100,
		"ubicacion": map[string]any{"type": "Point", "coordinates": []float64{-99.1, 19.4}},
		"direccion": map[string]string{
			"calle": "Calle Segura", "numero": "1", "colonia": "Centro",
			"municipio": "Cuauhtémoc", "estado": "CDMX", "cp": "06000",
		},
		"telefono": "+5215512345678",
		"horarios": []map[string]any{{"dia": 1, "abre": "09:00", "cierra": "18:00", "cerrado": false}},
		"fotos": []map[string]any{
			{"url": "https://example.test/a.jpg", "tipo": "fachada", "orden": 1},
			{"url": "https://example.test/b.jpg", "tipo": "interior", "orden": 2},
			{"url": "https://example.test/c.jpg", "tipo": "producto", "orden": 3},
		},
		"servicios": []string{"wifi"}, "accesibilidad": map[string]bool{"silla_ruedas": true},
		"opciones_dieta": []string{"vegetariano"}, "alergenos": []string{"gluten"},
		"etiquetas": []string{"tacos"}, "acepta_reservas": true,
		"calificacion": map[string]any{"promedio": 4.5, "total_resenas": 10},
	}}
}

func TestNegociosSecurity_RequiresAdminAndOnlyApprovedInCatalog(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	path := "/api/v1/admin/negocios/importar"
	payload := importPayload("seguridad-restaurante")

	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodPost, path, payload, "").status)

	consumer := env.CreateUser(t, "consumidor-negocios@wherever.test")
	consumerToken := testutil.ValidTokenFor(t, consumer.ID.Hex(), "jti-consumidor-negocios", middleware.RoleConsumer)
	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodPost, path, payload, consumerToken).status)

	// Un token con rol admin pero `sub` que no es ObjectId no puede resolver el dueño.
	fakeAdmin := testutil.ValidToken(t, "jti-admin-falso", middleware.RoleAdmin)
	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodPost, path, payload, fakeAdmin).status)

	admin := env.CreateAdmin(t, "admin-negocios@wherever.test")
	adminToken := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-negocios", middleware.RoleAdmin)
	adminReply := send(t, app, http.MethodPost, path, payload, adminToken)
	require.Equal(t, http.StatusOK, adminReply.status, adminReply.body)
	var report negocios.ImportReport
	require.NoError(t, json.Unmarshal([]byte(adminReply.body), &report))
	assert.Equal(t, 1, report.Created)
	assert.Empty(t, report.Errors)

	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), bson.D{
		{Key: "estado", Value: negocios.StatusDraft},
		{Key: "categoria", Value: "restaurante"},
		{Key: "etiquetas", Value: []string{"secreto"}},
	})
	require.NoError(t, err)

	catalog := send(t, app, http.MethodGet, "/api/v1/categorias", nil, "")
	require.Equal(t, http.StatusOK, catalog.status)
	assert.NotContains(t, catalog.body, "secreto")
	assert.Contains(t, catalog.body, "tacos")
}

// TestLugaresSecurity_HiddenNeverPublic: un negocio no aprobado o eliminado no sale por
// ningún endpoint público, y ninguno expone el QR de check-in ni el dueño.
func TestLugaresSecurity_HiddenNeverPublic(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	visibleID, hiddenID := bson.NewObjectID(), bson.NewObjectID()
	doc := func(id bson.ObjectID, slug, status string) bson.D {
		return bson.D{
			{Key: "_id", Value: id}, {Key: "slug", Value: slug}, {Key: "qr_checkin", Value: "qr-" + slug},
			{Key: "nombre", Value: slug}, {Key: "estado", Value: status}, {Key: "categoria", Value: "bar"},
			{Key: "propietario_id", Value: bson.NewObjectID()},
			{Key: "ubicacion", Value: bson.D{{Key: "type", Value: "Point"}, {Key: "coordinates", Value: bson.A{-99.1332, 19.4326}}}},
		}
	}
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertMany(context.Background(), []any{
		doc(visibleID, "visible", negocios.StatusApproved),
		doc(hiddenID, "oculto-pausado", negocios.StatusPaused),
	})
	require.NoError(t, err)

	for _, path := range []string{
		"/api/v1/lugares",
		"/api/v1/lugares?lat=19.4326&lng=-99.1332",
		"/api/v1/lugares/mapa?min_lat=19.4&max_lat=19.5&min_lng=-99.2&max_lng=-99.1&zoom=14",
		"/api/v1/lugares/" + visibleID.Hex(),
	} {
		got := send(t, app, http.MethodGet, path, nil, "")
		require.Equal(t, http.StatusOK, got.status, path)
		assert.NotContains(t, got.body, "oculto-pausado", path)
		assert.NotContains(t, got.body, "qr-", path)
		assert.NotContains(t, got.body, "propietario", path)
	}
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, "/api/v1/lugares/"+hiddenID.Hex(), nil, "").status)
}
