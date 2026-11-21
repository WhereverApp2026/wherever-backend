package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/insignias"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
	"github.com/WhereverApp2026/wherever-backend/internal/resenas"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Las colecciones y valores que lee insignias son los de cada módulo (no los importa por el
// ciclo personas → insignias; ver internal/insignias/repository.go).
func TestInsignias_ColeccionesIgualesALosModulos(t *testing.T) {
	assert.Equal(t, usuarios.Collection, insignias.ColUsuarios)
	assert.Equal(t, planes.Collection, insignias.ColPlanes)
	assert.Equal(t, resenas.Collection, insignias.ColResenas)
	assert.Equal(t, resenas.CheckinsCollection, insignias.ColCheckins)
	assert.Equal(t, postales.Collection, insignias.ColPostales)
	assert.Equal(t, amistades.Collection, insignias.ColAmistades)
	assert.Equal(t, grupos.Collection, insignias.ColGrupos)
	assert.Equal(t, negocios.Collection, insignias.ColNegocios)
	assert.Equal(t, usuarios.StatusActive, insignias.UsuarioActivo)
	assert.Equal(t, resenas.EstadoPublicada, insignias.ResenaPublicada)
	assert.Equal(t, postales.EstadoLista, insignias.PostalLista)
	assert.True(t, negocios.ValidCategory(insignias.CategoriaCafeteria))
}

// diaMX es el día (AAAA-MM-DD) de hace `semanas` semanas en México, como `checkins.dia`.
func diaMX(semanas int) string {
	loc, err := time.LoadLocation("America/Mexico_City")
	if err != nil {
		loc = time.FixedZone("CST", -6*60*60)
	}
	return time.Now().In(loc).AddDate(0, 0, -7*semanas).Format(time.DateOnly)
}

type actividad struct {
	cafeterias, otros int // lugares distintos con check-in válido
	repetidas         int // visitas extra a un lugar ya visitado, otro día
	resenas, postales int
	planes, grupos    int
}

// sembrarActividad inserta lo mínimo que lee insignias. Las visitas se reparten en esta semana
// y las dos anteriores (racha de 3). Devuelve los check-ins válidos.
func sembrarActividad(t *testing.T, db *mongo.Database, user bson.ObjectID, a actividad) []bson.ObjectID {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	var checkins []bson.ObjectID
	visita := func(negocio bson.ObjectID, dia string) {
		id := bson.NewObjectID()
		_, err := db.Collection(resenas.CheckinsCollection).InsertOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "usuario_id", Value: user},
			{Key: "negocio_id", Value: negocio}, {Key: "valido", Value: true}, {Key: "dia", Value: dia}, {Key: "creado_en", Value: now}})
		require.NoError(t, err)
		checkins = append(checkins, id)
	}
	var lugares []bson.ObjectID
	for i := range a.cafeterias + a.otros {
		cat := "restaurante"
		if i < a.cafeterias {
			cat = insignias.CategoriaCafeteria
		}
		id := bson.NewObjectID()
		_, err := db.Collection(negocios.Collection).InsertOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "slug", Value: "insignias-" + id.Hex()}, {Key: "qr_checkin", Value: "qr-" + id.Hex()}, {Key: "nombre", Value: "Lugar insignias"}, {Key: "categoria", Value: cat}})
		require.NoError(t, err)
		lugares = append(lugares, id)
		visita(id, diaMX(i%3))
	}
	for i := range a.repetidas {
		visita(lugares[0], diaMX(1+i%2)) // el mismo lugar, otro día ya dentro de la racha
	}
	_, err := db.Collection(resenas.CheckinsCollection).InsertOne(ctx, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "usuario_id", Value: user},
		{Key: "negocio_id", Value: bson.NewObjectID()}, {Key: "valido", Value: false}, {Key: "dia", Value: diaMX(0)}, {Key: "creado_en", Value: now}})
	require.NoError(t, err, "un check-in no válido no cuenta")
	for i := range a.resenas {
		_, err := db.Collection(resenas.Collection).InsertOne(ctx, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "usuario_id", Value: user}, {Key: "estado", Value: resenas.EstadoPublicada}, {Key: "checkin_id", Value: checkins[i]}})
		require.NoError(t, err)
	}
	_, err = db.Collection(resenas.Collection).InsertOne(ctx, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "usuario_id", Value: user}, {Key: "estado", Value: resenas.EstadoEnModeracion}, {Key: "checkin_id", Value: bson.NewObjectID()}})
	require.NoError(t, err, "una reseña en moderación no cuenta")
	for range a.postales {
		_, err := db.Collection(postales.Collection).InsertOne(ctx, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "usuario_id", Value: user}, {Key: "estado", Value: postales.EstadoLista}, {Key: "imagen_id", Value: bson.NewObjectID().Hex()}})
		require.NoError(t, err)
	}
	for range a.planes {
		_, err := db.Collection(planes.Collection).InsertOne(ctx, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "dueno_id", Value: user}, {Key: "estado", Value: planes.StatusActive}, {Key: "creado_en", Value: now}})
		require.NoError(t, err)
	}
	for range a.grupos {
		_, err := db.Collection(grupos.Collection).InsertOne(ctx, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "organizador_id", Value: user}, {Key: "plan_id", Value: bson.NewObjectID()}, {Key: "creado_en", Value: now}})
		require.NoError(t, err)
	}
	return checkins
}

