package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
	"github.com/WhereverApp2026/wherever-backend/internal/reacciones"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

const comentariosPath = "/api/v1/comentarios/"

// momentoDe publica un momento de who (postal con foto aprobada) y devuelve su id.
func momentoDe(t *testing.T, s postalesEnv, who amigo, vis string) string {
	t.Helper()
	postal := crearPostal(t, s, who, cuerpoPostal(fotoPublicada(t, s, who), vis))
	m := pcall(t, s.app, http.MethodPost, momentosPath, map[string]any{"postal_id": postal, "visibilidad": vis}, who.token, http.StatusCreated)
	return str(t, m["id"])
}

// contadoresMomento lee `reacciones_total` y `comentarios_total` como los ve la autora.
func contadoresMomento(t *testing.T, s postalesEnv, autora amigo, mid string) (float64, float64) {
	t.Helper()
	m := pcall(t, s.app, http.MethodGet, momentosPath+"/"+mid, nil, autora.token, http.StatusOK)
	return num(t, m["reacciones_total"]), num(t, m["comentarios_total"])
}

func num(t *testing.T, v any) float64 {
	t.Helper()
	f, ok := v.(float64)
	require.True(t, ok, "se esperaba número: %v", v)
	return f
}

// vencerAgrupados simula que pasaron los 10 minutos de los avisos agrupados.
func vencerAgrupados(t *testing.T, s postalesEnv) {
	t.Helper()
	_, err := s.env.Mongo.DB().Collection(notificaciones.GroupedCollection).UpdateMany(context.Background(), bson.D{},
		bson.D{{Key: "$set", Value: bson.D{{Key: "hasta", Value: time.Now().UTC().Add(-time.Minute)}}}})
	require.NoError(t, err)
}

func TestReacciones_MeGustaIdempotenteConcurrenteYAvisos(t *testing.T) {
	s := nuevoPostales(t)
	yo, amiga, otra := seedAmigo(t, s.env, "mg-autora"), seedAmigo(t, s.env, "mg-amiga"), seedAmigo(t, s.env, "mg-otra")
	befriend(t, s.env, yo, amiga)
	befriend(t, s.env, yo, otra)
	mid := momentoDe(t, s, yo, postales.VisAmigos)
	path := momentosPath + "/" + mid + "/me-gusta"

	pcall(t, s.app, http.MethodPut, path, nil, amiga.token, http.StatusNoContent)
	pcall(t, s.app, http.MethodPut, path, nil, amiga.token, http.StatusNoContent)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- s.env.Reacciones.DarMeGusta(context.Background(), claimsDe(otra.user.ID), reacciones.ObjetoMomento, mid)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	likes, _ := contadoresMomento(t, s, yo, mid)
	assert.Equal(t, float64(2), likes, "un me gusta por persona aunque llegue repetido o a la vez")
	assert.Equal(t, int64(1), avisos(t, s, yo, notificaciones.KindLike), "agrupado: un aviso por objeto cada 10 min")

	pcall(t, s.app, http.MethodPut, path, nil, yo.token, http.StatusNoContent)
	assert.Equal(t, int64(1), avisos(t, s, yo, notificaciones.KindLike), "el propio no avisa")
	pcall(t, s.app, http.MethodDelete, path, nil, amiga.token, http.StatusNoContent)
	pcall(t, s.app, http.MethodDelete, path, nil, amiga.token, http.StatusNoContent)
	likes, _ = contadoresMomento(t, s, yo, mid)
	assert.Equal(t, float64(2), likes)

	vencerAgrupados(t, s)
	pcall(t, s.app, http.MethodPut, path, nil, amiga.token, http.StatusNoContent)
	assert.Equal(t, int64(2), avisos(t, s, yo, notificaciones.KindLike), "pasado el lapso vuelve a avisar")
	n, err := s.env.Mongo.DB().Collection(notificaciones.Collection).CountDocuments(context.Background(),
		bson.D{{Key: "usuario_id", Value: yo.user.ID}, {Key: "datos.momento_id", Value: mid}})
	require.NoError(t, err)
	assert.Equal(t, int64(2), n, "el aviso solo lleva el id del momento")

	// Planes: privado = 404; público = 204 y suma a `me_gusta_total`.
	plan := planFuturo(t, s, yo)
	planPath := "/api/v1/planes/" + plan + "/me-gusta"
	pcall(t, s.app, http.MethodPut, planPath, nil, amiga.token, http.StatusNotFound)
	setDoc(t, s.env, planes.Collection, oidDe(t, plan), bson.D{{Key: "visibilidad", Value: planes.VisibilityPublic}})
	pcall(t, s.app, http.MethodPut, planPath, nil, amiga.token, http.StatusNoContent)
	pcall(t, s.app, http.MethodPut, planPath, nil, amiga.token, http.StatusNoContent)
	p := pcall(t, s.app, http.MethodGet, "/api/v1/planes/"+plan, nil, yo.token, http.StatusOK)
	assert.Equal(t, float64(1), p["me_gusta_total"])
	pcall(t, s.app, http.MethodPut, "/api/v1/planes/"+bson.NewObjectID().Hex()+"/me-gusta", nil, amiga.token, http.StatusNotFound)
}

