package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

const recoPath = "/api/v1/recomendaciones?lat=19.4326&lng=-99.1332"

// recoUser crea un usuario con los gustos de la prueba del motor (nv-23) y su token.
func recoUser(t *testing.T, email string, tastes *usuarios.Tastes, privacy usuarios.Privacy) (*fiber.App, string) {
	t.Helper()
	env := insertRecoEnv(t)
	env.Recomendaciones.WithClock(func() time.Time { return lunesMediodia })
	u := env.CreateUser(t, email)
	_, err := env.Users.Update(context.Background(), u.ID, bson.D{
		{Key: "gustos", Value: tastes}, {Key: "privacidad", Value: privacy},
	})
	require.NoError(t, err)
	return env.App(), testutil.ValidTokenFor(t, u.ID.Hex(), "jti-"+email, middleware.RoleConsumer)
}

func fullTastes() *usuarios.Tastes {
	return &usuarios.Tastes{Categories: []string{"Cafeterías", "postres"}, BudgetMin: 100, BudgetMax: 200, Allergies: "cacahuate"}
}

func recoGet(t *testing.T, app *fiber.App, path, token string) []map[string]any {
	t.Helper()
	resp := sendJSON(t, app, http.MethodGet, path, nil, token)
	raw := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	assert.Equal(t, "no-store", resp.Header.Get(fiber.HeaderCacheControl))
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &body))
	assertSnakeCase(t, body)
	out := []map[string]any{}
	for _, item := range list(t, body["items"]) {
		out = append(out, obj(t, item))
	}
	return out
}

func cardSlugs(t *testing.T, cards []map[string]any) []string {
	t.Helper()
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		out = append(out, str(t, c["slug"]))
	}
	return out
}

func cardID(t *testing.T, cards []map[string]any, slug string) string {
	t.Helper()
	for _, c := range cards {
		if c["slug"] == slug {
			return str(t, c["id"])
		}
	}
	t.Fatalf("sin %s", slug)
	return ""
}

func TestRecomendaciones_RankingEsperadoConPorQue(t *testing.T) {
	app, token := recoUser(t, "reco-ranking@wherever.test", fullTastes(), usuarios.Privacy{Learning: true, SensitiveData: true})
	cards := recoGet(t, app, recoPath, token)
	assert.Equal(t, []string{"cafe-ideal", "cafe-caro", "cafe-cierra-pronto", "rest-cerca", "bar-lejos"}, cardSlugs(t, cards))
	for _, c := range cards {
		assert.NotEmpty(t, c["por_que"], "cada tarjeta trae su porqué")
		assert.NotNil(t, c["puntaje"])
	}
	assert.Equal(t, "Te lo recomendamos porque coincide con tus gustos, entra en tu presupuesto y está cerca de ti.", cards[0]["por_que"])
	assert.NotContains(t, readAll(t, sendJSON(t, app, http.MethodGet, recoPath, nil, token)), "cacahuate", "la alergia no sale en la respuesta")
}

func TestRecomendaciones_NoMeInteresaYDeshacer(t *testing.T) {
	app, token := recoUser(t, "reco-descarte@wherever.test", fullTastes(), usuarios.Privacy{Learning: true, SensitiveData: true})
	id := cardID(t, recoGet(t, app, recoPath, token), "cafe-ideal")
	mark := "/api/v1/recomendaciones/" + id + "/no-me-interesa"

	require.Equal(t, http.StatusNoContent, sendJSON(t, app, http.MethodPost, mark, nil, token).StatusCode)
	require.Equal(t, http.StatusNoContent, sendJSON(t, app, http.MethodPost, mark, nil, token).StatusCode, "repetir no falla")
	assert.NotContains(t, cardSlugs(t, recoGet(t, app, recoPath, token)), "cafe-ideal")

	require.Equal(t, http.StatusNoContent, sendJSON(t, app, http.MethodDelete, mark, nil, token).StatusCode)
	assert.Contains(t, cardSlugs(t, recoGet(t, app, recoPath, token)), "cafe-ideal")
	assert.Equal(t, http.StatusNotFound, sendJSON(t, app, http.MethodDelete, mark, nil, token).StatusCode, "ya no estaba descartado")
}

func TestRecomendaciones_NoMeInteresaNegocioInexistente(t *testing.T) {
	app, token := recoUser(t, "reco-404@wherever.test", fullTastes(), usuarios.Privacy{Learning: true, SensitiveData: true})
	for _, id := range []string{bson.NewObjectID().Hex(), "no-es-id"} {
		resp := sendJSON(t, app, http.MethodPost, "/api/v1/recomendaciones/"+id+"/no-me-interesa", nil, token)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, id)
	}
}

func TestRecomendaciones_AprendizajeApagadoMantieneLaAlergia(t *testing.T) {
	app, token := recoUser(t, "reco-sin-aprendizaje@wherever.test", fullTastes(), usuarios.Privacy{Learning: false, SensitiveData: true})
	cards := recoGet(t, app, recoPath, token)
	slugs := cardSlugs(t, cards)
	assert.NotContains(t, slugs, "cafe-cacahuate")
	assert.NotContains(t, slugs, "cafe-sin-dato")
	for _, c := range cards {
		assert.NotContains(t, list(t, c["razones"]), "gustos")
		assert.NotContains(t, list(t, c["razones"]), "presupuesto")
	}
}

func TestRecomendaciones_UsuarioSinGustos(t *testing.T) {
	app, token := recoUser(t, "reco-sin-gustos@wherever.test", nil, usuarios.Privacy{Learning: true})
	slugs := cardSlugs(t, recoGet(t, app, recoPath+"&compania=amigos&hora=12:00", token))
	assert.Contains(t, slugs, "cafe-cacahuate", "sin alergias declaradas no hay filtro de alérgenos")
	assert.Contains(t, slugs, "cafe-sin-dato")
	assert.NotContains(t, slugs, "cafe-borrador")
}

func TestRecomendaciones_ValidacionYAutenticacion(t *testing.T) {
	app, token := recoUser(t, "reco-422@wherever.test", nil, usuarios.Privacy{})
	resp := sendJSON(t, app, http.MethodGet, "/api/v1/recomendaciones?compania=jefe", nil, token)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodGet, recoPath, nil, "").StatusCode)
}

func TestRecomendaciones_ArcoExportaYBorraDescartes(t *testing.T) {
	env := insertRecoEnv(t)
	repo := recomendaciones.NewRepository(env.Mongo.DB())
	privacy := recomendaciones.NewPrivacy(repo)
	user, business := bson.NewObjectID(), bson.NewObjectID()
	ctx := context.Background()
	require.NoError(t, repo.Mark(ctx, user, business, time.Now().UTC()))

	exported, err := privacy.Export(ctx, user)
	require.NoError(t, err)
	raw, err := json.Marshal(exported)
	require.NoError(t, err)
	assert.Contains(t, string(raw), business.Hex())

	require.NoError(t, privacy.PurgeAccount(ctx, user))
	left, err := repo.Excluded(ctx, user)
	require.NoError(t, err)
	assert.Empty(t, left)
}
