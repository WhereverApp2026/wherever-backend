package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

func negociosEnv(t *testing.T) *authtest.Env {
	t.Helper()
	return authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
}

func adminToken(t *testing.T, adminID string) string {
	t.Helper()
	return testutil.ValidTokenFor(t, adminID, "jti-admin-negocios", middleware.RoleAdmin)
}

func csvValueRow(slug, name, category string, overrides map[string]string) string {
	values := map[string]string{
		"slug":                  slug,
		"nombre":                name,
		"categoria":             category,
		"descripcion":           "Negocio de prueba",
		"nivel_precio":          "2",
		"precio_promedio":       "150",
		"latitud":               "19.4326",
		"longitud":              "-99.1332",
		"calle":                 "Avenida Ejemplo",
		"numero":                "123",
		"colonia":               "Centro",
		"municipio":             "Cuauhtémoc",
		"estado_republica":      "CDMX",
		"cp":                    "06000",
		"telefono":              "+5215512345678",
		"lun":                   "09:00-18:00",
		"mar":                   "09:00-18:00",
		"mie":                   "09:00-18:00",
		"jue":                   "09:00-18:00",
		"vie":                   "09:00-18:00",
		"sab":                   "10:00-16:00",
		"dom":                   "",
		"etiquetas":             "tacos|mexicana",
		"servicios":             "terraza|wifi",
		"accesibilidad":         "silla_ruedas|apto_ninos",
		"opciones_dieta":        "vegetariano",
		"alergenos":             "gluten|lacteos",
		"fotos":                 "https://example.test/fachada.jpg|https://example.test/interior.jpg|https://example.test/producto.jpg",
		"propietario_correo":    "",
		"calificacion_promedio": "4.5",
		"total_resenas":         "10",
		"acepta_reservas":       "true",
	}
	for key, value := range overrides {
		values[key] = value
	}
	cells := make([]string, len(negocios.CSVHeader))
	for i, header := range negocios.CSVHeader {
		cells[i] = values[header]
	}
	return strings.Join(cells, ",")
}

func csvWithRows(rows ...string) string {
	return strings.Join(negocios.CSVHeader, ",") + "\n" + strings.Join(rows, "\n") + "\n"
}

func seedCSV(n int) string {
	categories := []string{"restaurante", "cafeteria", "bar", "entretenimiento", "servicios", "comercio"}
	rows := make([]string, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, csvValueRow(
			"negocio-"+twoDigits(i),
			"Negocio "+twoDigits(i),
			categories[i%len(categories)],
			nil,
		))
	}
	return csvWithRows(rows...)
}

func twoDigits(n int) string { return fmt.Sprintf("%02d", n) }

func findBusiness(t *testing.T, env *authtest.Env, slug string) negocios.Business {
	t.Helper()
	var business negocios.Business
	err := env.Mongo.DB().Collection(negocios.Collection).FindOne(context.Background(), bson.D{{Key: "slug", Value: slug}}).Decode(&business)
	require.NoError(t, err)
	return business
}

func postCSV(t *testing.T, app *fiber.App, path, body, token string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("archivo", "negocios.csv")
	require.NoError(t, err)
	_, err = io.WriteString(part, body)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set(fiber.HeaderContentType, writer.FormDataContentType())
	if token != "" {
		req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func decodeReport(t *testing.T, resp *http.Response) negocios.ImportReport {
	t.Helper()
	var report negocios.ImportReport
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&report))
	return report
}

