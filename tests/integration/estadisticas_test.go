package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/cupones"
	"github.com/WhereverApp2026/wherever-backend/internal/estadisticas"
	"github.com/WhereverApp2026/wherever-backend/internal/favoritos"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/resenas"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/estadisticastest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

func zonaMX(t *testing.T) *time.Location {
	t.Helper()
	z, err := time.LoadLocation(estadisticas.Zona)
	require.NoError(t, err)
	return z
}

func insertar(t *testing.T, env *authtest.Env, col string, doc bson.D) {
	t.Helper()
	_, err := env.Mongo.DB().Collection(col).InsertOne(context.Background(), doc)
	require.NoError(t, err)
}

// actividadDe siembra, con fecha `cuando`, una reserva (más una apartada que no cuenta), un
// check-in válido (más uno inválido), un canje, un favorito y una reseña publicada de 4.
func actividadDe(t *testing.T, env *authtest.Env, b negocios.Business, cliente bson.ObjectID, cuando time.Time) {
	t.Helper()
	db := env.Mongo.DB()
	for _, estado := range []string{reservas.EstadoConfirmada, reservas.EstadoBloqueada} {
		r := reservas.Reserva{ID: bson.NewObjectID(), Codigo: bson.NewObjectID().Hex()[16:], UsuarioID: cliente, NegocioID: b.ID, FechaHora: cuando.Add(48 * time.Hour),
			Personas: 2, Estado: estado, Historial: []reservas.Cambio{}, Version: 1, CreadoEn: cuando, ActualizadoEn: cuando}
		require.NoError(t, reservas.NewRepository(db).Insert(context.Background(), &r))
	}
	for _, valido := range []bool{true, false} {
		insertar(t, env, resenas.CheckinsCollection, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "usuario_id", Value: cliente}, {Key: "negocio_id", Value: b.ID},
			{Key: "metodo", Value: "qr"}, {Key: "valido", Value: valido}, {Key: "dia", Value: estadisticastest.Dia(cuando)}, {Key: "creado_en", Value: cuando}})
	}
	insertar(t, env, cupones.CarteraCollection, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "usuario_id", Value: cliente}, {Key: "cupon_id", Value: bson.NewObjectID()},
		{Key: "negocio_id", Value: b.ID}, {Key: "estado", Value: "canjeado"}, {Key: "canjeado_en", Value: cuando}})
	insertar(t, env, favoritos.Collection, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "usuario_id", Value: cliente}, {Key: "tipo", Value: favoritos.TypeBusiness},
		{Key: "objetivo_id", Value: b.ID}, {Key: "creado_en", Value: cuando}})
	insertar(t, env, resenas.Collection, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "negocio_id", Value: b.ID}, {Key: "usuario_id", Value: cliente},
		{Key: "checkin_id", Value: bson.NewObjectID()}, {Key: "calificacion", Value: 4}, {Key: "estado", Value: resenas.EstadoPublicada},
		{Key: "aspectos", Value: bson.D{{Key: "sabor", Value: 5}, {Key: "atencion", Value: 4}, {Key: "ambiente", Value: 4}, {Key: "espera", Value: 3}, {Key: "precio_calidad", Value: 4}}},
		{Key: "creado_en", Value: cuando}, {Key: "actualizado_en", Value: cuando}})
}

