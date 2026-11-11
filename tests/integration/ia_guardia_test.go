package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
)

// modeloIA sustituye al proveedor (interfaz externa, AGENTS.md §3); Redis y MongoDB son reales.
type modeloIA struct {
	llamadas int
	salida   string
}

func (m *modeloIA) Generar(context.Context, ia.Peticion) (ia.Respuesta, error) {
	m.llamadas++
	return ia.Respuesta{JSON: []byte(m.salida), TokensEntrada: 150, TokensSalida: 60, Duracion: time.Millisecond}, nil
}

func (m *modeloIA) Nombre() string { return "prueba/integracion" }

type idsIA struct {
	IDs []string `json:"ids"`
}

// guardiaIA arma la guardia con un mes propio para que las pruebas no compartan contador.
func guardiaIA(t *testing.T, mes time.Time, tope float64) (*ia.Guardia, *modeloIA, *ia.RegistroMongo) {
	t.Helper()
	ctx := context.Background()
	clave := ia.Mes(mes)
	require.NoError(t, redisClient.Del(ctx, "cuota:ia_gasto:"+clave, "rate:ia_usuario:sin_usuario").Err())
	db := mongoClient.DB()
	_, err := db.Collection(ia.ColeccionUsos).DeleteMany(ctx, bson.D{{Key: "mes", Value: clave}})
	require.NoError(t, err)

	m := &modeloIA{salida: `{"ids":["n1","n2"]}`}
	reg := ia.NewRegistroMongo(db)
	g, err := ia.NewGuardia(m, ia.NewContadorRedis(redisClient), reg, ia.NewCacheRedis(redisClient),
		ia.GuardiaConfig{TopeUSDMes: tope, Precios: ia.Precios{EntradaUSDMTok: 0.10, SalidaUSDMTok: 0.40},
			Limites: ia.Limites{PorHora: 50, PorMinuto: 15, PorDia: 500}, Limitador: ia.NewLimitadorRedis(redisClient),
			Ahora: func() time.Time { return mes }},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	return g, m, reg
}

func peticionIA(t *testing.T, frase string) ia.Peticion {
	return ia.Peticion{Instrucciones: "elige ids de la lista " + t.Name(), Contexto: "candidatos: n1,n2,n3", TextoUsuario: frase}
}

func TestIACache_MismaEntradaAcierta(t *testing.T) {
	g, m, _ := guardiaIA(t, time.Date(2031, 1, 10, 0, 0, 0, 0, time.UTC), 5)
	ctx := context.Background()

	var a, b idsIA
	_, err := g.GenerarValidado(ctx, peticionIA(t, "Tacos al pastor"), &a)
	require.NoError(t, err)
	_, err = g.GenerarValidado(ctx, peticionIA(t, "  tacos AL pastor "), &b)
	require.NoError(t, err)

	assert.Equal(t, 1, m.llamadas)
	assert.Equal(t, a, b)
}

func TestIACache_EntradaDistintaFalla(t *testing.T) {
	g, m, _ := guardiaIA(t, time.Date(2031, 2, 10, 0, 0, 0, 0, time.UTC), 5)
	ctx := context.Background()

	var out idsIA
	_, err := g.GenerarValidado(ctx, peticionIA(t, "tacos"), &out)
	require.NoError(t, err)
	otra := peticionIA(t, "tacos")
	otra.Contexto = "candidatos: n4,n5"
	_, err = g.GenerarValidado(ctx, otra, &out)
	require.NoError(t, err)

	assert.Equal(t, 2, m.llamadas, "otra lista de candidatos no acierta")
}

func TestIACache_ValorSinTextoDelUsuarioYConVida(t *testing.T) {
	g, _, _ := guardiaIA(t, time.Date(2031, 3, 10, 0, 0, 0, 0, time.UTC), 5)
	ctx := context.Background()
	pet := peticionIA(t, "algo tranquilo para leer")

	var out idsIA
	_, err := g.GenerarValidado(ctx, pet, &out)
	require.NoError(t, err)

	clave := "cache:ia:" + ia.Huella("prueba/integracion", pet)
	valor, err := redisClient.Get(ctx, clave).Result()
	require.NoError(t, err)
	assert.NotContains(t, strings.ToLower(valor), "tranquilo", "el valor guardado no contiene la frase")
	assert.NotContains(t, clave, "tranquilo", "la llave es la huella, no la frase")
	ttl, err := redisClient.TTL(ctx, clave).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, 59*time.Minute)
	assert.LessOrEqual(t, ttl, ia.VidaCache)
}