func TestReacciones_ComentariosRespuestasModeracionYBorrado(t *testing.T) {
	s := nuevoPostales(t)
	yo, amiga, otra := seedAmigo(t, s.env, "rc-autora"), seedAmigo(t, s.env, "rc-amiga"), seedAmigo(t, s.env, "rc-otra")
	befriend(t, s.env, yo, amiga)
	befriend(t, s.env, yo, otra)
	mid := momentoDe(t, s, yo, postales.VisAmigos)
	path := momentosPath + "/" + mid + "/comentarios"

	c1 := pcall(t, s.app, http.MethodPost, path, map[string]any{"texto": "  ¡Qué bonito!  "}, amiga.token, http.StatusCreated)
	assert.Equal(t, "¡Qué bonito!", c1["texto"])
	assert.Equal(t, reacciones.EstadoPublicado, c1["estado"])
	assert.Equal(t, "rc-amiga", obj(t, c1["autor"])["nombre_usuario"])
	assert.Equal(t, true, c1["propio"])
	pcall(t, s.app, http.MethodPost, path, map[string]any{"texto": "   "}, amiga.token, http.StatusUnprocessableEntity)
	r1 := pcall(t, s.app, http.MethodPost, path, map[string]any{"texto": "Gracias", "padre_id": c1["id"]}, yo.token, http.StatusCreated)
	assert.Equal(t, c1["id"], r1["padre_id"])
	pcall(t, s.app, http.MethodPost, path, map[string]any{"texto": "x", "padre_id": r1["id"]}, amiga.token, http.StatusUnprocessableEntity)
	pcall(t, s.app, http.MethodPost, path, map[string]any{"texto": "x", "padre_id": bson.NewObjectID().Hex()}, amiga.token, http.StatusUnprocessableEntity)
	assert.Equal(t, int64(1), avisos(t, s, yo, notificaciones.KindComment), "la respuesta propia no avisa")

	// Moderación: rechazado = 422 sin guardar; dudoso = en moderación, solo lo ve su autora.
	s.env.ModeradorSocial.Fijar(moderacion.VeredictoRechazado)
	pcall(t, s.app, http.MethodPost, path, map[string]any{"texto": "feo"}, otra.token, http.StatusUnprocessableEntity)
	s.env.ModeradorSocial.Fijar(moderacion.VeredictoCuarentena)
	dud := pcall(t, s.app, http.MethodPost, path, map[string]any{"texto": "dudoso"}, otra.token, http.StatusCreated)
	assert.Equal(t, reacciones.EstadoEnModeracion, dud["estado"])
	s.env.ModeradorSocial.Fijar(moderacion.VeredictoAprobado)
	assert.Len(t, list(t, pcall(t, s.app, http.MethodGet, path, nil, otra.token, http.StatusOK)["items"]), 2)
	items := list(t, pcall(t, s.app, http.MethodGet, path, nil, amiga.token, http.StatusOK)["items"])
	require.Len(t, items, 1)
	assert.Len(t, list(t, obj(t, items[0])["respuestas"]), 1)
	_, total := contadoresMomento(t, s, yo, mid)
	assert.Equal(t, float64(2), total, "solo cuentan los publicados")
	decidirCola(t, s.env, str(t, dud["id"]), "aprobado")
	assert.Len(t, list(t, pcall(t, s.app, http.MethodGet, path, nil, amiga.token, http.StatusOK)["items"]), 2)
	_, total = contadoresMomento(t, s, yo, mid)
	assert.Equal(t, float64(3), total)

	// Borrar: ni autora ni dueña = 403; la dueña del momento sí; queda «Comentario eliminado».
	pcall(t, s.app, http.MethodDelete, comentariosPath+str(t, c1["id"]), nil, otra.token, http.StatusForbidden)
	pcall(t, s.app, http.MethodDelete, comentariosPath+str(t, c1["id"]), nil, yo.token, http.StatusNoContent)
	pcall(t, s.app, http.MethodDelete, comentariosPath+str(t, c1["id"]), nil, yo.token, http.StatusNotFound)
	pcall(t, s.app, http.MethodDelete, comentariosPath+str(t, dud["id"]), nil, otra.token, http.StatusNoContent)
	items = list(t, pcall(t, s.app, http.MethodGet, path, nil, amiga.token, http.StatusOK)["items"])
	require.Len(t, items, 1, "el eliminado con respuestas se queda; el otro ya no sale")
	hilo := obj(t, items[0])
	assert.Equal(t, reacciones.EstadoEliminado, hilo["estado"])
	assert.Nil(t, hilo["autor"])
	assert.Empty(t, hilo["texto"])
	assert.Len(t, list(t, hilo["respuestas"]), 1)
	_, total = contadoresMomento(t, s, yo, mid)
	assert.Equal(t, float64(1), total)

	// Paginación del más viejo al más nuevo con cursor atado a quien mira.
	pcall(t, s.app, http.MethodPost, path, map[string]any{"texto": "otro más"}, amiga.token, http.StatusCreated)
	page := pcall(t, s.app, http.MethodGet, path+"?limite=2", nil, amiga.token, http.StatusOK)
	assert.Len(t, list(t, page["items"]), 1, "el eliminado sin respuestas ocupa lugar pero no sale")
	next := str(t, page["siguiente_cursor"])
	require.NotEmpty(t, next)
	page = pcall(t, s.app, http.MethodGet, path+"?limite=2&cursor="+next, nil, amiga.token, http.StatusOK)
	assert.Equal(t, "otro más", obj(t, list(t, page["items"])[0])["texto"])
	pcall(t, s.app, http.MethodGet, path+"?limite=2&cursor="+next, nil, otra.token, http.StatusUnprocessableEntity)
}

