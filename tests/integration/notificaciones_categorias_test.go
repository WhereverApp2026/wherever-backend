package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/cupones"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/resenas"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

const prefsPath = "/api/v1/usuarios/yo/preferencias"

// cuantosAvisos cuenta los avisos de un tipo guardados para la persona.
func cuantosAvisos(t *testing.T, env *authtest.Env, who amigo, kind string) int64 {
	t.Helper()
	n, err := env.Mongo.DB().Collection(notificaciones.Collection).CountDocuments(context.Background(),
		bson.D{{Key: "usuario_id", Value: who.user.ID}, {Key: "tipo", Value: kind}})
	require.NoError(t, err)
	return n
}

func setNotifPrefs(t *testing.T, app *fiber.App, who amigo, body map[string]any) map[string]any {
	t.Helper()
	out := notifCall(t, app, http.MethodPatch, prefsPath, body, who.token, http.StatusOK)
	return obj(t, out["notificaciones"])
}

func TestNotificaciones_FiltroPorCategoriaYConteos(t *testing.T) {
	env, app := amistadesEnv(t)
	me := seedAmigo(t, env, "categorias")
	svc := notificaciones.NewService(notificaciones.NewRepository(env.Mongo.DB()), nil, env.Log)
	for _, kind := range []string{notificaciones.KindLike, notificaciones.KindComment, notificaciones.KindReservaRecordatorio,
		notificaciones.KindVoteOpened, notificaciones.KindBusinessReviewed} {
		svc.Emit(context.Background(), me.user.ID, kind, map[string]string{"objeto_id": bson.NewObjectID().Hex()})
		time.Sleep(2 * time.Millisecond)
	}

	tab := func(cat string) []any {
		return list(t, notifCall(t, app, http.MethodGet, notifPath+"?categoria="+cat, nil, me.token, http.StatusOK)["items"])
	}
	social := tab("social")
	require.Len(t, social, 2)
	for _, it := range social {
		assert.Equal(t, "social", obj(t, it)["categoria"])
	}
	assert.Len(t, tab("reservas"), 1)
	assert.Len(t, tab("planes"), 1)
	all := kinds(t, app, me)
	assert.Len(t, all, 5, "sin categoría trae todo")
	notifCall(t, app, http.MethodGet, notifPath+"?categoria=otra", nil, me.token, http.StatusUnprocessableEntity)

	// Un cursor de una pestaña no sirve en otra.
	page := notifCall(t, app, http.MethodGet, notifPath+"?categoria=social&limite=1", nil, me.token, http.StatusOK)
	next := str(t, page["siguiente_cursor"])
	require.NotEmpty(t, next)
	assert.Len(t, list(t, notifCall(t, app, http.MethodGet, notifPath+"?categoria=social&limite=1&cursor="+next, nil, me.token, http.StatusOK)["items"]), 1)
	notifCall(t, app, http.MethodGet, notifPath+"?categoria=planes&cursor="+next, nil, me.token, http.StatusUnprocessableEntity)

	counts := notifCall(t, app, http.MethodGet, notifPath+"/no-leidas", nil, me.token, http.StatusOK)
	assert.Equal(t, float64(5), counts["total"])
	assert.Equal(t, map[string]any{"social": float64(2), "reservas": float64(1), "planes": float64(1)}, counts["por_categoria"])
}

func TestNotificaciones_PreferenciasApagadasNoGuardan(t *testing.T) {
	env, app := amistadesEnv(t)
	me := seedAmigo(t, env, "social_apagado")
	got := setNotifPrefs(t, app, me, map[string]any{"notificaciones": map[string]bool{"social": false}})
	assert.Equal(t, false, got["social"])
	assert.Equal(t, true, got["recordatorios_plan"], "ausente = encendido")

	objeto := bson.NewObjectID().Hex()
	env.Avisos.Emit(context.Background(), me.user.ID, notificaciones.KindLike, "momento:"+objeto, map[string]string{"momento_id": objeto})
	assert.Zero(t, cuantosAvisos(t, env, me, notificaciones.KindLike), "social apagado: el me gusta no se guarda")

	svc := notificaciones.NewService(notificaciones.NewRepository(env.Mongo.DB()), nil, env.Log).WithUsers(env.Users)
	svc.Emit(context.Background(), me.user.ID, notificaciones.KindFriendRequest, map[string]string{"usuario_id": bson.NewObjectID().Hex()})
	assert.Equal(t, int64(1), cuantosAvisos(t, env, me, notificaciones.KindFriendRequest), "las solicitudes no dependen de social")

	setNotifPrefs(t, app, me, map[string]any{"notificaciones": map[string]bool{"social": true}})
	// Otro objeto: el primero ya reservó su lapso de 10 min (agrupado de nv-83) aunque no se guardó.
	otro := bson.NewObjectID().Hex()
	env.Avisos.Emit(context.Background(), me.user.ID, notificaciones.KindLike, "momento:"+otro, map[string]string{"momento_id": otro})
	assert.Equal(t, int64(1), cuantosAvisos(t, env, me, notificaciones.KindLike))
}

