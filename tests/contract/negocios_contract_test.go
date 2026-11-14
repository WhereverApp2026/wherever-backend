package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

func contractBusiness(slug string) negocios.ImportRow {
	return negocios.ImportRow{
		Slug: slug, Name: "Contrato Negocio", Category: "restaurante",
		Description: "Negocio para el contrato", PriceLevel: 2, AveragePrice: 150,
		Location: negocios.Location{Type: "Point", Coordinates: []float64{-99.1332, 19.4326}},
		Address: negocios.Direccion{
			Calle: "Calle Contrato", Numero: "1", Colonia: "Centro",
			Municipio: "Cuauhtémoc", Estado: "CDMX", CP: "06000",
		},
		Phone: "+5215512345678",
		Schedules: []negocios.Horario{
			{Dia: 1, Abre: "09:00", Cierra: "18:00"},
		},
		Photos: []negocios.Foto{
			{URL: "https://example.test/fachada.jpg", Tipo: "fachada", Orden: 1},
			{URL: "https://example.test/interior.jpg", Tipo: "interior", Orden: 2},
			{URL: "https://example.test/producto.jpg", Tipo: "producto", Orden: 3},
		},
		Services:            []string{"terraza"},
		Accessibility:       negocios.Accesibilidad{SillaRuedas: true},
		DietOptions:         []string{"vegetariano"},
		Allergens:           []string{"gluten"},
		Tags:                []string{"tacos"},
		AcceptsReservations: true,
		Rating:              negocios.Calificacion{Promedio: 4.5, TotalResenas: 10},
	}
}

func TestContract_Negocios(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	admin := env.CreateAdmin(t, "admin-contrato@wherever.test")

	first, err := json.Marshal([]negocios.ImportRow{contractBusiness("contrato-categorias")})
	require.NoError(t, err)
	_, err = env.Negocios.Import(context.Background(), admin.ID.Hex(), bytes.NewReader(first), "json")
	require.NoError(t, err)

	app := env.App()
	status, body := call(t, app, http.MethodGet, "/api/v1/categorias", nil, "")
	check(t, "negocios/categorias_200", status, body)

	second := []negocios.ImportRow{contractBusiness("contrato-importado")}
	token := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-contrato-negocios", middleware.RoleAdmin)
	status, body = call(t, app, http.MethodPost, "/api/v1/admin/negocios/importar", second, token)
	check(t, "negocios/importar_200", status, body)

	status, body = call(t, app, http.MethodGet, "/api/v1/lugares?lat=19.4326&lng=-99.1332&limite=1", nil, "")
	check(t, "negocios/lugares_200", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/lugares?orden=distancia", nil, "")
	check(t, "negocios/lugares_422", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/lugares/mapa?min_lat=19.4&max_lat=19.5&min_lng=-99.2&max_lng=-99.1&zoom=14", nil, "")
	check(t, "negocios/lugares_mapa_200", status, body)
	var business negocios.Business
	require.NoError(t, env.Mongo.DB().Collection(negocios.Collection).
		FindOne(context.Background(), bson.D{{Key: "slug", Value: "contrato-importado"}}).Decode(&business))
	status, body = call(t, app, http.MethodGet, "/api/v1/lugares/"+business.ID.Hex(), nil, "")
	check(t, "negocios/lugares_ficha_200", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/lugares/"+business.OwnerID.Hex(), nil, "")
	check(t, "negocios/lugares_ficha_404", status, body)
}

// TestContract_NegociosAlta fija el alta en borrador y la cola del administrador (nv-55).
func TestContract_NegociosAlta(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	duena := env.CreateUser(t, "duena-contrato@wherever.test")
	token := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-contrato", middleware.RoleConsumer)
	status, body := call(t, app, http.MethodPost, "/api/v1/negocios", map[string]string{"nombre": "Contrato Alta", "categoria": "restaurante", "descripcion": "Alta de contrato"}, token)
	check(t, "negocios/alta_crear_201", status, body)

	env.AltaEnRevision(t, duena, "Contrato Revision")
	admin := env.CreateAdmin(t, "admin-alta-contrato@wherever.test")
	adminToken := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-alta-contrato", middleware.RoleAdmin)
	status, body = call(t, app, http.MethodGet, "/api/v1/admin/negocios?estado=en_revision", nil, adminToken)
	check(t, "negocios/admin_cola_200", status, body)
}