func TestIACache_SalidaQueRepiteLaFraseNoSeGuarda(t *testing.T) {
	g, m, _ := guardiaIA(t, time.Date(2031, 4, 10, 0, 0, 0, 0, time.UTC), 5)
	m.salida = `{"ids":["cerca del metro"]}`
	ctx := context.Background()
	pet := peticionIA(t, "cerca del metro")

	var out idsIA
	_, err := g.GenerarValidado(ctx, pet, &out)
	require.NoError(t, err)

	n, err := redisClient.Exists(ctx, "cache:ia:"+ia.Huella("prueba/integracion", pet)).Result()
	require.NoError(t, err)
	assert.Zero(t, n)
}

func TestIAGasto_AlLlegarAlTopeNoLlamaYReconstruye(t *testing.T) {
	mes := time.Date(2031, 5, 10, 0, 0, 0, 0, time.UTC)
	// Tope de 78 micro-USD: dos llamadas de 39 (150 × 0.10 + 60 × 0.40).
	g, m, _ := guardiaIA(t, mes, 0.000078)
	ctx := context.Background()

	for range 2 {
		_, err := g.Generar(ctx, peticionIA(t, "x"))
		require.NoError(t, err)
	}
	total, err := redisClient.Get(ctx, "cuota:ia_gasto:2031-05").Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(78), total)
	ttl, err := redisClient.TTL(ctx, "cuota:ia_gasto:2031-05").Result()
	require.NoError(t, err)
	assert.Positive(t, ttl, "la llave de gasto siempre lleva TTL")

	_, err = g.Generar(ctx, peticionIA(t, "x"))
	require.ErrorIs(t, err, ia.ErrTope)
	assert.Equal(t, 2, m.llamadas, "al llegar al tope no se llama al modelo")

	// Sin la llave en Redis, el gasto se reconstruye desde usos_ia y el tope se mantiene.
	require.NoError(t, redisClient.Del(ctx, "cuota:ia_gasto:2031-05").Err())
	_, err = g.Generar(ctx, peticionIA(t, "x"))
	require.ErrorIs(t, err, ia.ErrTope)
	assert.Equal(t, 2, m.llamadas)
	total, err = redisClient.Get(ctx, "cuota:ia_gasto:2031-05").Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(78), total)
}

func TestIAUsos_PrivacidadDesligaYExporta(t *testing.T) {
	mes := time.Date(2031, 6, 10, 0, 0, 0, 0, time.UTC)
	g, _, reg := guardiaIA(t, mes, 5)
	usuario, otro := bson.NewObjectID(), bson.NewObjectID()
	frase := "quiero cenar con mi pareja"

	_, err := g.Generar(ia.ConUsuario(context.Background(), usuario), peticionIA(t, frase))
	require.NoError(t, err)
	_, err = g.Generar(ia.ConUsuario(context.Background(), otro), peticionIA(t, frase))
	require.NoError(t, err)

	exportado, err := reg.Export(context.Background(), usuario)
	require.NoError(t, err)
	crudo, err := json.Marshal(exportado)
	require.NoError(t, err)
	assert.Contains(t, string(crudo), `"tokens_entrada":150`)
	assert.NotContains(t, string(crudo), "pareja", "usos_ia no guarda la frase")
	assert.NotContains(t, string(crudo), usuario.Hex())

	require.NoError(t, reg.PurgeAccount(context.Background(), usuario))

	db := mongoClient.DB().Collection(ia.ColeccionUsos)
	n, err := db.CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: usuario}})
	require.NoError(t, err)
	assert.Zero(t, n, "no queda nada ligado al usuario")
	n, err = db.CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: otro}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "los usos de otro usuario no cambian")
	gasto, err := reg.GastoDelMes(context.Background(), ia.Mes(mes))
	require.NoError(t, err)
	assert.Equal(t, int64(78), gasto, "el costo se conserva para el tope")
	exportado, err = reg.Export(context.Background(), usuario)
	require.NoError(t, err)
	assert.Empty(t, exportado)
}
