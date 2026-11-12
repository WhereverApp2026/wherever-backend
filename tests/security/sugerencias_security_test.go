package security

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	neo4jdrv "github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/grafo"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// red siembra cuentas con nombre de usuario y relaciones en Mongo, y deja Neo4j igual
// (como `make reconstruir-grafo`).
type red struct {
	t   *testing.T
	env *authtest.Env
}

func grafoSegEnv(t *testing.T) (*red, *fiber.App) {
	t.Helper()
	env := authEnv(t)
	env.ClearRateLimits(t)
	_, err := neo4jdrv.ExecuteQuery(context.Background(), neo4jDriver.DriverWithContext, "MATCH (n) DETACH DELETE n", nil, neo4jdrv.EagerResultTransformer)
	require.NoError(t, err)
	require.NoError(t, neo4jDriver.EnsureSchema(context.Background()))
	env.Sugerencias.WithGraph(neo4jDriver)
	return &red{t: t, env: env}, env.App()
}

func (r *red) persona(username string, oculta bool) usuarios.User {
	r.t.Helper()
	u := r.env.CreateUser(r.t, username+"@grafo-seg.test")
	set := bson.D{{Key: "nombre_usuario", Value: username}}
	if oculta {
		set = append(set, bson.E{Key: "privacidad.buscable_por_usuario", Value: false})
	}
	u, err := r.env.Users.Update(context.Background(), u.ID, set)
	require.NoError(r.t, err)
	return u
}

func (r *red) amigos(a, b usuarios.User) {
	r.t.Helper()
	now := time.Now().UTC()
	f := amistades.Friendship{ID: bson.NewObjectID(), From: a.ID, To: b.ID, Status: amistades.StatusAccepted, Pair: amistades.PairKey(a.ID, b.ID), CreatedAt: now, UpdatedAt: now}
	require.NoError(r.t, amistades.NewRepository(r.env.Mongo.DB()).Insert(context.Background(), &f))
}

func (r *red) proyectar() {
	r.t.Helper()
	db := r.env.Mongo.DB()
	w := grafo.NewWorker(r.env.Grafo, amistades.NewRepository(db), planes.NewRepository(db), neo4jDriver, r.env.Log)
	_, err := w.Rebuild(context.Background(), db)
	require.NoError(r.t, err)
}

func getGrafo(t *testing.T, app *fiber.App, path string, u usuarios.User) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if u.ID != (bson.ObjectID{}) {
		req.Header.Set("Authorization", "Bearer "+testutil.ValidTokenFor(t, u.ID.Hex(), "jti-"+u.ID.Hex(), middleware.RoleConsumer))
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}

func sugeridosSeg(t *testing.T, app *fiber.App, u usuarios.User) []string {
	t.Helper()
	status, raw := getGrafo(t, app, "/api/v1/amistades/sugerencias", u)
	require.Equal(t, http.StatusOK, status, raw)
	var body struct {
		Resultados []struct {
			Persona map[string]any `json:"persona"`
		} `json:"resultados"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &body))
	out := []string{}
	for _, r := range body.Resultados {
		assert.NotContains(t, r.Persona, "correo")
		name, _ := r.Persona["nombre_usuario"].(string)
		out = append(out, name)
	}
	return out
}

func graphSize(t *testing.T) int64 {
	t.Helper()
	size, err := neo4jDriver.GraphSize(context.Background())
	require.NoError(t, err)
	return size.Nodes + size.Relationships
}

// El id del lugar llega a Cypher solo como parámetro: un intento de inyección es un id que
// no existe (404) y el grafo queda igual.
func TestGrafoSecurity_CypherParametrizado(t *testing.T) {
	r, app := grafoSegEnv(t)
	ana, beto := r.persona("ana_cy", false), r.persona("beto_cy", false)
	r.amigos(ana, beto)
	r.proyectar()
	before := graphSize(t)

	for _, raw := range []string{"x'}) DETACH DELETE n //", "0' OR 1=1 RETURN n //", "$id"} {
		status, body := getGrafo(t, app, "/api/v1/lugares/"+url.PathEscape(raw)+"/amigos", ana)
		assert.Equal(t, http.StatusNotFound, status, body)
	}
	assert.Equal(t, before, graphSize(t))
}

// Cada quien ve solo su propio grafo: las sugerencias de una persona sin amigos van vacías
// aunque otras tengan amigos de amigos.
func TestGrafoSecurity_AislamientoEntreUsuarios(t *testing.T) {
	r, app := grafoSegEnv(t)
	ana, beto, carla := r.persona("ana_ais", false), r.persona("beto_ais", false), r.persona("carla_ais", false)
	sola := r.persona("sola_ais", false)
	r.amigos(ana, beto)
	r.amigos(beto, carla)
	r.proyectar()

	assert.Equal(t, []string{"carla_ais"}, sugeridosSeg(t, app, ana))
	assert.Empty(t, sugeridosSeg(t, app, sola))
}

// Quien apagó buscable_por_usuario no llega a nadie por sugerencias.
func TestGrafoSecurity_PrivacidadDeNoBuscable(t *testing.T) {
	r, app := grafoSegEnv(t)
	ana, beto := r.persona("ana_priv", false), r.persona("beto_priv", false)
	oculta, visible := r.persona("oculta_priv", true), r.persona("visible_priv", false)
	r.amigos(ana, beto)
	r.amigos(beto, oculta)
	r.amigos(beto, visible)
	r.proyectar()

	assert.Equal(t, []string{"visible_priv"}, sugeridosSeg(t, app, ana))
	assert.NotContains(t, sugeridosSeg(t, app, visible), "oculta_priv")
}

// Sin token, 401 en las dos rutas.
func TestGrafoSecurity_RequiereSesion(t *testing.T) {
	_, app := grafoSegEnv(t)
	for _, path := range []string{"/api/v1/amistades/sugerencias", "/api/v1/lugares/" + bson.NewObjectID().Hex() + "/amigos"} {
		status, _ := getGrafo(t, app, path, usuarios.User{})
		assert.Equal(t, http.StatusUnauthorized, status, path)
	}
}