type insigniaVista struct {
	ID       string     `json:"id"`
	Ganada   bool       `json:"ganada"`
	GanadaEn *time.Time `json:"ganada_en"`
	Progreso struct {
		Actual int `json:"actual"`
		Meta   int `json:"meta"`
	} `json:"progreso"`
}

type nivelVista struct {
	Numero    int    `json:"numero"`
	Nombre    string `json:"nombre"`
	Puntos    int    `json:"puntos"`
	Siguiente *struct {
		Numero int    `json:"numero"`
		Nombre string `json:"nombre"`
		Puntos int    `json:"puntos"`
	} `json:"siguiente"`
}

type miasVista struct {
	byID  map[string]insigniaVista
	ids   []string
	nivel nivelVista
	racha int
}

func mias(t *testing.T, raw string) miasVista {
	t.Helper()
	var out struct {
		Insignias    []insigniaVista `json:"insignias"`
		Ganadas      int             `json:"ganadas"`
		Total        int             `json:"total"`
		Nivel        nivelVista      `json:"nivel"`
		RachaSemanas int             `json:"racha_semanas"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &out), raw)
	require.Equal(t, len(insignias.Catalogo), out.Total)
	v := miasVista{byID: map[string]insigniaVista{}, nivel: out.Nivel, racha: out.RachaSemanas}
	ganadas := 0
	for _, it := range out.Insignias {
		v.byID[it.ID] = it
		v.ids = append(v.ids, it.ID)
		if it.Ganada {
			ganadas++
		}
	}
	require.Equal(t, ganadas, out.Ganadas)
	return v
}

func pedirMias(t *testing.T, app *fiber.App, token string) miasVista {
	t.Helper()
	resp := sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo/insignias", nil, token)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	return mias(t, readAll(t, resp))
}

func puntosDe(t *testing.T, db *mongo.Database, id bson.ObjectID) usuarios.User {
	t.Helper()
	var u usuarios.User
	require.NoError(t, db.Collection(usuarios.Collection).FindOne(context.Background(), bson.D{{Key: "_id", Value: id}}).Decode(&u))
	return u
}

func movimientos(t *testing.T, db *mongo.Database, id bson.ObjectID) map[string]int {
	t.Helper()
	cur, err := db.Collection(insignias.MovimientosColl).Find(context.Background(), bson.D{{Key: "usuario_id", Value: id}})
	require.NoError(t, err)
	var movs []insignias.Movimiento
	require.NoError(t, cur.All(context.Background(), &movs))
	out := map[string]int{}
	for _, m := range movs {
		out[m.Motivo]++
		assert.Equal(t, insignias.PuntosPor[m.Motivo], m.Puntos, m.Motivo)
	}
	return out
}

// Catálogo de la maqueta, puntos por cada motivo, nivel con el siguiente, racha y perfil al día.
func TestInsignias_CatalogoPuntosNivelYRacha(t *testing.T) {
	env, app := amistadesEnv(t)
	ana := seedAmigo(t, env, "insig_ana")
	db := env.Mongo.DB()
	base := puntosDe(t, db, ana.user.ID).Points
	// Una insignia de nv-78 guardada: ya no se muestra ni suma.
	require.NoError(t, insignias.NewRepository(db).Guardar(context.Background(), ana.user.ID, "primer_plan", time.Now().UTC()))
	checkins := sembrarActividad(t, db, ana.user.ID, actividad{cafeterias: 5, otros: 5, repetidas: 1, resenas: 1, postales: 3, planes: 2, grupos: 1})

	got := pedirMias(t, app, ana.token)
	ids := make([]string, 0, len(insignias.Catalogo))
	for _, d := range insignias.Catalogo {
		ids = append(ids, d.ID)
	}
	assert.Equal(t, ids, got.ids)
	assert.NotContains(t, got.ids, "primer_plan")
	for _, id := range []string{"catador_cafe", "resenista", "anfitrion_grupo", "viajero_pueblo", "racha_3"} {
		assert.True(t, got.byID[id].Ganada, id)
		require.NotNil(t, got.byID[id].GanadaEn, id)
	}
	for id, actual := range map[string]int{"cronista": 3, "sello_oro": 1, "querido": 0, "explorador_total": 10} {
		assert.False(t, got.byID[id].Ganada, id)
		assert.Equal(t, actual, got.byID[id].Progreso.Actual, id)
	}
	assert.Equal(t, 3, got.racha)

	// 11 check-ins × 20 + 1 reseña × 30 + 3 postales × 10 + 2 planes × 5 + 5 insignias × 50.
	esperados := base + 11*20 + 30 + 3*10 + 2*5 + 5*50
	assert.Equal(t, map[string]int{"registro": 1, "checkin": 11, "resena": 1, "postal": 3, "plan": 2, "insignia": 5}, movimientos(t, db, ana.user.ID))
	assert.Equal(t, nivelVista{Numero: 4, Nombre: insignias.NivelAnfitrion, Puntos: esperados}, nivelSinSiguiente(got.nivel))
	require.NotNil(t, got.nivel.Siguiente)
	assert.Equal(t, 5, got.nivel.Siguiente.Numero)
	assert.Equal(t, 800, got.nivel.Siguiente.Puntos)
	u := puntosDe(t, db, ana.user.ID)
	assert.Equal(t, esperados, u.Points)
	assert.Equal(t, insignias.NivelAnfitrion, u.Level)
	assert.Equal(t, 4, u.LevelNumber)
	assert.Equal(t, 3, u.StreakWeeks)

	// Repetir, borrar o recrear no suma ni resta.
	_, err := db.Collection(resenas.Collection).DeleteMany(context.Background(), bson.D{{Key: "usuario_id", Value: ana.user.ID}})
	require.NoError(t, err)
	_, err = db.Collection(resenas.Collection).InsertOne(context.Background(), bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "usuario_id", Value: ana.user.ID}, {Key: "estado", Value: resenas.EstadoPublicada}, {Key: "checkin_id", Value: checkins[0]}})
	require.NoError(t, err, "otra reseña de la misma visita")
	_, err = db.Collection(postales.Collection).DeleteMany(context.Background(), bson.D{{Key: "usuario_id", Value: ana.user.ID}})
	require.NoError(t, err)
	_, err = db.Collection(planes.Collection).DeleteMany(context.Background(), bson.D{{Key: "dueno_id", Value: ana.user.ID}})
	require.NoError(t, err)
	sembrarActividad(t, db, ana.user.ID, actividad{planes: 2})
	again := pedirMias(t, app, ana.token)
	assert.Equal(t, esperados, again.nivel.Puntos)
	assert.True(t, again.byID["resenista"].Ganada, "una insignia ganada se conserva")

	// GET /usuarios/yo trae lo conciliado.
	resp := sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, ana.token)
	raw := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	var perfil struct {
		Puntos      int    `json:"puntos"`
		Nivel       string `json:"nivel"`
		NivelNumero int    `json:"nivel_numero"`
		Racha       int    `json:"racha_semanas"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &perfil))
	assert.Equal(t, esperados, perfil.Puntos)
	assert.Equal(t, insignias.NivelAnfitrion, perfil.Nivel)
	assert.Equal(t, 4, perfil.NivelNumero)
	assert.Equal(t, 3, perfil.Racha)
}