func TestNotificaciones_ResenaPendienteUnaVez(t *testing.T) {
	env, app := amistadesEnv(t)
	me := seedAmigo(t, env, "resena_pendiente")
	setNotifPrefs(t, app, me, map[string]any{"notificaciones": map[string]bool{"resenas": true}})
	now := time.Now().UTC()
	checkins := env.Mongo.DB().Collection(resenas.CheckinsCollection)
	checkin := func(dia string, hace time.Duration, extra bson.E) bson.ObjectID {
		id := bson.NewObjectID()
		doc := bson.D{{Key: "_id", Value: id}, {Key: "usuario_id", Value: me.user.ID}, {Key: "negocio_id", Value: bson.NewObjectID()},
			{Key: "metodo", Value: "qr"}, {Key: "valido", Value: true}, {Key: "dia", Value: dia}, {Key: "creado_en", Value: now.Add(-hace)}}
		if extra.Key != "" {
			doc = append(doc, extra)
		}
		_, err := checkins.InsertOne(context.Background(), doc)
		require.NoError(t, err)
		return id
	}
	pendiente := checkin("2026-11-20", 3*time.Hour, bson.E{})
	checkin("2026-11-21", time.Hour, bson.E{})                                             // aún no pasan 2 h
	checkin("2026-11-22", 3*time.Hour, bson.E{Key: "resena_id", Value: bson.NewObjectID()}) // ya reseñado

	task := resenas.NewAvisoPendiente(resenas.NewRepository(env.Mongo.DB()), env.Avisos)
	for range 2 {
		_, err := task.Avisar(context.Background(), now)
		require.NoError(t, err)
	}
	assert.Equal(t, int64(1), cuantosAvisos(t, env, me, notificaciones.KindReviewPending), "uno por check-in, aunque corra dos veces")
	items := list(t, notifCall(t, app, http.MethodGet, notifPath, nil, me.token, http.StatusOK)["items"])
	require.Len(t, items, 1)
	datos := obj(t, obj(t, items[0])["datos"])
	assert.Equal(t, pendiente.Hex(), datos["checkin_id"])
	assert.ElementsMatch(t, []string{"checkin_id", "negocio_id"}, keysOf(datos), "solo ids")
	assert.Nil(t, obj(t, items[0])["categoria"], "reseña pendiente solo sale en «todo»")
}

func TestNotificaciones_CuponPorVencerUnaVezYConPreferencia(t *testing.T) {
	env, app := amistadesEnv(t)
	me := seedAmigo(t, env, "cupon_vence")
	off := seedAmigo(t, env, "cupon_vence_off")
	setNotifPrefs(t, app, me, map[string]any{"notificaciones": map[string]bool{"cupones": true}})
	setNotifPrefs(t, app, off, map[string]any{"notificaciones": map[string]bool{"cupones": false}})
	// Mediodía de hoy en México: dentro del horario de aviso.
	local := time.Now().In(negocios.MexicoCity())
	noon := time.Date(local.Year(), local.Month(), local.Day(), 12, 0, 0, 0, local.Location()).UTC()
	cartera := env.Mongo.DB().Collection(cupones.CarteraCollection)
	guardar := func(who amigo, estado string, vence time.Duration) {
		_, err := cartera.InsertOne(context.Background(), bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "usuario_id", Value: who.user.ID},
			{Key: "cupon_id", Value: bson.NewObjectID()}, {Key: "negocio_id", Value: bson.NewObjectID()}, {Key: "estado", Value: estado},
			{Key: "vence_en", Value: noon.Add(vence)}, {Key: "creado_en", Value: noon}})
		require.NoError(t, err)
	}
	guardar(me, cupones.CarteraGuardado, 48*time.Hour)
	guardar(me, cupones.CarteraGuardado, 5*24*time.Hour) // vence en 5 días: aún no
	guardar(me, cupones.CarteraCanjeado, 24*time.Hour)   // ya canjeado
	guardar(off, cupones.CarteraGuardado, 24*time.Hour)

	task := cupones.NewAvisoPorVencer(cupones.NewRepository(env.Mongo.DB()), env.Avisos)
	_, err := task.Avisar(context.Background(), noon.Add(-4*time.Hour)) // 08:00: fuera de horario
	require.NoError(t, err)
	assert.Zero(t, cuantosAvisos(t, env, me, notificaciones.KindCouponExpiring))
	for range 2 {
		_, err = task.Avisar(context.Background(), noon)
		require.NoError(t, err)
	}
	assert.Equal(t, int64(1), cuantosAvisos(t, env, me, notificaciones.KindCouponExpiring))
	assert.Zero(t, cuantosAvisos(t, env, off, notificaciones.KindCouponExpiring), "cupones apagado")
}

