package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

// planConParadas guarda un plan de owner con una parada por negocio, mañana a la hora dada
// (México). Va directo al repositorio: los negocios de reservastest no traen `horario_min`.
func planConParadas(t *testing.T, env *authtest.Env, owner bson.ObjectID, paradas map[bson.ObjectID]time.Time) planes.Plan {
	t.Helper()
	p := planes.Plan{
		OwnerID: owner, Nombre: "Todo el día", Fecha: reservastest.Manana(0), Personas: 2, Movilidad: planes.MobilityWalk,
		Origen: planes.OriginManual, Visibilidad: planes.VisibilityPrivate, Paradas: []planes.Parada{}, Traslados: []planes.Traslado{},
	}
	for negocio, llega := range paradas {
		p.Paradas = append(p.Paradas, planes.Parada{ID: bson.NewObjectID(), NegocioID: negocio, Llega: llega, Minutos: 60, Precio: 100})
	}
	require.NoError(t, planes.NewRepository(env.Mongo.DB()).Create(context.Background(), &p))
	return p
}

func reservarTodo(t *testing.T, app *fiber.App, p planes.Plan, token, key string, body any) (int, reservas.PlanResponse, string) {
	t.Helper()
	headers := map[string]string{}
	if key != "" {
		headers["Idempotency-Key"] = key
	}
	resp := sendWithHeaders(t, app, http.MethodPost, planPath(p, "/reservas"), body, token, headers)
	raw := readAll(t, resp)
	var out reservas.PlanResponse
	if resp.StatusCode == http.StatusOK {
		require.NoError(t, json.Unmarshal([]byte(raw), &out), raw)
	}
	return resp.StatusCode, out, raw
}

func resultadoDe(t *testing.T, out reservas.PlanResponse, negocio bson.ObjectID) reservas.PlanResultado {
	t.Helper()
	for _, r := range out.Resultados {
		if r.NegocioID == negocio.Hex() {
			return r
		}
	}
	t.Fatalf("sin resultado para %s", negocio.Hex())
	return reservas.PlanResultado{}
}

func reservasDelPlan(t *testing.T, env *authtest.Env, p planes.Plan) int64 {
	t.Helper()
	return contar(t, env, bson.D{{Key: "plan_id", Value: p.ID}, {Key: "estado", Value: bson.D{{Key: "$ne", Value: reservas.EstadoCancelada}}}})
}

func TestReservarPlan_ExitoParcialEIdempotencia(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	libre, _ := dueno(t, env, "nv89-libre@wherever.test", reservastest.Config{Capacidad: 10})
	lleno, _ := dueno(t, env, "nv89-lleno@wherever.test", reservastest.Config{Capacidad: 2})
	sinReservas, _ := dueno(t, env, "nv89-sin@wherever.test", reservastest.Config{SinReservas: true})
	owner, token := cliente(t, env, "nv89-owner@wherever.test")
	_, tokenOtro := cliente(t, env, "nv89-otro@wherever.test")
	requireStatus(t, reservar(t, app, tokenOtro, llave(8901), cuerpoReserva(lleno, reservastest.Manana(12), 2)), http.StatusCreated)
	p := planConParadas(t, env, owner.ID, map[bson.ObjectID]time.Time{
		libre.ID:       reservastest.Manana(10).Add(30 * time.Minute),
		lleno.ID:       reservastest.Manana(12),
		sinReservas.ID: reservastest.Manana(14),
	})

	status, out, raw := reservarTodo(t, app, p, token, llave(8902), nil)
	require.Equal(t, http.StatusOK, status, raw)
	require.Len(t, out.Resultados, 3)
	ok := resultadoDe(t, out, libre.ID)
	assert.Equal(t, reservas.ParadaReservada, ok.Estado)
	require.NotNil(t, ok.Reserva)
	assert.Equal(t, reservastest.Manana(10), ok.Reserva.FechaHora, "el turno que contiene la llegada")
	assert.Equal(t, 2, ok.Reserva.Personas, "las personas del plan")
	assert.Equal(t, p.ID.Hex(), ok.Reserva.PlanID)
	assert.Equal(t, reservas.ParadaSinCupo, resultadoDe(t, out, lleno.ID).Estado)
	assert.Equal(t, reservas.ParadaNoAcepta, resultadoDe(t, out, sinReservas.ID).Estado)
	assert.Zero(t, out.AnticipoTotal)
	assert.Zero(t, out.RequierenPago)

	// Reintento con la misma llave: la misma reserva, nada nuevo.
	status, again, raw := reservarTodo(t, app, p, token, llave(8902), nil)
	require.Equal(t, http.StatusOK, status, raw)
	assert.Equal(t, ok.Reserva.ID, resultadoDe(t, again, libre.ID).Reserva.ID)
	assert.Equal(t, int64(1), reservasDelPlan(t, env, p))

	// Otra llave: la parada ya está reservada y no se repite.
	status, other, raw := reservarTodo(t, app, p, token, llave(8903), map[string]any{"personas": 3})
	require.Equal(t, http.StatusOK, status, raw)
	assert.Equal(t, reservas.ParadaYaReservada, resultadoDe(t, other, libre.ID).Estado)
	assert.Nil(t, resultadoDe(t, other, libre.ID).Reserva)
	assert.Equal(t, int64(1), reservasDelPlan(t, env, p))
}

