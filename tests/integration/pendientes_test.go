package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/sincronizacion"
)

const pendientesPath = "/api/v1/sync/pendientes"

func pendientes(t *testing.T, app *fiber.App, token string, actions []map[string]any, want int) sincronizacion.PendingResponse {
	t.Helper()
	resp := sendJSON(t, app, http.MethodPost, pendientesPath, map[string]any{"acciones": actions}, token)
	raw := readAll(t, resp)
	require.Equal(t, want, resp.StatusCode, raw)
	var out sincronizacion.PendingResponse
	if want == http.StatusOK {
		require.NoError(t, json.Unmarshal([]byte(raw), &out), raw)
	}
	return out
}

// planData es el cuerpo de un plan nuevo de una parada en shop, para dentro de una semana.
func planData(shop bson.ObjectID, name string) map[string]any {
	day := time.Now().AddDate(0, 0, 7).Format(planes.DayLayout)
	return map[string]any{
		"nombre": name, "fecha": day, "personas": 2, "presupuesto": 800, "movilidad": "a_pie",
		"paradas": []map[string]any{{"negocio_id": shop.Hex(), "llega": day + "T12:00:00-06:00", "minutos": 60, "precio": 100}},
	}
}

func results(out sincronizacion.PendingResponse) []string {
	r := make([]string, 0, len(out.Results))
	for _, x := range out.Results {
		r = append(r, x.Result)
	}
	return r
}

func TestPendientes_LoteNuevoRepetidoYConflicto(t *testing.T) {
	s := newGrupos(t)
	batch := []map[string]any{
		{"id_cliente": "accion-0001", "tipo": "plan.crear", "datos": planData(s.shop, "Sin red")},
		{"id_cliente": "accion-0002", "tipo": "plan.editar", "plan_id": s.plan.ID.Hex(), "version": 1, "datos": map[string]any{"nombre": "Editado sin red"}},
		{"id_cliente": "accion-0003", "tipo": "privacidad.editar", "datos": map[string]any{"ubicacion": true}},
	}
	first := pendientes(t, s.app, s.org.token, batch, http.StatusOK)
	assert.Equal(t, []string{"aplicada", "aplicada", "aplicada"}, results(first))
	require.NotNil(t, first.Results[0].Plan)
	created := first.Results[0].PlanID
	require.NotNil(t, first.Results[2].Settings)
	assert.True(t, first.Results[2].Settings.Privacy.Location)

	// El mismo lote otra vez no cambia nada.
	again := pendientes(t, s.app, s.org.token, batch, http.StatusOK)
	assert.Equal(t, []string{"duplicada", "duplicada", "duplicada"}, results(again))
	assert.Equal(t, "aplicada", again.Results[0].Original)
	assert.Equal(t, created, again.Results[0].PlanID)
	n, err := s.env.Mongo.DB().Collection(planes.Collection).CountDocuments(context.Background(), bson.D{{Key: "dueno_id", Value: s.org.user.ID}})
	require.NoError(t, err)
	assert.Equal(t, int64(2), n, "repetir el lote no crea otro plan")

	// Otra edición con la versión vieja: conflicto con la copia del servidor, sin sobrescribir.
	conflict := pendientes(t, s.app, s.org.token, []map[string]any{
		{"id_cliente": "accion-0004", "tipo": "plan.editar", "plan_id": s.plan.ID.Hex(), "version": 1, "datos": map[string]any{"nombre": "Pisaría"}},
	}, http.StatusOK)
	require.Equal(t, "conflicto", conflict.Results[0].Result)
	require.NotNil(t, conflict.Results[0].Plan)
	assert.Equal(t, "Editado sin red", conflict.Results[0].Plan.Nombre)
	assert.Equal(t, 2, conflict.Results[0].Plan.Version)
}

func TestPendientes_RechazadasYAjenas(t *testing.T) {
	s := newGrupos(t)
	other := seedAmigo(t, s.env, "pend_otro")
	out := pendientes(t, s.app, other.token, []map[string]any{
		// El plan de otra persona: 404 como rechazada; los datos inválidos: 422 como rechazada.
		{"id_cliente": "ajena-0001", "tipo": "plan.borrar", "plan_id": s.plan.ID.Hex(), "version": 1},
		{"id_cliente": "ajena-0002", "tipo": "plan.crear", "datos": map[string]any{"nombre": ""}},
		{"id_cliente": "ajena-0003", "tipo": "plan.crear", "datos": planData(s.shop, "Mío")},
	}, http.StatusOK)
	assert.Equal(t, []string{"rechazada", "rechazada", "aplicada"}, results(out), "un lote parcial sigue con las demás")
	require.NotNil(t, out.Results[0].Error)
	assert.Equal(t, "no_encontrado", out.Results[0].Error.Code)
	assert.Equal(t, http.StatusOK, planCall(t, s.app, http.MethodGet, planPath(s.plan, ""), nil, s.org.token, "").status, "el plan ajeno sigue ahí")

	// 101 acciones = 422; sin tipo o sin plan_id cuando hace falta = 422.
	big := make([]map[string]any, 0, 101)
	for i := range 101 {
		big = append(big, map[string]any{"id_cliente": fmt.Sprintf("grande-%04d", i), "tipo": "plan.borrar", "plan_id": s.plan.ID.Hex(), "version": 1})
	}
	pendientes(t, s.app, other.token, big, http.StatusUnprocessableEntity)
	pendientes(t, s.app, other.token, []map[string]any{{"id_cliente": "falta-0001", "tipo": "plan.editar"}}, http.StatusUnprocessableEntity)
}

// Dos lotes del mismo usuario a la vez: cada cambio recibe un número distinto y sin huecos.
func TestPendientes_Concurrencia_SinHuecosNiRepetidos(t *testing.T) {
	s := newGrupos(t)
	var wg sync.WaitGroup
	for b := range 2 {
		wg.Add(1)
		go func(b int) {
			defer wg.Done()
			batch := []map[string]any{}
			for i := range 5 {
				batch = append(batch, map[string]any{"id_cliente": fmt.Sprintf("lote%d-%04d", b, i), "tipo": "plan.crear", "datos": planData(s.shop, "Concurrente")})
			}
			resp := sendJSON(t, s.app, http.MethodPost, pendientesPath, map[string]any{"acciones": batch}, s.org.token)
			_ = resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		}(b)
	}
	wg.Wait()
	cur, err := s.env.Mongo.DB().Collection(planes.Collection).Find(context.Background(), bson.D{{Key: "dueno_id", Value: s.org.user.ID}})
	require.NoError(t, err)
	var rows []planes.Plan
	require.NoError(t, cur.All(context.Background(), &rows))
	require.Len(t, rows, 11)
	seqs := make([]int, 0, len(rows))
	for _, p := range rows {
		seqs = append(seqs, int(p.SyncSeq))
	}
	sort.Ints(seqs)
	for i := 1; i < len(seqs); i++ {
		assert.Equal(t, seqs[i-1]+1, seqs[i], "números seguidos, sin repetir: %v", seqs)
	}
}