func nivelSinSiguiente(n nivelVista) nivelVista {
	n.Siguiente = nil
	return n
}

// Los planes suman hasta 5 por semana y GET /usuarios/yo también concilia.
func TestInsignias_PlanesTopeSemanalYPerfil(t *testing.T) {
	env, app := amistadesEnv(t)
	ana := seedAmigo(t, env, "insig_planes")
	db := env.Mongo.DB()
	base := puntosDe(t, db, ana.user.ID).Points
	sembrarActividad(t, db, ana.user.ID, actividad{planes: 7})
	resp := sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, ana.token)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, 5, movimientos(t, db, ana.user.ID)["plan"])
	u := puntosDe(t, db, ana.user.ID)
	assert.Equal(t, base+5*5, u.Points)
	assert.Equal(t, 1, u.LevelNumber)
	assert.Equal(t, insignias.NivelExplorador, u.Level)
}

// Peticiones a la vez guardan un movimiento por acción y suman una sola vez.
func TestInsignias_ConcurrenciaSinDuplicados(t *testing.T) {
	env, app := amistadesEnv(t)
	ana := seedAmigo(t, env, "insig_conc")
	db := env.Mongo.DB()
	base := puntosDe(t, db, ana.user.ID).Points
	sembrarActividad(t, db, ana.user.ID, actividad{otros: 3, resenas: 1, planes: 1, grupos: 1})
	var wg sync.WaitGroup
	codes := make(chan int, 8)
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path := "/api/v1/usuarios/yo/insignias"
			if i%2 == 1 {
				path = "/api/v1/usuarios/yo"
			}
			codes <- sendJSON(t, app, http.MethodGet, path, nil, ana.token).StatusCode
		}()
	}
	wg.Wait()
	close(codes)
	for c := range codes {
		assert.Equal(t, http.StatusOK, c)
	}
	n, err := db.Collection(insignias.Collection).CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: ana.user.ID}})
	require.NoError(t, err)
	assert.Equal(t, int64(3), n, "resenista, anfitrion_grupo y racha_3, una vez")
	assert.Equal(t, map[string]int{"registro": 1, "checkin": 3, "resena": 1, "plan": 1, "insignia": 3}, movimientos(t, db, ana.user.ID))
	assert.Equal(t, base+3*20+30+5+3*50, puntosDe(t, db, ana.user.ID).Points)
}