func TestReservarPlan_Rechazos(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, _ := dueno(t, env, "nv89-rech@wherever.test", reservastest.Config{})
	owner, token := cliente(t, env, "nv89-rech-owner@wherever.test")
	p := planConParadas(t, env, owner.ID, map[bson.ObjectID]time.Time{b.ID: reservastest.Manana(9)})

	status, _, _ := reservarTodo(t, app, p, token, "", nil)
	assert.Equal(t, http.StatusBadRequest, status, "sin Idempotency-Key")
	status, _, _ = reservarTodo(t, app, p, token, llave(8910), map[string]any{"personas": 21})
	assert.Equal(t, http.StatusUnprocessableEntity, status)

	pasado := planConParadas(t, env, owner.ID, map[bson.ObjectID]time.Time{b.ID: time.Now().Add(-2 * time.Hour)})
	status, _, raw := reservarTodo(t, app, pasado, token, llave(8911), nil)
	assert.Equal(t, http.StatusUnprocessableEntity, status, raw)

	_, ajeno := cliente(t, env, "nv89-rech-ajeno@wherever.test")
	status, _, _ = reservarTodo(t, app, p, ajeno, llave(8912), nil)
	assert.Equal(t, http.StatusNotFound, status, "plan ajeno")
	assert.Zero(t, reservasDelPlan(t, env, p))
}

// La política `reservas` cuenta una reserva por parada que se intenta; si no alcanza, 429 sin
// reservar nada.
func TestReservarPlan_LimiteCuentaPorParada(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, _ := dueno(t, env, "nv89-lim@wherever.test", reservastest.Config{Capacidad: 50})
	owner, token := cliente(t, env, "nv89-lim-owner@wherever.test")
	for i := range 9 {
		requireStatus(t, reservar(t, app, token, llave(8920+i), cuerpoReserva(b, reservastest.Manana(1+i), 1)), http.StatusCreated)
	}
	otro, _ := dueno(t, env, "nv89-lim2@wherever.test", reservastest.Config{})
	p := planConParadas(t, env, owner.ID, map[bson.ObjectID]time.Time{b.ID: reservastest.Manana(20), otro.ID: reservastest.Manana(21)})
	status, _, raw := reservarTodo(t, app, p, token, llave(8930), nil)
	assert.Equal(t, http.StatusTooManyRequests, status, raw)
	assert.Zero(t, reservasDelPlan(t, env, p))
}