func TestReacciones_PestanasDeUnPlan(t *testing.T) {
	s := nuevoPostales(t)
	yo, amiga, otra := seedAmigo(t, s.env, "rp-dueno"), seedAmigo(t, s.env, "rp-amiga"), seedAmigo(t, s.env, "rp-otra")
	befriend(t, s.env, yo, amiga)
	befriend(t, s.env, yo, otra)
	plan := planFuturo(t, s, yo)
	setDoc(t, s.env, planes.Collection, oidDe(t, plan), bson.D{{Key: "visibilidad", Value: planes.VisibilityPublic}})
	grupoCon(t, s, plan, yo, otra, grupos.StatusConfirmed)
	for _, who := range []amigo{amiga, otra} {
		pcall(t, s.app, http.MethodPut, "/api/v1/planes/"+plan+"/me-gusta", nil, who.token, http.StatusNoContent)
	}
	path := "/api/v1/planes/" + plan + "/reacciones"

	out := pcall(t, s.app, http.MethodGet, path, nil, yo.token, http.StatusOK)
	assert.Equal(t, map[string]any{"todos": float64(2), "me_gusta": float64(2), "van": float64(1)}, out["totales"])
	assert.Len(t, list(t, out["personas"]), 2)
	van := list(t, pcall(t, s.app, http.MethodGet, path+"?pestana=van", nil, yo.token, http.StatusOK)["personas"])
	require.Len(t, van, 1)
	p := obj(t, van[0])
	assert.Equal(t, otra.id(), p["id"])
	assert.Equal(t, true, p["va"])
	assert.Equal(t, true, p["me_gusta"])
	assert.Nil(t, p["correo"])

	// Vista de la amiga: ella es `yo`, la otra es `ninguna` con 1 amigo en común (el dueño).
	gente := list(t, pcall(t, s.app, http.MethodGet, path+"?pestana=me_gusta", nil, amiga.token, http.StatusOK)["personas"])
	require.Len(t, gente, 2)
	rel := map[string]any{}
	for _, g := range gente {
		rel[str(t, obj(t, g)["id"])] = obj(t, g)
	}
	assert.Equal(t, reacciones.RelacionYo, obj(t, rel[amiga.id()])["relacion"])
	assert.Equal(t, reacciones.RelacionNinguna, obj(t, rel[otra.id()])["relacion"])
	assert.Equal(t, float64(1), obj(t, rel[otra.id()])["amigos_en_comun"])
	pcall(t, s.app, http.MethodGet, path+"?pestana=otra", nil, yo.token, http.StatusUnprocessableEntity)

	// `actividad: nadie` (nv-82): sale para el dueño del plan, no para terceros; el total no cambia.
	setDoc(t, s.env, usuarios.Collection, otra.user.ID, bson.D{{Key: "privacidad.social.actividad", Value: usuarios.ActivityNobody}})
	out = pcall(t, s.app, http.MethodGet, path, nil, amiga.token, http.StatusOK)
	assert.Len(t, list(t, out["personas"]), 1)
	assert.Equal(t, float64(2), obj(t, out["totales"])["todos"])
	assert.Len(t, list(t, pcall(t, s.app, http.MethodGet, path, nil, yo.token, http.StatusOK)["personas"]), 2)

	// Momentos: no hay «van».
	mid := momentoDe(t, s, yo, postales.VisAmigos)
	out = pcall(t, s.app, http.MethodGet, momentosPath+"/"+mid+"/reacciones?pestana=van", nil, amiga.token, http.StatusOK)
	assert.Empty(t, list(t, out["personas"]))
}