func TestNegociosImport_SeedsAndIsIdempotent(t *testing.T) {
	env := negociosEnv(t)
	admin := env.CreateAdmin(t, "admin-semilla@wherever.test")
	ctx := context.Background()
	body := seedCSV(24)

	report, err := env.Negocios.Import(ctx, admin.ID.Hex(), strings.NewReader(body), "csv")
	require.NoError(t, err)
	assert.Equal(t, 24, report.Created)
	assert.Equal(t, 0, report.Updated)
	assert.Empty(t, report.Errors)

	business := findBusiness(t, env, "negocio-00")
	assert.Equal(t, negocios.StatusApproved, business.State)
	assert.Equal(t, admin.ID, business.OwnerID)
	assert.Equal(t, negocios.StepComplete, business.Step)
	assert.NotEmpty(t, business.QRCheckin)
	assert.Equal(t, "negocio 00", business.NormalizedName)
	assert.NotEmpty(t, business.ScheduleMin)
	assert.Equal(t, 4.5, business.Rating.Aspectos.Sabor)
	assert.Equal(t, negocios.PlanFree, business.Plan)
	assert.False(t, business.Verified)
	assert.False(t, business.ExcellenceSeal)
	assert.Equal(t, 1, business.Version)
	require.NotNil(t, business.Revision.RevisadoPor)
	assert.Equal(t, admin.ID, *business.Revision.RevisadoPor)

	report, err = env.Negocios.Import(ctx, admin.ID.Hex(), strings.NewReader(body), "csv")
	require.NoError(t, err)
	assert.Equal(t, 0, report.Created)
	assert.Equal(t, 24, report.Updated)
	assert.Empty(t, report.Errors)

	again := findBusiness(t, env, "negocio-00")
	assert.Equal(t, business.ID, again.ID, "no duplica")
	assert.Equal(t, business.QRCheckin, again.QRCheckin, "qr estable")
	assert.Equal(t, 2, again.Version, "version sube")
}

func TestNegociosImport_PartialErrorsAndOwnerResolution(t *testing.T) {
	env := negociosEnv(t)
	admin := env.CreateAdmin(t, "admin-dueno@wherever.test")
	owner := env.CreateUser(t, "duena@wherever.test")
	body := csvWithRows(
		csvValueRow("con-duena", "Con Dueña", "restaurante", map[string]string{"propietario_correo": owner.Email}),
		csvValueRow("sin-duena", "Sin Dueña", "cafeteria", nil),
		csvValueRow("categoria-mala", "Categoría Mala", "otro", nil),
	)

	report, err := env.Negocios.Import(context.Background(), admin.ID.Hex(), strings.NewReader(body), "csv")
	require.NoError(t, err)
	assert.Equal(t, 2, report.Created)
	assert.Empty(t, report.Updated)
	require.Len(t, report.Errors, 1)
	assert.Equal(t, 3, report.Errors[0].Row)
	assert.Equal(t, "categoria", report.Errors[0].Field)

	assert.Equal(t, owner.ID, findBusiness(t, env, "con-duena").OwnerID)
	assert.Equal(t, admin.ID, findBusiness(t, env, "sin-duena").OwnerID)

	body = csvWithRows(
		csvValueRow("duena-inexistente", "Dueña Inexistente", "bar", map[string]string{"propietario_correo": "nadie@wherever.test"}),
	)
	report, err = env.Negocios.Import(context.Background(), admin.ID.Hex(), strings.NewReader(body), "csv")
	require.NoError(t, err)
	assert.Zero(t, report.Created)
	require.Len(t, report.Errors, 1)
	assert.Equal(t, "propietario_correo", report.Errors[0].Field)
}

func TestNegociosImport_PreservesControlFieldsOnReimport(t *testing.T) {
	env := negociosEnv(t)
	admin := env.CreateAdmin(t, "admin-control@wherever.test")
	body := csvWithRows(csvValueRow("control", "Control", "restaurante", nil))

	_, err := env.Negocios.Import(context.Background(), admin.ID.Hex(), strings.NewReader(body), "csv")
	require.NoError(t, err)
	first := findBusiness(t, env, "control")
	_, err = env.Mongo.DB().Collection(negocios.Collection).UpdateOne(context.Background(),
		bson.D{{Key: "slug", Value: "control"}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "cupones_activos", Value: 3},
			{Key: "sello_excelencia", Value: true},
			{Key: "calificacion.promedio", Value: 4.9},
		}}})
	require.NoError(t, err)

	report, err := env.Negocios.Import(context.Background(), admin.ID.Hex(), strings.NewReader(body), "csv")
	require.NoError(t, err)
	assert.Equal(t, 1, report.Updated)
	again := findBusiness(t, env, "control")
	assert.Equal(t, 3, again.Coupons)
	assert.True(t, again.ExcellenceSeal)
	assert.Equal(t, 4.9, again.Rating.Promedio)
	assert.Equal(t, first.QRCheckin, again.QRCheckin)
	assert.Equal(t, 2, again.Version)
}