// Dos «reservar todo» con llaves distintas a la vez: la parada queda reservada una sola vez.
func TestReservarPlan_Concurrencia_UnaReservaPorParada(t *testing.T) {
	env := authEnv(t)
	b, _ := dueno(t, env, "nv89-conc@wherever.test", reservastest.Config{Capacidad: 20})
	owner, _ := cliente(t, env, "nv89-conc-owner@wherever.test")
	p := planConParadas(t, env, owner.ID, map[bson.ObjectID]time.Time{b.ID: reservastest.Manana(18)})
	cl := authtest.ClaimsDe(owner.ID.Hex(), middleware.RoleConsumer)

	var wg sync.WaitGroup
	var mu sync.Mutex
	estados := map[string]int{}
	for i := range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := env.Reservas.ReservarPlan(context.Background(), cl, p.ID.Hex(), llave(8940+i), reservas.PlanRequest{}, nil)
			mu.Lock()
			defer mu.Unlock()
			var ae *apperr.Error
			if errors.As(err, &ae) && ae.Kind == apperr.KindConflict {
				estados["conflicto"]++
				return
			}
			if !assert.NoError(t, err) || !assert.Len(t, out.Resultados, 1) {
				return
			}
			estados[out.Resultados[0].Estado]++
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, estados[reservas.ParadaReservada], "%v", estados)
	assert.Equal(t, int64(1), reservasDelPlan(t, env, p))
}

// compartirPlan crea el enlace con ese permiso y devuelve el código.
func compartirPlan(t *testing.T, app *fiber.App, p planes.Plan, token string, body any) grupos.ShareResponse {
	t.Helper()
	resp := sendJSON(t, app, http.MethodPost, planPath(p, "/compartir"), body, token)
	raw := readAll(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode, raw)
	var out grupos.ShareResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	return out
}

func permisoDe(v grupos.View, who string) string {
	for _, m := range v.Members {
		if m.UserID == who {
			return m.Permission
		}
	}
	return ""
}

