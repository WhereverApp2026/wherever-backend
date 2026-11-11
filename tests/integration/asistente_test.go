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

func generarBody(frase string, inicio time.Time) map[string]any {
	return map[string]any{"frase": frase, "lat": centerLat, "lng": centerLng, "movilidad": "a_pie", "fecha": inicio.Format(time.RFC3339)}
}

// inicioPrueba es dentro de 2 horas, en minuto exacto: la prueba corre con el reloj real.
func inicioPrueba() time.Time { return time.Now().Add(2 * time.Hour).Truncate(time.Minute) }

// idDe lee el id sembrado de un negocio por su slug.
func idDe(t *testing.T, env *authtest.Env, slug string) string {
	t.Helper()
	var b negocios.Business
	require.NoError(t, env.Mongo.DB().Collection(negocios.Collection).FindOne(context.Background(), bson.D{{Key: "slug", Value: slug}}).Decode(&b))
	return b.ID.Hex()
}

// paradaPrueba es una parada del paso 2: el id y los minutos desde el inicio.
type paradaPrueba struct {
	id     string
	offset int
}

// itinerario es una respuesta del paso 2 con la hora local del piloto, como la escribe la IA.
func itinerario(t *testing.T, inicio time.Time, paradas ...paradaPrueba) string {
	t.Helper()
	loc, err := time.LoadLocation("America/Mexico_City")
	require.NoError(t, err)
	partes := make([]string, 0, len(paradas))
	for _, p := range paradas {
		llega := inicio.Add(time.Duration(p.offset) * time.Minute).In(loc).Format("15:04")
		partes = append(partes, fmt.Sprintf(`{"id":%q,"llega":%q,"minutos":60}`, p.id, llega))
	}
	return `{"paradas":[` + strings.Join(partes, ",") + `]}`
}

func TestAsistente_GeneraPlanSinGuardarConCandidatosReales(t *testing.T) {
	env, user, token := asistenteEnv(t)
	inicio := inicioPrueba()
	inventado := bson.NewObjectID().Hex()
	// La IA propone el negocio con cacahuate (existe, pero la alergia lo sacó de los
	// candidatos) y un id inventado: la API descarta ambos, los registra y completa con el motor.
	env.IA.Responder(interpretacionTacos).Responder(itinerario(t, inicio,
		paradaPrueba{idDe(t, env, "taqueria"), 0}, paradaPrueba{idDe(t, env, "cacahuates"), 70},
		paradaPrueba{inventado, 70}, paradaPrueba{idDe(t, env, "cafe"), 70}))
	frase := "unos tacos con la banda, escríbeme a yo@correo.mx"

	resp := sendJSON(t, env.App(), http.MethodPost, generarPath, generarBody(frase, inicio), token)
	require.Equal(t, http.StatusOK, resp.StatusCode, readAll(t, resp))
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	var out asistente.GenerarResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))

	assert.Equal(t, asistente.FuenteIA, out.Fuente)
	assert.NotEmpty(t, out.Aviso)
	assert.Equal(t, []string{"tacos"}, out.Parametros.PalabrasClave)
	assert.Empty(t, out.Parametros.Animo, "sin consentimiento de ánimo no se devuelve")
	require.Len(t, out.Plan.Paradas, 3)
	assert.Equal(t, []string{"Lugar taqueria", "Lugar cafe", "Lugar bar"},
		[]string{out.Plan.Paradas[0].Nombre, out.Plan.Paradas[1].Nombre, out.Plan.Paradas[2].Nombre})
	assert.Equal(t, inicio.Unix(), out.Plan.Paradas[0].Llega.Unix(), "se conserva la hora de la IA")
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

	// Verificación: cada descarte queda en ia_descartes con su motivo y sin datos de la persona.
	var descartes []struct {
		Motivo    string        `bson:"motivo"`
		NegocioID bson.ObjectID `bson:"negocio_id"`
		Modelo    string        `bson:"modelo"`
	}
	cur, err := env.Mongo.DB().Collection(asistente.DescartesCollection).Find(context.Background(), bson.D{})
	require.NoError(t, err)
	require.NoError(t, cur.All(context.Background(), &descartes))
	require.Len(t, descartes, 2)
	ids := []string{}
	for _, d := range descartes {
		assert.Equal(t, asistente.MotivoFueraDeCandidatos, d.Motivo)
		assert.Equal(t, "doble", d.Modelo)
		ids = append(ids, d.NegocioID.Hex())
	}
	assert.ElementsMatch(t, []string{idDe(t, env, "cacahuates"), inventado}, ids)

	// Privacidad: al modelo solo llega la frase anonimizada en el turno de usuario; en el
	// paso 2, los candidatos públicos sin el negocio con el alérgeno y sin la frase.
	peticiones := env.IA.Peticiones()
	require.Len(t, peticiones, 2)
	assert.Equal(t, "unos tacos con la banda, escríbeme a [correo]", peticiones[0].TextoUsuario)
	for _, p := range peticiones {
		todo := p.Instrucciones + p.Contexto + p.TextoUsuario
		for _, prohibido := range []string{user.ID.Hex(), user.Email, "yo@correo.mx", "cacahuate", fmt.Sprint(centerLat), "Prueba"} {
			assert.NotContains(t, todo, prohibido)
		}
	}
	assert.NotContains(t, peticiones[1].TextoUsuario, "con la banda")
	assert.Contains(t, peticiones[1].Contexto, idDe(t, env, "taqueria"))
	assert.NotContains(t, env.Logs.String(), "tacos con la banda", "la frase no va a los logs")
}

