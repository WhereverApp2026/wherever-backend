package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/asistente"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

const generarPath = "/api/v1/planes/generar"

// interpretacionTacos es una respuesta válida del modelo para el paso 1.
const interpretacionTacos = `{"compania":"amigos","con_ninos":false,"con_mascota":false,
	"presupuesto_max":0,"hora":"","animo":"festivo","palabras_clave":["tacos"]}`

// abiertoSiempre abre los siete días todo el día: la prueba corre con el reloj real.
func abiertoSiempre() []negocios.VentanaMin {
	out := make([]negocios.VentanaMin, 0, 7)
	for d := 0; d < 7; d++ {
		out = append(out, negocios.VentanaMin{Dia: d, Desde: 0, Hasta: 24 * 60})
	}
	return out
}

// asistenteEnv siembra 4 negocios abiertos: uno con cacahuate (alergia del perfil) que nunca
// debe salir, y una persona con gustos, alergia y presupuesto de 100 a 400.
func asistenteEnv(t *testing.T) (*authtest.Env, usuarios.User, string) {
	t.Helper()
	env := negociosEnv(t)
	specs := []struct {
		placeSpec
		allergens []string
	}{
		{placeSpec{slug: "taqueria", category: "restaurante", avgPrice: 150, rating: 4.5, dLat: 0.001}, []string{negocios.AllergenNone}},
		{placeSpec{slug: "cafe", category: "cafeteria", avgPrice: 90, rating: 4.2, dLat: 0.002}, []string{negocios.AllergenNone}},
		{placeSpec{slug: "bar", category: "bar", avgPrice: 120, rating: 4, dLat: 0.003}, []string{negocios.AllergenNone}},
		{placeSpec{slug: "cacahuates", category: "restaurante", avgPrice: 100, rating: 5}, []string{"cacahuate"}},
	}
	docs := make([]any, 0, len(specs))
	for _, s := range specs {
		s.windows = abiertoSiempre()
		b := placeDoc(s.placeSpec)
		b.Allergens, b.Tags = s.allergens, []string{"tacos"}
		docs = append(docs, b)
	}
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertMany(context.Background(), docs)
	require.NoError(t, err)

	user := env.CreateUser(t, "asistente@wherever.test")
	_, err = env.Users.Update(context.Background(), user.ID, bson.D{{Key: "gustos", Value: usuarios.Tastes{
		Categories: []string{"Restaurantes"}, BudgetMin: 100, BudgetMax: 400, Allergies: "cacahuate",
	}}})
	require.NoError(t, err)
	return env, user, testutil.ValidTokenFor(t, user.ID.Hex(), "jti-asistente", middleware.RoleConsumer)
}

func generarBody(frase string) map[string]any {
	return map[string]any{"frase": frase, "lat": centerLat, "lng": centerLng, "movilidad": "a_pie"}
}

func TestAsistente_GeneraPlanSinGuardarConCandidatosReales(t *testing.T) {
	env, user, token := asistenteEnv(t)
	env.IA.Responder(interpretacionTacos)
	frase := "unos tacos con la banda, escríbeme a yo@correo.mx"

	resp := sendJSON(t, env.App(), http.MethodPost, generarPath, generarBody(frase), token)
	require.Equal(t, http.StatusOK, resp.StatusCode, readAll(t, resp))
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	var out asistente.GenerarResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))

	assert.Equal(t, asistente.FuenteIA, out.Fuente)
	assert.NotEmpty(t, out.Aviso)
	assert.Equal(t, []string{"tacos"}, out.Parametros.PalabrasClave)
	assert.Empty(t, out.Parametros.Animo, "sin consentimiento de ánimo no se devuelve")
	require.Len(t, out.Plan.Paradas, 3)
	for _, p := range out.Plan.Paradas {
		assert.NotEqual(t, "Lugar cacahuates", p.Nombre, "la alergia del perfil es filtro duro")
		assert.NotEmpty(t, p.PorQue)
	}
	assert.Len(t, out.Plan.Traslados, 2)
	assert.Equal(t, 360, out.Plan.CostoPorPersona)
	assert.Equal(t, 400, out.Plan.Presupuesto, "presupuesto del perfil × 1 persona")

	n, err := env.Mongo.DB().Collection("planes").CountDocuments(context.Background(), bson.D{})
	require.NoError(t, err)
	assert.Zero(t, n, "el plan no se guarda")

	// Privacidad: al modelo solo llega la frase anonimizada en el turno de usuario.
	peticiones := env.IA.Peticiones()
	require.Len(t, peticiones, 1)
	p := peticiones[0]
	assert.Equal(t, "unos tacos con la banda, escríbeme a [correo]", p.TextoUsuario)
	todo := p.Instrucciones + p.Contexto + p.TextoUsuario
	for _, prohibido := range []string{user.ID.Hex(), user.Email, "yo@correo.mx", "cacahuate", fmt.Sprint(centerLat), "Prueba"} {
		assert.NotContains(t, todo, prohibido)
	}
	assert.NotContains(t, env.Logs.String(), "tacos con la banda", "la frase no va a los logs")
}

func TestAsistente_AnimoSoloConConsentimiento(t *testing.T) {
	env, user, token := asistenteEnv(t)
	_, err := env.Users.Update(context.Background(), user.ID, bson.D{{Key: "privacidad.estado_animo", Value: true}})
	require.NoError(t, err)
	env.IA.Responder(interpretacionTacos)
	resp := sendJSON(t, env.App(), http.MethodPost, generarPath, generarBody("tacos de fiesta"), token)
	require.Equal(t, http.StatusOK, resp.StatusCode, readAll(t, resp))
	assert.Equal(t, "festivo", obj(t, decode(t, resp)["parametros"])["animo"])
}

func TestAsistente_ErroresDeLaIAYSinCandidatos(t *testing.T) {
	env, _, token := asistenteEnv(t)
	app := env.App()

	env.IA.Fallar(&ia.LimiteError{Espera: 42 * time.Second})
	resp := sendJSON(t, app, http.MethodPost, generarPath, generarBody("tacos"), token)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, "42", resp.Header.Get("Retry-After"))

	env.IA.Reset()
	env.IA.Responder(`{"plan":"inventado"}`)
	resp = sendJSON(t, app, http.MethodPost, generarPath, generarBody("tacos"), token)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	env.IA.Reset()
	env.IA.Responder(interpretacionTacos)
	lejos := map[string]any{"frase": "tacos", "lat": centerLat + 1, "lng": centerLng}
	resp = sendJSON(t, app, http.MethodPost, generarPath, lejos, token)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	env.IA.Reset()
	resp = sendJSON(t, app, http.MethodPost, generarPath, generarBody(strings.Repeat("a", asistente.MaxFrase+1)), token)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Empty(t, env.IA.Peticiones(), "una frase inválida no llega al modelo")
}