func TestPermisosPlan_EnlaceYOrganizador(t *testing.T) {
	s := newGrupos(t)
	editor, lector := seedAmigo(t, s.env, "nv89editor"), seedAmigo(t, s.env, "nv89lector")

	// Sin cuerpo el enlace es de `ver`; con `editar`, quien entra edita.
	verEnlace := compartirPlan(t, s.app, s.plan, s.org.token, nil)
	assert.Equal(t, planes.PermissionView, verEnlace.Permission)
	v := grupoCall(t, s.app, http.MethodPost, gruposPath+"/unirse", map[string]string{"codigo": verEnlace.Code}, lector.token, http.StatusOK)
	assert.Equal(t, planes.PermissionView, permisoDe(v, lector.id()))
	assert.Equal(t, planes.PermissionEdit, permisoDe(v, s.org.id()), "el organizador siempre edita")
	editEnlace := compartirPlan(t, s.app, s.plan, s.org.token, map[string]string{"permiso": "editar"})
	assert.Equal(t, planes.PermissionEdit, editEnlace.Permission)
	v = grupoCall(t, s.app, http.MethodPost, gruposPath+"/unirse", map[string]string{"codigo": editEnlace.Code}, editor.token, http.StatusOK)
	assert.Equal(t, planes.PermissionEdit, permisoDe(v, editor.id()))

	got := planCall(t, s.app, http.MethodGet, planPath(s.plan, ""), nil, editor.token, "")
	require.Equal(t, http.StatusOK, got.status, got.body)
	assert.Equal(t, planes.PermissionEdit, got.plan(t).MiPermiso)
	assert.NotContains(t, planCall(t, s.app, http.MethodGet, planPath(s.plan, ""), nil, s.org.token, "").body, "mi_permiso", "el dueño no lo lleva")

	// El integrante con `editar` cambia el plan con If-Match, igual que el dueño.
	edited := planCall(t, s.app, http.MethodPatch, planPath(s.plan, ""), map[string]any{"nombre": "Lo cambió el grupo"}, editor.token, `"1"`)
	require.Equal(t, http.StatusOK, edited.status, edited.body)
	assert.Equal(t, "Lo cambió el grupo", edited.plan(t).Nombre)
	assert.Equal(t, 2, edited.plan(t).Version)
	stale := planCall(t, s.app, http.MethodPatch, planPath(s.plan, ""), map[string]any{"nombre": "Viejo"}, editor.token, `"1"`)
	assert.Equal(t, http.StatusConflict, stale.status)
	stop := s.plan.Paradas[0].ID.Hex()
	moved := planCall(t, s.app, http.MethodPatch, planPath(s.plan, "/paradas/"+stop), map[string]any{"minutos": 90}, editor.token, `"2"`)
	require.Equal(t, http.StatusOK, moved.status, moved.body)

	// Lo que sigue siendo del dueño.
	assert.Equal(t, http.StatusForbidden, planCall(t, s.app, http.MethodPatch, planPath(s.plan, ""), map[string]any{"visibilidad": "publico"}, editor.token, `"3"`).status)
	assert.Equal(t, http.StatusNotFound, planCall(t, s.app, http.MethodDelete, planPath(s.plan, ""), nil, editor.token, `"3"`).status)
	assert.Equal(t, http.StatusNotFound, sendJSON(t, s.app, http.MethodPost, planPath(s.plan, "/compartir"), nil, editor.token).StatusCode)
	grupoCall(t, s.app, http.MethodPatch, gruposPath+"/"+v.ID+"/integrantes/"+lector.id(), map[string]string{"permiso_plan": "editar"}, editor.token, http.StatusForbidden)

	// El lector solo ve; el organizador le da y le quita el permiso.
	assert.Equal(t, http.StatusForbidden, planCall(t, s.app, http.MethodPatch, planPath(s.plan, ""), map[string]any{"nombre": "Mío"}, lector.token, `"3"`).status)
	v = grupoCall(t, s.app, http.MethodPatch, gruposPath+"/"+v.ID+"/integrantes/"+lector.id(), map[string]string{"permiso_plan": "editar"}, s.org.token, http.StatusOK)
	assert.Equal(t, planes.PermissionEdit, permisoDe(v, lector.id()))
	assert.Equal(t, http.StatusOK, planCall(t, s.app, http.MethodPatch, planPath(s.plan, ""), map[string]any{"nombre": "Ahora sí"}, lector.token, `"3"`).status)
	v = grupoCall(t, s.app, http.MethodPatch, gruposPath+"/"+v.ID+"/integrantes/"+editor.id(), map[string]string{"permiso_plan": "ver"}, s.org.token, http.StatusOK)
	assert.Equal(t, planes.PermissionView, permisoDe(v, editor.id()))
	assert.Equal(t, http.StatusForbidden, planCall(t, s.app, http.MethodPatch, planPath(s.plan, ""), map[string]any{"nombre": "No"}, editor.token, `"4"`).status)

	// Al organizador no se le cambia (422) y quien no está es 404.
	grupoCall(t, s.app, http.MethodPatch, gruposPath+"/"+v.ID+"/integrantes/"+s.org.id(), map[string]string{"permiso_plan": "ver"}, s.org.token, http.StatusUnprocessableEntity)
	grupoCall(t, s.app, http.MethodPatch, gruposPath+"/"+v.ID+"/integrantes/"+bson.NewObjectID().Hex(), map[string]string{"permiso_plan": "ver"}, s.org.token, http.StatusNotFound)
	grupoCall(t, s.app, http.MethodPatch, gruposPath+"/"+v.ID+"/integrantes/"+lector.id(), map[string]string{"permiso_plan": "dueno"}, s.org.token, http.StatusUnprocessableEntity)
}