// quietNow es un horario silencioso que cubre la hora actual de México.
func quietNow() map[string]string {
	local := time.Now().In(negocios.MexicoCity())
	return map[string]string{"desde": local.Add(-time.Hour).Format("15:04"), "hasta": local.Add(time.Hour).Format("15:04")}
}

func TestNotificaciones_HoraDeSalirUnaVezConPreferenciaYSilencio(t *testing.T) {
	env, app := amistadesEnv(t)
	me := seedAmigo(t, env, "hora_salir")
	off := seedAmigo(t, env, "hora_salir_off")
	quiet := seedAmigo(t, env, "hora_salir_silencio")
	setNotifPrefs(t, app, off, map[string]any{"notificaciones": map[string]bool{"recordatorios_plan": false}})
	setNotifPrefs(t, app, quiet, map[string]any{"horario_silencioso": quietNow()})

	now := time.Now().UTC().Truncate(time.Second)
	local := now.In(negocios.MexicoCity())
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
	plans := env.Mongo.DB().Collection(planes.Collection)
	plan := func(who amigo, fecha time.Time) {
		a, b := bson.NewObjectID(), bson.NewObjectID()
		_, err := plans.InsertOne(context.Background(), bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "dueno_id", Value: who.user.ID},
			{Key: "estado", Value: planes.StatusActive}, {Key: "fecha", Value: fecha}, {Key: "nombre", Value: "salida"},
			{Key: "paradas", Value: bson.A{
				bson.D{{Key: "id", Value: a}, {Key: "negocio_id", Value: bson.NewObjectID()}, {Key: "llega", Value: now.Add(-time.Hour)}, {Key: "minutos", Value: 30}},
				bson.D{{Key: "id", Value: b}, {Key: "negocio_id", Value: bson.NewObjectID()}, {Key: "llega", Value: now.Add(20 * time.Minute)}, {Key: "minutos", Value: 30}},
			}},
			{Key: "traslados", Value: bson.A{bson.D{{Key: "desde", Value: a}, {Key: "hasta", Value: b}, {Key: "minutos", Value: 15}, {Key: "km", Value: 1.2}}}}})
		require.NoError(t, err)
	}
	plan(me, today)
	plan(me, today.AddDate(0, 0, 1)) // mañana: no se mira
	plan(off, today)
	plan(quiet, today)

	task := planes.NewAvisoSalida(planes.NewRepository(env.Mongo.DB()), env.Avisos)
	for range 2 {
		_, err := task.Avisar(context.Background(), now)
		require.NoError(t, err)
	}
	assert.Equal(t, int64(1), cuantosAvisos(t, env, me, notificaciones.KindTimeToLeave), "uno por tramo de hoy")
	assert.Zero(t, cuantosAvisos(t, env, off, notificaciones.KindTimeToLeave), "recordatorios del plan apagado")
	assert.Zero(t, cuantosAvisos(t, env, quiet, notificaciones.KindTimeToLeave), "en horario silencioso no se guarda")
	planesTab := list(t, notifCall(t, app, http.MethodGet, notifPath+"?categoria=planes", nil, me.token, http.StatusOK)["items"])
	require.Len(t, planesTab, 1)
	datos := obj(t, obj(t, planesTab[0])["datos"])
	assert.ElementsMatch(t, []string{"plan_id", "parada_id", "negocio_id"}, keysOf(datos), "solo ids")
}