func TestNegociosImport_HTTPMultipart(t *testing.T) {
	env := negociosEnv(t)
	admin := env.CreateAdmin(t, "admin-multipart@wherever.test")
	app := env.App()
	body := csvWithRows(csvValueRow("multipart", "Multipart", "bar", nil))

	resp := postCSV(t, app, "/api/v1/admin/negocios/importar", body, adminToken(t, admin.ID.Hex()))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	report := decodeReport(t, resp)
	assert.Equal(t, 1, report.Created)
	assert.Empty(t, report.Errors)
	assert.Equal(t, negocios.StatusApproved, findBusiness(t, env, "multipart").State)
}

func TestNegociosCategories_CountsAndExclusion(t *testing.T) {
	env := negociosEnv(t)
	ctx := context.Background()
	collection := env.Mongo.DB().Collection(negocios.Collection)
	deletedAt := time.Now().UTC()
	raw := func(i int, status, category string, tags, services []string) bson.D {
		return bson.D{
			{Key: "slug", Value: fmt.Sprintf("categoria-%02d", i)},
			{Key: "qr_checkin", Value: fmt.Sprintf("qr-%02d", i)},
			{Key: "estado", Value: status},
			{Key: "categoria", Value: category},
			{Key: "etiquetas", Value: tags},
			{Key: "servicios", Value: services},
		}
	}
	deleted := append(raw(7, negocios.StatusApproved, "restaurante", []string{"fantasma"}, []string{}),
		bson.E{Key: "eliminado_en", Value: deletedAt})
	_, err := collection.InsertMany(ctx, []any{
		raw(0, negocios.StatusApproved, "restaurante", []string{"tacos"}, []string{"terraza"}),
		raw(1, negocios.StatusApproved, "cafeteria", []string{"tacos"}, []string{}),
		raw(2, negocios.StatusApproved, "bar", []string{}, []string{}),
		raw(3, negocios.StatusDraft, "restaurante", []string{"fantasma"}, []string{}),
		raw(4, negocios.StatusReview, "restaurante", []string{"fantasma"}, []string{}),
		raw(5, negocios.StatusRejected, "restaurante", []string{"fantasma"}, []string{}),
		raw(6, negocios.StatusPaused, "restaurante", []string{"fantasma"}, []string{}),
		deleted,
	})
	require.NoError(t, err)

	app := env.App()
	resp := get(t, app, "/api/v1/categorias")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "no-store", resp.Header.Get(fiber.HeaderCacheControl))
	assert.Empty(t, resp.Header.Get(fiber.HeaderETag))
	var facets negocios.FacetsResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&facets))
	require.Len(t, facets.Categories, 6)
	counts := map[string]int{}
	for _, item := range facets.Categories {
		counts[item.ID] = item.Count
	}
	assert.Equal(t, 1, counts["restaurante"])
	assert.Equal(t, 1, counts["cafeteria"])
	assert.Equal(t, 1, counts["bar"])
	assert.Equal(t, 0, counts["entretenimiento"])
	assert.Equal(t, []negocios.ItemCount{{ID: "tacos", Name: "tacos", Count: 2}}, facets.Tags)
	assert.Equal(t, []negocios.ItemCount{{ID: "terraza", Name: "terraza", Count: 1}}, facets.Services)
	for _, item := range facets.Tags {
		assert.NotEqual(t, "fantasma", item.ID)
	}
	for _, item := range facets.Services {
		assert.NotEqual(t, "fantasma", item.ID)
	}

	resp = request(t, app, http.MethodGet, "/api/v1/categorias", map[string]string{fiber.HeaderIfNoneMatch: `"x"`})
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestNegociosCategories_NoApprovedStillReturnsSix(t *testing.T) {
	env := negociosEnv(t)
	app := env.App()

	resp := get(t, app, "/api/v1/categorias")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var facets negocios.FacetsResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&facets))
	require.Len(t, facets.Categories, 6)
	for _, item := range facets.Categories {
		assert.Zero(t, item.Count)
	}
	assert.Empty(t, facets.Tags)
	assert.Empty(t, facets.Services)
	assert.Equal(t, []int{1, 2, 3, 4}, facets.Filters.PriceLevels)
}