// Un integrante con `editar` reserva todo el plan; con `ver`, 403.
func TestReservarPlan_IntegranteConEditar(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, _ := dueno(t, env, "nv89-int@wherever.test", reservastest.Config{})
	owner, ownerToken := cliente(t, env, "nv89-int-owner@wherever.test")
	editor, editorToken := cliente(t, env, "nv89-int-editor@wherever.test")
	_, lectorToken := cliente(t, env, "nv89-int-lector@wherever.test")
	p := planConParadas(t, env, owner.ID, map[bson.ObjectID]time.Time{b.ID: reservastest.Manana(11)})

	ver := compartirPlan(t, app, p, ownerToken, nil)
	grupoCall(t, app, http.MethodPost, gruposPath+"/unirse", map[string]string{"codigo": ver.Code}, lectorToken, http.StatusOK)
	status, _, _ := reservarTodo(t, app, p, lectorToken, llave(8950), nil)
	assert.Equal(t, http.StatusForbidden, status, "con `ver` no reserva")

	edit := compartirPlan(t, app, p, ownerToken, map[string]string{"permiso": "editar"})
	grupoCall(t, app, http.MethodPost, gruposPath+"/unirse", map[string]string{"codigo": edit.Code}, editorToken, http.StatusOK)
	status, out, raw := reservarTodo(t, app, p, editorToken, llave(8951), nil)
	require.Equal(t, http.StatusOK, status, raw)
	r := resultadoDe(t, out, b.ID)
	assert.Equal(t, reservas.ParadaReservada, r.Estado)
	stored, err := reservas.NewRepository(env.Mongo.DB()).FindByID(context.Background(), mustID(t, r.Reserva.ID))
	require.NoError(t, err)
	assert.Equal(t, editor.ID, stored.UsuarioID, "la reserva es de quien la hizo")

	// La misma llave de otra persona no le devuelve la reserva del editor.
	status, ajena, raw := reservarTodo(t, app, p, ownerToken, llave(8951), nil)
	require.Equal(t, http.StatusOK, status, raw)
	assert.Equal(t, reservas.ParadaYaReservada, resultadoDe(t, ajena, b.ID).Estado)
	assert.Nil(t, resultadoDe(t, ajena, b.ID).Reserva)
}

// nv-89: reservar todo con dos paradas cuesta una escritura autenticada, la ventana
// `reservas` una sola vez (INCRBY + EXPIRE NX + PTTL) y, por parada, lo de POST /reservas
// (6 medidos). Repetir la llave cuesta SET NX + GET por parada y no toca la ventana.
func TestPresupuestoRedis_ReservarTodoNV89(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	access := registerHTTP(t, app, "presupuesto89@example.com")
	a, _ := dueno(t, env, "presupuesto89-a@example.com", reservastest.Config{Capacidad: 20})
	b, _ := dueno(t, env, "presupuesto89-b@example.com", reservastest.Config{Capacidad: 20})
	owner, token := cliente(t, env, "presupuesto89-cliente@example.com")
	p := planConParadas(t, env, owner.ID, map[bson.ObjectID]time.Time{a.ID: reservastest.Manana(10), b.ID: reservastest.Manana(12)})

	read := commandsOf(t, func() {
		assert.Equal(t, http.StatusOK, sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access).StatusCode)
	})
	write := commandsOf(t, func() {
		resp := sendJSON(t, app, http.MethodPatch, "/api/v1/usuarios/yo/privacidad", map[string]any{"datos_sensibles": false}, access)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
	window := commandsOf(t, func() {
		assert.Equal(t, http.StatusOK, sendJSON(t, app, http.MethodGet, "/api/v1/personas/buscar?q=nadie", nil, access).StatusCode)
	}) - read
	// Primera reserva: carga el script de liberar el candado (EVALSHA falla y va EVAL).
	requireStatus(t, reservar(t, app, token, llave(8990), cuerpoReserva(a, reservastest.Manana(9), 1)), http.StatusCreated)

	nueva := commandsOf(t, func() {
		status, _, raw := reservarTodo(t, app, p, token, llave(8991), nil)
		require.Equal(t, http.StatusOK, status, raw)
	})
	repetida := commandsOf(t, func() {
		status, _, raw := reservarTodo(t, app, p, token, llave(8991), nil)
		require.Equal(t, http.StatusOK, status, raw)
	})
	t.Logf("reservar todo (2 paradas): %d comandos; repetida: %d", nueva, repetida)
	assert.Equal(t, write+window+2*6, nueva, "escritura + ventana una vez + 6 por parada")
	assert.Equal(t, write+2*2, repetida, "escritura + SET NX y GET por parada, sin ventana")
}
