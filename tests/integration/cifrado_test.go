package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/cifrado"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios/account"
)

// rawUser lee el documento tal como quedó en la base, sin pasar por el repositorio.
func rawUser(t *testing.T, env *authtest.Env, id bson.ObjectID) bson.M {
	t.Helper()
	var doc bson.M
	require.NoError(t, env.Mongo.DB().Collection(usuarios.Collection).FindOne(context.Background(), bson.D{{Key: "_id", Value: id}}).Decode(&doc))
	return doc
}

// rawAllergies lee `gustos.alergias` tal como quedó en la base (el driver v2 decodifica los
// subdocumentos de un bson.M como bson.D: se lee con un struct).
func rawAllergies(t *testing.T, doc bson.M) string {
	t.Helper()
	raw, err := bson.Marshal(doc)
	require.NoError(t, err)
	var out struct {
		Gustos struct {
			Alergias string `bson:"alergias"`
		} `bson:"gustos"`
	}
	require.NoError(t, bson.Unmarshal(raw, &out))
	return out.Gustos.Alergias
}

func TestCifrado_RegistroGuardaLaFechaCifrada(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/usuarios", map[string]any{
		"nombre": "Cifra", "fecha_nacimiento": "1998-04-12", "correo": "cifra@wherever.test", "password": authtest.Password,
		"acepta_terminos": true, "version_terminos": "2026-10",
	}, "")
	require.Equal(t, http.StatusCreated, resp.StatusCode, readAll(t, resp))
	u, err := env.Users.FindByEmail(context.Background(), "cifra@wherever.test")
	require.NoError(t, err)
	require.NotNil(t, u.BirthDate)
	assert.Equal(t, "1998-04-12", u.BirthDate.Format("2006-01-02"), "el repositorio la devuelve en claro")

	doc := rawUser(t, env, u.ID)
	stored, ok := doc["fecha_nacimiento"].(string)
	require.True(t, ok, "en la base es texto cifrado, no una fecha")
	assert.True(t, cifrado.EsCifrado(stored))
	assert.NotContains(t, stored, "1998")
	assert.Equal(t, true, doc["mayor_de_edad"])
}

func TestCifrado_AlergiasCifradasYLeidasEnClaro(t *testing.T) {
	env := authEnv(t)
	u := env.CreateUser(t, "alergias@wherever.test")
	_, err := env.Users.Update(context.Background(), u.ID, bson.D{
		{Key: "privacidad.datos_sensibles", Value: true},
		{Key: "gustos", Value: usuarios.Tastes{Categories: []string{"Cafeterías", "Bares", "Comercio"}, BudgetMax: 300, Allergies: "cacahuate, mariscos"}},
	})
	require.NoError(t, err)
	assert.True(t, cifrado.EsCifrado(rawAllergies(t, rawUser(t, env, u.ID))))
	assert.NotContains(t, rawAllergies(t, rawUser(t, env, u.ID)), "cacahuate")

	// La API las muestra en claro a su dueño, y la exportación ARCO también.
	token := testutil.ValidTokenFor(t, u.ID.Hex(), "jti-alergias", "consumidor")
	resp := sendJSON(t, env.App(), http.MethodGet, "/api/v1/usuarios/yo", nil, token)
	body := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	assert.Contains(t, body, "cacahuate, mariscos")
	exported, err := account.NewPrivacy(env.Users).Export(context.Background(), u.ID)
	require.NoError(t, err)
	raw, err := json.Marshal(exported)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "cacahuate, mariscos")

	// Cambiar solo `gustos.alergias` también cifra.
	_, err = env.Users.Update(context.Background(), u.ID, bson.D{{Key: "gustos.alergias", Value: "kiwi"}})
	require.NoError(t, err)
	assert.NotContains(t, rawAllergies(t, rawUser(t, env, u.ID)), "kiwi")
	got, err := env.Users.FindByID(context.Background(), u.ID)
	require.NoError(t, err)
	assert.Equal(t, "kiwi", got.Tastes.Allergies)
	assert.NotContains(t, env.Logs.String(), "cacahuate", "nada sensible en los logs")
}

func TestCifrado_ValorCopiadoAOtraCuentaNoAbre(t *testing.T) {
	env := authEnv(t)
	a := env.CreateUser(t, "origen@wherever.test")
	b := env.CreateUser(t, "destino@wherever.test")
	_, err := env.Users.Update(context.Background(), a.ID, bson.D{{Key: "gustos", Value: usuarios.Tastes{Categories: []string{"a", "b", "c"}, Allergies: "nueces"}}})
	require.NoError(t, err)
	stolen := rawAllergies(t, rawUser(t, env, a.ID))
	_, err = env.Mongo.DB().Collection(usuarios.Collection).UpdateOne(context.Background(), bson.D{{Key: "_id", Value: b.ID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "gustos", Value: bson.D{{Key: "categorias", Value: bson.A{"a", "b", "c"}}, {Key: "alergias", Value: stolen}}}}}})
	require.NoError(t, err)
	_, err = env.Users.FindByID(context.Background(), b.ID)
	assert.ErrorIs(t, err, usuarios.ErrCipher, "el dato asociado ata el valor a su cuenta")
}

func TestCifrado_MigracionIdempotente(t *testing.T) {
	env := authEnv(t)
	u := env.CreateUser(t, "legado@wherever.test")
	birth := time.Date(1990, 3, 4, 0, 0, 0, 0, time.UTC)
	// Una cuenta de antes de nv-49: todo en claro.
	_, err := env.Mongo.DB().Collection(usuarios.Collection).UpdateOne(context.Background(), bson.D{{Key: "_id", Value: u.ID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "fecha_nacimiento", Value: birth},
			{Key: "gustos", Value: bson.D{{Key: "categorias", Value: bson.A{"a", "b", "c"}}, {Key: "alergias", Value: "gluten"}}},
		}}})
	require.NoError(t, err)

	// Se lee igual antes de migrar.
	got, err := env.Users.FindByID(context.Background(), u.ID)
	require.NoError(t, err)
	assert.Equal(t, "gluten", got.Tastes.Allergies)
	assert.True(t, birth.Equal(*got.BirthDate))
	before := rawUser(t, env, u.ID)["version"]

	dry, err := env.Users.Reseal(context.Background(), false)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, dry.Pendientes, 1)
	assert.Zero(t, dry.Cifrados, "el modo de prueba no cambia nada")
	assert.Equal(t, "gluten", rawAllergies(t, rawUser(t, env, u.ID)))

	done, err := env.Users.Reseal(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, done.Pendientes, done.Cifrados)
	doc := rawUser(t, env, u.ID)
	assert.True(t, cifrado.EsCifrado(rawAllergies(t, doc)))
	stored, ok := doc["fecha_nacimiento"].(string)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(stored, "enc:v1:"))
	assert.Equal(t, before, doc["version"], "migrar no es una edición de la persona")

	again, err := env.Users.Reseal(context.Background(), true)
	require.NoError(t, err)
	assert.Zero(t, again.Pendientes, "la segunda corrida no encuentra nada")
	got, err = env.Users.FindByID(context.Background(), u.ID)
	require.NoError(t, err)
	assert.Equal(t, "gluten", got.Tastes.Allergies)
	assert.True(t, birth.Equal(*got.BirthDate))
}