func TestAsistente_AnimoSoloConConsentimiento(t *testing.T) {
	env, user, token := asistenteEnv(t)
	_, err := env.Users.Update(context.Background(), user.ID, bson.D{{Key: "privacidad.estado_animo", Value: true}})
	require.NoError(t, err)
	inicio := inicioPrueba()
	env.IA.Responder(interpretacionTacos).Responder(itinerario(t, inicio,
		paradaPrueba{idDe(t, env, "taqueria"), 0}, paradaPrueba{idDe(t, env, "bar"), 70}))
	resp := sendJSON(t, env.App(), http.MethodPost, generarPath, generarBody("tacos de fiesta", inicio), token)
	require.Equal(t, http.StatusOK, resp.StatusCode, readAll(t, resp))
	assert.Equal(t, "festivo", obj(t, decode(t, resp)["parametros"])["animo"])
	assert.Contains(t, env.IA.Peticiones()[1].TextoUsuario, "ánimo festivo", "con consentimiento el ánimo orienta el itinerario")
}

func TestAsistente_ErroresDeLaIAYSinCandidatos(t *testing.T) {
	env, _, token := asistenteEnv(t)
	app := env.App()
	inicio := inicioPrueba()

	env.IA.Fallar(&ia.LimiteError{Espera: 42 * time.Second})
	resp := sendJSON(t, app, http.MethodPost, generarPath, generarBody("tacos", inicio), token)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, "42", resp.Header.Get("Retry-After"))

	env.IA.Reset()
	env.IA.Responder(`{"plan":"inventado"}`)
	resp = sendJSON(t, app, http.MethodPost, generarPath, generarBody("tacos", inicio), token)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	// Un itinerario con forma inválida dos veces también es 503.
	env.IA.Reset()
	env.IA.Responder(interpretacionTacos).Responder(`{"paradas":[]}`)
	resp = sendJSON(t, app, http.MethodPost, generarPath, generarBody("tacos", inicio), token)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Len(t, env.IA.Peticiones(), 3, "interpretar + itinerario con un reintento")

	env.IA.Reset()
	env.IA.Responder(interpretacionTacos)
	lejos := map[string]any{"frase": "tacos", "lat": centerLat + 1, "lng": centerLng}
	resp = sendJSON(t, app, http.MethodPost, generarPath, lejos, token)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Len(t, env.IA.Peticiones(), 1, "sin candidatos no se pide itinerario")

	env.IA.Reset()
	resp = sendJSON(t, app, http.MethodPost, generarPath, generarBody(strings.Repeat("a", asistente.MaxFrase+1), inicio), token)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Empty(t, env.IA.Peticiones(), "una frase inválida no llega al modelo")
}
