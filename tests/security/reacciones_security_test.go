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
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
	"github.com/WhereverApp2026/wherever-backend/internal/reacciones"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// momentoSembrado inserta una postal lista y su momento publicado de autor.
func momentoSembrado(t *testing.T, env *authtest.Env, autor bson.ObjectID, vis string) bson.ObjectID {
	t.Helper()
	ctx, db, now := context.Background(), env.Mongo.DB(), time.Now().UTC()
	p := postales.Postal{ID: bson.NewObjectID(), UsuarioID: autor, ImagenID: bson.NewObjectID().Hex(), ImagenEstado: "publicada",
		Estilo: postales.EstiloPolaroid, TextoEstado: postales.TextoAprobado, Visibilidad: vis, Estado: postales.EstadoLista, Version: 1, CreadoEn: now, ActualizadoEn: now}
	_, err := db.Collection(postales.Collection).InsertOne(ctx, p)
	require.NoError(t, err)
	m := postales.Momento{ID: bson.NewObjectID(), AutorID: autor, PostalID: p.ID, Visibilidad: vis, Estado: postales.MomentoPublicado,
		Etiquetados: []bson.ObjectID{}, Version: 1, CreadoEn: now, ActualizadoEn: now}
	_, err = db.Collection(postales.MomentosCollection).InsertOne(ctx, m)
	require.NoError(t, err)
	return m.ID
}

func relacionar(t *testing.T, env *authtest.Env, a, b bson.ObjectID, estado string) {
	t.Helper()
	now := time.Now().UTC()
	f := amistades.Friendship{ID: bson.NewObjectID(), From: a, To: b, Status: estado, Pair: amistades.PairKey(a, b), CreatedAt: now, UpdatedAt: now}
	require.NoError(t, amistades.NewRepository(env.Mongo.DB()).Insert(context.Background(), &f))
}

// Sin token = 401 y con la cuenta en eliminación pendiente = 403 en las 11 rutas de nv-83.
func TestReaccionesSecurity_SesionYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	pending := env.CreateUser(t, "pendiente-reacc@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	pendiente := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-reacc", middleware.RoleConsumer)
	id := bson.NewObjectID().Hex()
	rutas := [][2]string{{http.MethodDelete, "/api/v1/comentarios/" + id}}
	for _, o := range []string{"momentos", "planes"} {
		base := "/api/v1/" + o + "/" + id
		rutas = append(rutas, [2]string{http.MethodPut, base + "/me-gusta"}, [2]string{http.MethodDelete, base + "/me-gusta"},
			[2]string{http.MethodGet, base + "/comentarios"}, [2]string{http.MethodPost, base + "/comentarios"}, [2]string{http.MethodGet, base + "/reacciones"})
	}
	for _, r := range rutas {
		body := map[string]any{"texto": "hola"}
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r[0], r[1], body, "").status, r[0]+" "+r[1])
		assert.Equal(t, http.StatusForbidden, send(t, app, r[0], r[1], body, pendiente).status, r[0]+" "+r[1])
	}
}