func TestEstadisticas_TrabajoCuentaFuentesSinDuplicar(t *testing.T) {
	env := authEnv(t)
	ctx := context.Background()
	b, _ := dueno(t, env, "duena-estadisticas@wherever.test", reservastest.Config{})
	u, _ := cliente(t, env, "cliente-estadisticas@wherever.test")
	now := time.Now()
	actividadDe(t, env, b, u.ID, now)

	for range 2 {
		_, err := env.Estadisticas.Recalcular(ctx)
		require.NoError(t, err)
	}
	d := estadisticastest.Leer(t, env.Mongo.DB(), b.ID, estadisticastest.Dia(now))
	assert.Equal(t, 1, d.Reservas, "la apartada no cuenta")
	assert.Equal(t, 1, d.Llegadas, "el check-in inválido no cuenta")
	assert.Equal(t, 1, d.PorHora[now.In(zonaMX(t)).Hour()])
	assert.Equal(t, 1, d.CuponesCanjeados)
	assert.Equal(t, 1, d.FavoritosNuevos)
	assert.Equal(t, 1, d.ResenasDia)
	require.NotNil(t, d.CalificacionDia)
	assert.InDelta(t, 4.0, *d.CalificacionDia, 0.001)
	assert.InDelta(t, 5.0, d.AspectosSuma.Sabor, 0.001)
	assert.Equal(t, b.Category, d.Categoria)

	// Dos días después la persona borra su cuenta: el día cerrado no cambia.
	despues := estadisticas.NewService(estadisticas.NewRepository(env.Mongo.DB()), negocios.NewRepository(env.Mongo.DB()), env.Log).
		WithReloj(func() time.Time { return now.Add(48 * time.Hour) })
	_, err := env.Mongo.DB().Collection(reservas.Collection).DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: u.ID}})
	require.NoError(t, err)
	_, err = despues.Recalcular(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, estadisticastest.Leer(t, env.Mongo.DB(), b.ID, estadisticastest.Dia(now)).Reservas)

	// La baja del negocio borra sus días.
	require.NoError(t, estadisticas.NewObservador(env.Estadisticas).DadoDeBaja(ctx, b.ID))
	n, err := env.Mongo.DB().Collection(estadisticas.Collection).CountDocuments(ctx, bson.D{{Key: "negocio_id", Value: b.ID}})
	require.NoError(t, err)
	assert.Zero(t, n)
}

func TestEstadisticas_RellenoDe90Dias(t *testing.T) {
	env := authEnv(t)
	ctx := context.Background()
	b, _ := dueno(t, env, "duena-relleno@wherever.test", reservastest.Config{})
	u, _ := cliente(t, env, "cliente-relleno@wherever.test")
	hace10 := time.Now().AddDate(0, 0, -10)
	actividadDe(t, env, b, u.ID, hace10)

	n, err := env.Estadisticas.Rellenar(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, 1, estadisticastest.Leer(t, env.Mongo.DB(), b.ID, estadisticastest.Dia(hace10)).Llegadas)
	n, err = env.Estadisticas.Rellenar(ctx)
	require.NoError(t, err)
	assert.Zero(t, n, "ya rellenado")
}

func TestEstadisticas_VistaCuentaUnaVezPorVentana(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	b, _ := dueno(t, env, "duena-vistas74@wherever.test", reservastest.Config{})
	_, token := cliente(t, env, "cliente-vistas74@wherever.test")
	for range 2 {
		requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/lugares/"+b.ID.Hex()+"/vistas", map[string]any{"origen": "mapa"}, token), http.StatusNoContent)
	}
	d := estadisticastest.Leer(t, env.Mongo.DB(), b.ID, estadisticastest.Dia(time.Now()))
	assert.Equal(t, 1, d.VistasFicha, "la segunda cae en la ventana de 10 min")
	assert.Equal(t, 1, d.Origen.Mapa)
}

func rutaEstadisticas(b negocios.Business) string {
	return "/api/v1/negocios/" + b.ID.Hex() + "/estadisticas"
}

