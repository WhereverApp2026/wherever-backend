package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/insignias"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
	"github.com/WhereverApp2026/wherever-backend/internal/resenas"
)

// Las colecciones que cuenta insignias son las de cada módulo (no las importa por el ciclo
// personas → insignias; ver internal/insignias/repository.go).
func TestInsignias_ColeccionesIgualesALosModulos(t *testing.T) {
	assert.Equal(t, planes.Collection, insignias.ColPlanes)
	assert.Equal(t, resenas.Collection, insignias.ColResenas)
	assert.Equal(t, resenas.CheckinsCollection, insignias.ColCheckins)
	assert.Equal(t, postales.Collection, insignias.ColPostales)
	assert.Equal(t, amistades.Collection, insignias.ColAmistades)
	assert.Equal(t, planes.StatusActive, "activo")
	assert.Equal(t, resenas.EstadoPublicada, "publicada")
	assert.Equal(t, postales.EstadoLista, "lista")
	assert.Equal(t, amistades.StatusAccepted, "aceptada")
}

// sembrar inserta lo mínimo que cuenta cada fuente.
func sembrar(t *testing.T, db *mongo.Database, user bson.ObjectID, planesN, resenasN, lugares, postalesN int, amigos []bson.ObjectID) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	for range planesN {
		_, err := db.Collection(planes.Collection).InsertOne(ctx, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "dueno_id", Value: user}, {Key: "estado", Value: planes.StatusActive}})
		require.NoError(t, err)
	}
	_, err := db.Collection(planes.Collection).InsertOne(ctx, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "dueno_id", Value: user}, {Key: "estado", Value: planes.StatusDeleted}})
	require.NoError(t, err, "un plan borrado no cuenta")
	for range resenasN {
		_, err := db.Collection(resenas.Collection).InsertOne(ctx, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "usuario_id", Value: user}, {Key: "estado", Value: resenas.EstadoPublicada}, {Key: "checkin_id", Value: bson.NewObjectID()}})
		require.NoError(t, err)
	}
	for i := range lugares {
		negocio := bson.NewObjectID()
		for _, dia := range []string{"2026-11-01", "2026-11-02"} { // dos visitas al mismo lugar cuentan como un lugar
			_, err := db.Collection(resenas.CheckinsCollection).InsertOne(ctx, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "usuario_id", Value: user},
				{Key: "negocio_id", Value: negocio}, {Key: "valido", Value: true}, {Key: "dia", Value: dia}, {Key: "creado_en", Value: now}})
			require.NoError(t, err, i)
		}
	}
	_, err = db.Collection(resenas.CheckinsCollection).InsertOne(ctx, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "usuario_id", Value: user},
		{Key: "negocio_id", Value: bson.NewObjectID()}, {Key: "valido", Value: false}, {Key: "dia", Value: "2026-11-03"}, {Key: "creado_en", Value: now}})
	require.NoError(t, err, "un check-in no válido no cuenta")
	for range postalesN {
		_, err := db.Collection(postales.Collection).InsertOne(ctx, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "usuario_id", Value: user}, {Key: "estado", Value: postales.EstadoLista}, {Key: "imagen_id", Value: bson.NewObjectID().Hex()}})
		require.NoError(t, err)
	}
	for _, a := range amigos {
		_, err := db.Collection(amistades.Collection).InsertOne(ctx, amistades.Friendship{ID: bson.NewObjectID(), From: a, To: user, Status: amistades.StatusAccepted, Pair: amistades.PairKey(a, user), CreatedAt: now, UpdatedAt: now})
		require.NoError(t, err)
	}
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