// El trabajo nocturno concilia a quien no abrió la app; ARCO exporta y borra la bitácora.
func TestInsignias_TareaNocturnaYArco(t *testing.T) {
	env, _ := amistadesEnv(t)
	ana := seedAmigo(t, env, "insig_noche")
	db := env.Mongo.DB()
	base := puntosDe(t, db, ana.user.ID).Points
	sembrarActividad(t, db, ana.user.ID, actividad{otros: 2, postales: 1})
	n, err := insignias.NewTarea(env.Insignias).Run(context.Background())
	require.NoError(t, err)
	assert.Positive(t, n)
	assert.Equal(t, base+2*20+10, puntosDe(t, db, ana.user.ID).Points)

	priv := insignias.NewPrivacyPuntos(insignias.NewRepository(db))
	assert.Equal(t, insignias.MovimientosColl, priv.Name())
	exp, err := priv.Export(context.Background(), ana.user.ID)
	require.NoError(t, err)
	b, err := json.Marshal(exp)
	require.NoError(t, err)
	var filas []struct {
		Motivo string `json:"motivo"`
		RefID  string `json:"ref_id"`
	}
	require.NoError(t, json.Unmarshal(b, &filas))
	motivos := []string{}
	for _, f := range filas {
		assert.NotEmpty(t, f.RefID)
		motivos = append(motivos, f.Motivo)
	}
	slices.Sort(motivos)
	assert.Equal(t, []string{"checkin", "checkin", "postal", "registro"}, motivos)
	require.NoError(t, priv.PurgeHistory(context.Background(), ana.user.ID))
	assert.Len(t, movimientos(t, db, ana.user.ID), 3, "borrar el historial no toca los puntos")
	require.NoError(t, priv.PurgeAccount(context.Background(), ana.user.ID))
	assert.Empty(t, movimientos(t, db, ana.user.ID))
}

// Otra persona ve solo las ganadas, sin avance; ARCO de las insignias exporta y borra.
func TestInsignias_AjenasYArco(t *testing.T) {
	env, app := amistadesEnv(t)
	ana := seedAmigo(t, env, "insig_ajena")
	beto := seedAmigo(t, env, "insig_beto")
	db := env.Mongo.DB()
	sembrarActividad(t, db, ana.user.ID, actividad{resenas: 1, otros: 1, grupos: 1})
	antes := puntosDe(t, db, ana.user.ID).Points

	r := sendJSON(t, app, http.MethodGet, "/api/v1/personas/"+ana.id()+"/insignias", nil, beto.token)
	code, raw := r.StatusCode, readAll(t, r)
	require.Equal(t, http.StatusOK, code, raw)
	var ajenas struct {
		Insignias []map[string]any `json:"insignias"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &ajenas))
	assert.Len(t, ajenas.Insignias, 2)
	assert.NotContains(t, raw, "progreso")
	assert.NotContains(t, raw, "puntos")
	assert.NotContains(t, raw, "cronista", "las bloqueadas no se ven")
	assert.Equal(t, antes, puntosDe(t, db, ana.user.ID).Points, "verlas no mueve los puntos de Ana")
	assert.Empty(t, movimientos(t, db, ana.user.ID))

	exp, err := insignias.NewPrivacy(insignias.NewRepository(db)).Export(context.Background(), ana.user.ID)
	require.NoError(t, err)
	b, err := json.Marshal(exp)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"insignia":"resenista"`)
	require.NoError(t, insignias.NewPrivacy(insignias.NewRepository(db)).PurgeAccount(context.Background(), ana.user.ID))
	n, err := db.Collection(insignias.Collection).CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: ana.user.ID}})
	require.NoError(t, err)
	assert.Zero(t, n)
}