// Nadie interactúa con lo que no puede ver: sin permiso o con bloqueo (en cualquier sentido) =
// 404, igual que un id inexistente; y no se borra un comentario ajeno de un objeto ajeno.
func TestReaccionesSecurity_VisibilidadBloqueoYPropiedad(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	ana := env.CreateUser(t, "ana-reacc@wherever.test")
	beto := env.CreateUser(t, "beto-reacc@wherever.test")
	cleo := env.CreateUser(t, "cleo-reacc@wherever.test")
	bloq := env.CreateUser(t, "bloq-reacc@wherever.test")
	relacionar(t, env, ana.ID, beto.ID, amistades.StatusAccepted)
	relacionar(t, env, ana.ID, cleo.ID, amistades.StatusAccepted)
	relacionar(t, env, bloq.ID, ana.ID, amistades.StatusBlocked)
	tok := func(u usuarios.User) string {
		return testutil.ValidTokenFor(t, u.ID.Hex(), "jti-"+u.ID.Hex(), middleware.RoleConsumer)
	}
	soloAmigos := momentoSembrado(t, env, ana.ID, postales.VisAmigos)
	publico := momentoSembrado(t, env, ana.ID, postales.VisPublica)
	extrana := env.CreateUser(t, "extrana-reacc@wherever.test")

	// Antes: la amiga sí puede (la prueba no pasa por un dato mal sembrado).
	require.Equal(t, http.StatusNoContent, send(t, app, http.MethodPut, "/api/v1/momentos/"+soloAmigos.Hex()+"/me-gusta", nil, tok(beto)).status)
	for _, caso := range []struct {
		nombre string
		token  string
		id     bson.ObjectID
	}{
		{"extraña en momento de amigos", tok(extrana), soloAmigos},
		{"bloqueada en momento público", tok(bloq), publico},
		{"inexistente", tok(beto), bson.NewObjectID()},
	} {
		base := "/api/v1/momentos/" + caso.id.Hex()
		assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodPut, base+"/me-gusta", nil, caso.token).status, caso.nombre)
		assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, base+"/comentarios", nil, caso.token).status, caso.nombre)
		assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodPost, base+"/comentarios", map[string]any{"texto": "hola"}, caso.token).status, caso.nombre)
		assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, base+"/reacciones", nil, caso.token).status, caso.nombre)
	}
	// Bloqueo en el otro sentido: la autora tampoco ve a quien bloqueó en la lista pública.
	publicoBloq := momentoSembrado(t, env, bloq.ID, postales.VisPublica)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodPut, "/api/v1/momentos/"+publicoBloq.Hex()+"/me-gusta", nil, tok(ana)).status)

	// Comentario de Beto en el momento de Ana: Cleo lo ve pero no es suyo ni del objeto = 403.
	out := send(t, app, http.MethodPost, "/api/v1/momentos/"+soloAmigos.Hex()+"/comentarios", map[string]any{"texto": "Hola Ana"}, tok(beto))
	require.Equal(t, http.StatusCreated, out.status, out.body)
	for _, campo := range []string{"correo", "telefono", "telefono_hash"} {
		assert.NotContains(t, out.body, campo)
	}
	var c reacciones.Comentario
	require.NoError(t, env.Mongo.DB().Collection(reacciones.ComentariosCollection).FindOne(context.Background(), bson.D{{Key: "autor_id", Value: beto.ID}}).Decode(&c))
	borrar := "/api/v1/comentarios/" + c.ID.Hex()
	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodDelete, borrar, nil, tok(cleo)).status)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodDelete, borrar, nil, tok(extrana)).status, "quien no ve el objeto no sabe que existe")
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodDelete, "/api/v1/comentarios/no-es-id", nil, tok(cleo)).status)

	lista := send(t, app, http.MethodGet, "/api/v1/momentos/"+soloAmigos.Hex()+"/reacciones", nil, tok(cleo))
	require.Equal(t, http.StatusOK, lista.status, lista.body)
	assert.Contains(t, lista.body, beto.ID.Hex())
	for _, campo := range []string{"correo", "telefono", "@wherever.test"} {
		assert.NotContains(t, lista.body, campo)
	}
	assert.Equal(t, http.StatusNoContent, send(t, app, http.MethodDelete, borrar, nil, tok(ana)).status, "la dueña del momento sí")
}

// Me gusta y comentarios comparten 60 escrituras por hora por usuario; leer no cuenta.
func TestReaccionesSecurity_Limite60PorHora(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	u := env.CreateUser(t, "limite-reacc@wherever.test")
	token := testutil.ValidTokenFor(t, u.ID.Hex(), "jti-limite-reacc", middleware.RoleConsumer)
	path := "/api/v1/momentos/" + bson.NewObjectID().Hex()
	for i := range 60 {
		method, sub, body := http.MethodPut, "/me-gusta", any(nil)
		if i%2 == 1 {
			method, sub, body = http.MethodPost, "/comentarios", map[string]any{"texto": "hola"}
		}
		require.Equal(t, http.StatusNotFound, send(t, app, method, path+sub, body, token).status)
	}
	assert.Equal(t, http.StatusTooManyRequests, send(t, app, http.MethodDelete, path+"/me-gusta", nil, token).status)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, path+"/comentarios", nil, token).status, "leer no cuenta")
	otro := env.CreateUser(t, "limite-reacc-otro@wherever.test")
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodPut, path+"/me-gusta", nil, testutil.ValidTokenFor(t, otro.ID.Hex(), "jti-limite-otro", middleware.RoleConsumer)).status)
}