func mias(t *testing.T, raw string) map[string]insigniaVista {
	t.Helper()
	var out struct {
		Insignias []insigniaVista `json:"insignias"`
		Ganadas   int             `json:"ganadas"`
		Total     int             `json:"total"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &out), raw)
	require.Equal(t, len(insignias.Catalogo), out.Total)
	byID := map[string]insigniaVista{}
	ganadas := 0
	for _, it := range out.Insignias {
		byID[it.ID] = it
		if it.Ganada {
			ganadas++
		}
	}
	require.Equal(t, ganadas, out.Ganadas)
	return byID
}

// Ganadas y bloqueadas con su avance; la fecha se guarda la primera vez y se conserva aunque
// el conteo baje; otra persona ve solo las ganadas; ARCO exporta y borra.
func TestInsignias_MiasGanadasYBloqueadas(t *testing.T) {
	env, app := amistadesEnv(t)
	ana := seedAmigo(t, env, "insig_ana")
	beto := seedAmigo(t, env, "insig_beto")
	db := env.Mongo.DB()
	amigos := []bson.ObjectID{bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID(), beto.user.ID}
	sembrar(t, db, ana.user.ID, 2, 1, 5, 4, amigos)

	resp := sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo/insignias", nil, ana.token)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	got := mias(t, readAll(t, resp))
	for _, id := range []string{"primer_plan", "primera_resena", "explorador", "buena_compania"} {
		assert.True(t, got[id].Ganada, id)
		require.NotNil(t, got[id].GanadaEn, id)
		assert.Equal(t, got[id].Progreso.Meta, got[id].Progreso.Actual, id)
	}
	for id, actual := range map[string]int{"planificador": 2, "critico": 1, "trotamundos": 5, "fotografo": 4} {
		assert.False(t, got[id].Ganada, id)
		assert.Nil(t, got[id].GanadaEn, id)
		assert.Equal(t, actual, got[id].Progreso.Actual, id)
	}
	primera := *got["primer_plan"].GanadaEn

	// Se borran los planes: la insignia ganada se queda con su fecha.
	_, err := db.Collection(planes.Collection).DeleteMany(context.Background(), bson.D{{Key: "dueno_id", Value: ana.user.ID}})
	require.NoError(t, err)
	again := mias(t, readAll(t, sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo/insignias", nil, ana.token)))
	assert.True(t, again["primer_plan"].Ganada)
	assert.True(t, primera.Equal(*again["primer_plan"].GanadaEn), "la fecha no cambia")
	n, err := db.Collection(insignias.Collection).CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: ana.user.ID}})
	require.NoError(t, err)
	assert.Equal(t, int64(4), n, "una por insignia, sin duplicados")

	// Beto ve solo las ganadas de Ana, sin avance.
	r := sendJSON(t, app, http.MethodGet, "/api/v1/personas/"+ana.id()+"/insignias", nil, beto.token)
	code, raw := r.StatusCode, readAll(t, r)
	require.Equal(t, http.StatusOK, code, raw)
	var ajenas struct {
		Insignias []map[string]any `json:"insignias"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &ajenas))
	assert.Len(t, ajenas.Insignias, 4)
	assert.NotContains(t, raw, "progreso")
	assert.NotContains(t, raw, "fotografo", "las bloqueadas no se ven")

	// ARCO: la exportación las trae y el borrado de la cuenta las quita.
	exp, err := insignias.NewPrivacy(insignias.NewRepository(db)).Export(context.Background(), ana.user.ID)
	require.NoError(t, err)
	b, err := json.Marshal(exp)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"insignia":"primer_plan"`)
	require.NoError(t, insignias.NewPrivacy(insignias.NewRepository(db)).PurgeAccount(context.Background(), ana.user.ID))
	n, err = db.Collection(insignias.Collection).CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: ana.user.ID}})
	require.NoError(t, err)
	assert.Zero(t, n)
}

// Dos peticiones a la vez guardan una sola fecha por insignia.
func TestInsignias_ConcurrenciaSinDuplicados(t *testing.T) {
	env, app := amistadesEnv(t)
	ana := seedAmigo(t, env, "insig_conc")
	sembrar(t, env.Mongo.DB(), ana.user.ID, 1, 0, 0, 0, nil)
	done := make(chan int, 4)
	for range 4 {
		go func() { done <- sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo/insignias", nil, ana.token).StatusCode }()
	}
	for range 4 {
		assert.Equal(t, http.StatusOK, <-done)
	}
	n, err := env.Mongo.DB().Collection(insignias.Collection).CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: ana.user.ID}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}