func TestReacciones_ARCO(t *testing.T) {
	s := nuevoPostales(t)
	yo, amiga := seedAmigo(t, s.env, "ra-autora"), seedAmigo(t, s.env, "ra-amiga")
	befriend(t, s.env, yo, amiga)
	mid := momentoDe(t, s, yo, postales.VisAmigos)
	pcall(t, s.app, http.MethodPut, momentosPath+"/"+mid+"/me-gusta", nil, amiga.token, http.StatusNoContent)
	c := pcall(t, s.app, http.MethodPost, momentosPath+"/"+mid+"/comentarios", map[string]any{"texto": "Comentario de ella"}, amiga.token, http.StatusCreated)
	pcall(t, s.app, http.MethodPost, momentosPath+"/"+mid+"/comentarios", map[string]any{"texto": "Respuesta mía", "padre_id": c["id"]}, yo.token, http.StatusCreated)

	ctx := context.Background()
	repo := reacciones.NewRepository(s.env.Mongo.DB())
	p := reacciones.NewPrivacy(repo)
	exp, err := p.Export(ctx, amiga.user.ID)
	require.NoError(t, err)
	raw, _ := json.Marshal(exp)
	assert.Contains(t, string(raw), "Comentario de ella")
	assert.Contains(t, string(raw), mid)
	assert.NotContains(t, string(raw), "Respuesta mía", "nunca contenido ajeno")

	require.NoError(t, p.PurgeHistory(ctx, amiga.user.ID))
	likes, total := contadoresMomento(t, s, yo, mid)
	assert.Equal(t, []float64{1, 2}, []float64{likes, total}, "borrar historial no toca el contenido")
	require.NoError(t, p.PurgeAccount(ctx, amiga.user.ID))
	likes, total = contadoresMomento(t, s, yo, mid)
	assert.Equal(t, []float64{0, 1}, []float64{likes, total})
	var doc reacciones.Comentario
	require.NoError(t, s.env.Mongo.DB().Collection(reacciones.ComentariosCollection).FindOne(ctx, bson.D{{Key: "_id", Value: oidDe(t, str(t, c["id"]))}}).Decode(&doc))
	assert.Nil(t, doc.AutorID, "queda sin autor")
	assert.Equal(t, reacciones.EstadoEliminado, doc.Estado)
	assert.Empty(t, doc.Texto)
	items := list(t, pcall(t, s.app, http.MethodGet, momentosPath+"/"+mid+"/comentarios", nil, yo.token, http.StatusOK)["items"])
	require.Len(t, items, 1, "«Comentario eliminado» con la respuesta")

	// Al borrar la cuenta de la autora se va lo que recibieron sus momentos.
	require.NoError(t, p.PurgeAccount(ctx, yo.user.ID))
	n, err := s.env.Mongo.DB().Collection(reacciones.ComentariosCollection).CountDocuments(ctx, bson.D{{Key: "dueno_id", Value: yo.user.ID}})
	require.NoError(t, err)
	assert.Zero(t, n)
}