func TestEstadisticas_SemanaAnteriorYSinPremium(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	b, token := dueno(t, env, "duena-semanas@wherever.test", reservastest.Config{})
	hasta := time.Now().AddDate(0, 0, -1)
	for i := range 28 {
		d := estadisticas.Diaria{VistasFicha: 2, Reservas: 1}
		if i >= 14 {
			d = estadisticas.Diaria{VistasFicha: 1, Reservas: 1}
		}
		estadisticastest.Sembrar(t, env.Mongo.DB(), b.ID, estadisticastest.Dia(hasta.AddDate(0, 0, -i)), d)
	}
	ruta := rutaEstadisticas(b) + "?agrupar=semana&desde=" + estadisticastest.Dia(hasta.AddDate(0, 0, -13)) + "&hasta=" + estadisticastest.Dia(hasta)
	resp := sendJSON(t, app, http.MethodGet, ruta, nil, token)
	requireStatus(t, resp, http.StatusOK)
	body := decode(t, resp)
	totales, anterior, cambio := obj(t, body["totales"]), obj(t, body["anterior"]), obj(t, body["cambio"])
	assert.InDelta(t, 28, totales["vistas_ficha"], 0.001)
	assert.InDelta(t, 14, anterior["vistas_ficha"], 0.001)
	assert.InDelta(t, 100, cambio["vistas_ficha"], 0.001)
	assert.InDelta(t, 0, cambio["reservas"], 0.001)
	assert.Equal(t, "Tus visitas subieron 100 % frente a las 2 semanas anteriores.", str(t, body["frase"]))
	assert.LessOrEqual(t, len(list(t, body["serie"])), 3, "14 días caben en 2 o 3 semanas")
	assert.Nil(t, body["avanzadas"], "sin premium")
	assert.True(t, boolean(t, body["premium_requerido"]))

	requireStatus(t, sendJSON(t, app, http.MethodGet, rutaEstadisticas(b)+"?desde=2026-01-01&hasta=2025-12-01", nil, token), http.StatusUnprocessableEntity)
	requireStatus(t, sendJSON(t, app, http.MethodGet, rutaEstadisticas(b)+"?agrupar=mes", nil, token), http.StatusUnprocessableEntity)
}

func TestEstadisticas_CategoriaSoloConCincoNegocios(t *testing.T) {
	env := authEnv(t)
	ctx := context.Background()
	app := env.App()
	b, token := dueno(t, env, "duena-categoria@wherever.test", reservastest.Config{})
	_, err := env.Mongo.DB().Collection(negocios.Collection).UpdateOne(ctx, bson.D{{Key: "_id", Value: b.ID}}, bson.D{{Key: "$set", Value: bson.D{{Key: "plan", Value: negocios.PlanPremium}}}})
	require.NoError(t, err)
	cal := 4.0
	conResena := estadisticas.Diaria{Categoria: b.Category, ResenasDia: 1, CalificacionDia: &cal, AspectosSuma: estadisticas.Aspectos{Sabor: 4, Atencion: 4, Ambiente: 4, Espera: 4, PrecioCalidad: 4}}
	estadisticastest.Sembrar(t, env.Mongo.DB(), b.ID, estadisticastest.Ayer(), conResena)
	for range 3 {
		estadisticastest.Sembrar(t, env.Mongo.DB(), bson.NewObjectID(), estadisticastest.Ayer(), conResena)
	}

	aspecto := func() map[string]any {
		_, err := env.Estadisticas.Nocturno(ctx)
		require.NoError(t, err)
		resp := sendJSON(t, app, http.MethodGet, rutaEstadisticas(b), nil, token)
		requireStatus(t, resp, http.StatusOK)
		body := decode(t, resp)
		assert.False(t, boolean(t, body["premium_requerido"]))
		return obj(t, list(t, obj(t, body["avanzadas"])["aspectos"])[0])
	}
	sabor := aspecto()
	assert.InDelta(t, 4.0, sabor["tuyo"], 0.001)
	assert.Nil(t, sabor["categoria"], "4 negocios: no se compara")

	estadisticastest.Sembrar(t, env.Mongo.DB(), bson.NewObjectID(), estadisticastest.Ayer(), conResena)
	assert.InDelta(t, 4.0, aspecto()["categoria"], 0.001, "con 5 negocios sí")
}
