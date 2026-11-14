package integration

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/expediente"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// Fuentes y observadores de prueba: ocupan el lugar de reservas (nv-59), cupones (nv-61) y
// reseñas (nv-62), que aún no existen. La base y Redis son reales.

type reservasPrueba struct {
	tardar  time.Duration
	futuras bool
}

func (r reservasPrueba) ResumenReservas(ctx context.Context, _ bson.ObjectID, dia negocios.Periodo) (negocios.ResumenReservas, error) {
	select {
	case <-time.After(r.tardar):
	case <-ctx.Done():
		return negocios.ResumenReservas{}, ctx.Err()
	}
	return negocios.ResumenReservas{Hoy: 4, Proxima: &negocios.ProximaReserva{Hora: dia.Desde.Add(14 * time.Hour), Personas: 3}}, nil
}

func (r reservasPrueba) HayReservasFuturas(context.Context, bson.ObjectID, time.Time) (bool, error) {
	return r.futuras, nil
}

type resenasPrueba struct{}

func (resenasPrueba) ResumenResenas(context.Context, bson.ObjectID, negocios.Periodo) (negocios.ResumenResenas, error) {
	return negocios.ResumenResenas{LlegadasHoy: 2, CalificacionSemana: 4.6, CambioVsSemanaAnterior: 0.2, PorResponder: 1, Verificadas90: 25, Promedio90: 4.7}, nil
}

type observadorPrueba struct {
	mu     sync.Mutex
	avisos []string
	falla  error
}

func (o *observadorPrueba) anotar(evento string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.falla != nil {
		return o.falla
	}
	o.avisos = append(o.avisos, evento)
	return nil
}

func (o *observadorPrueba) Pausado(context.Context, bson.ObjectID) error {
	return o.anotar(negocios.AvisoPausado)
}
func (o *observadorPrueba) Reanudado(context.Context, bson.ObjectID) error {
	return o.anotar(negocios.AvisoReanudado)
}
func (o *observadorPrueba) DadoDeBaja(context.Context, bson.ObjectID) error {
	return o.anotar(negocios.AvisoDadoDeBaja)
}

func (o *observadorPrueba) fallar(err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.falla = err
}

func (o *observadorPrueba) vistos() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string{}, o.avisos...)
}

func versionDe(t *testing.T, body map[string]any) int {
	t.Helper()
	v, ok := body["version"].(float64)
	require.True(t, ok, "falta version")
	return int(v)
}

func merchantToken(t *testing.T, id bson.ObjectID, jti string) string {
	t.Helper()
	return testutil.ValidTokenFor(t, id.Hex(), jti, middleware.RoleConsumer, middleware.RoleMerchant)
}

func TestNegociosPanel_FuentesNullYLenta(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	u := env.CreateUser(t, "duena-panel@wherever.test")
	b := env.AltaAprobada(t, u, "Panel Fonda")
	token := merchantToken(t, u.ID, "jti-panel")
	path := "/api/v1/negocios/" + b.ID + "/panel"

	// Sin fuentes registradas: las tarjetas en null y 200.
	resp := sendJSON(t, app, http.MethodGet, path, nil, token)
	requireStatus(t, resp, http.StatusOK)
	body := decode(t, resp)
	assertSnakeCase(t, body)
	for _, k := range []string{"reservas_hoy", "proxima_reserva", "cupones_canjeados_hoy", "cupones_activos", "llegadas_hoy", "calificacion_semana", "resenas_por_responder"} {
		assert.Contains(t, body, k)
		assert.Nil(t, body[k], k)
	}
	assert.Equal(t, negocios.StatusApproved, body["estado"])
	assert.Equal(t, false, body["datos_fiscales_completos"])
	assert.Equal(t, false, obj(t, body["avance_sello"])["elegible"])

	// Con fuentes: reservas y reseñas llenan su tarjeta; cupones sigue en null.
	env.Panel.WithFuentes(negocios.PanelFuente{Reservas: reservasPrueba{}, Resenas: resenasPrueba{}})
	resp = sendJSON(t, app, http.MethodGet, path, nil, token)
	requireStatus(t, resp, http.StatusOK)
	body = decode(t, resp)
	assert.Equal(t, float64(4), body["reservas_hoy"])
	assert.Equal(t, float64(3), obj(t, body["proxima_reserva"])["personas"])
	assert.Equal(t, 4.6, body["calificacion_semana"])
	assert.Nil(t, body["cupones_activos"])

	// Una fuente lenta (más que su plazo) deja null y el panel sigue en 200.
	env.Panel.WithFuentes(negocios.PanelFuente{Reservas: reservasPrueba{tardar: time.Second}, Resenas: resenasPrueba{}}).WithPlazoFuente(200 * time.Millisecond)
	inicio := time.Now()
	resp = sendJSON(t, app, http.MethodGet, path, nil, token)
	requireStatus(t, resp, http.StatusOK)
	body = decode(t, resp)
	assert.Nil(t, body["reservas_hoy"])
	assert.Equal(t, float64(2), body["llegadas_hoy"])
	assert.Less(t, time.Since(inicio), time.Second)
}

func TestNegociosPanel_PerfilYCambiosPendientes(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	ctx := context.Background()
	u := env.CreateUser(t, "duena-perfil@wherever.test")
	b := env.AltaAprobada(t, u, "Perfil Original")
	token := merchantToken(t, u.ID, "jti-perfil")
	admin := env.CreateAdmin(t, "admin-perfil@wherever.test")
	adminToken := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-perfil", middleware.RoleAdmin)
	path := "/api/v1/negocios/" + b.ID

	requireStatus(t, sendJSON(t, app, http.MethodPatch, path, map[string]any{"descripcion": "x"}, token), http.StatusBadRequest)
	requireStatus(t, withIfMatch(t, app, http.MethodPatch, path, map[string]any{"descripcion": "x"}, token, b.Version-1), http.StatusConflict)

	horarios := []map[string]any{{"dia": 2, "abre": "08:00", "cierra": "16:00"}}
	resp := withIfMatch(t, app, http.MethodPatch, path, map[string]any{"horarios": horarios, "nombre": "Perfil Nuevo"}, token, b.Version)
	requireStatus(t, resp, http.StatusOK)
	body := decode(t, resp)
	assert.Equal(t, "Perfil Original", body["nombre"], "el nombre espera al administrador")
	assert.Equal(t, []any{"nombre"}, obj(t, body["cambios_pendientes"])["campos"])
	version := versionDe(t, body)

	oid, err := bson.ObjectIDFromHex(b.ID)
	require.NoError(t, err)
	stored, err := negocios.NewRepository(env.Mongo.DB()).FindByID(ctx, oid)
	require.NoError(t, err)
	assert.Equal(t, negocios.DeriveScheduleMin([]negocios.Horario{{Dia: 2, Abre: "08:00", Cierra: "16:00"}}), stored.ScheduleMin, "recalcula horario_min")

	// Sigue publicado con el nombre de antes.
	resp = sendJSON(t, app, http.MethodGet, "/api/v1/lugares/"+b.ID, nil, "")
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, "Perfil Original", decode(t, resp)["nombre"])

	resp = sendJSON(t, app, http.MethodGet, "/api/v1/admin/negocios?estado=cambios_pendientes", nil, adminToken)
	requireStatus(t, resp, http.StatusOK)
	items := list(t, decode(t, resp)["items"])
	require.Len(t, items, 1)
	assert.Equal(t, "Perfil Nuevo", obj(t, obj(t, items[0])["cambios_pendientes"])["nombre"])

	resp = withIfMatch(t, app, http.MethodPatch, "/api/v1/admin/negocios/"+b.ID+"/revision", map[string]any{"decision": "aprobar"}, adminToken, version)
	requireStatus(t, resp, http.StatusOK)
	resp = sendJSON(t, app, http.MethodGet, "/api/v1/lugares/"+b.ID, nil, "")
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, "Perfil Nuevo", decode(t, resp)["nombre"])
	stored, err = negocios.NewRepository(env.Mongo.DB()).FindByID(ctx, oid)
	require.NoError(t, err)
	assert.Nil(t, stored.CambiosPendientes)
	assert.Equal(t, negocios.StatusApproved, stored.State)
}

func TestNegociosPanel_PausaObservadorYReanudacion(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	ctx := context.Background()
	obs := &observadorPrueba{}
	env.Panel.WithObservadores(negocios.Observadores{obs})
	u := env.CreateUser(t, "duena-pausa@wherever.test")
	b := env.AltaAprobada(t, u, "Pausa Local")
	token := merchantToken(t, u.ID, "jti-pausa")
	path := "/api/v1/negocios/" + b.ID + "/pausa"

	requireStatus(t, sendJSON(t, app, http.MethodPost, path, map[string]any{"pausado": true, "hasta": time.Now().Add(91 * 24 * time.Hour)}, token), http.StatusUnprocessableEntity)
	requireStatus(t, sendJSON(t, app, http.MethodPost, path, map[string]any{"pausado": false}, token), http.StatusConflict)

	resp := sendJSON(t, app, http.MethodPost, path, map[string]any{"pausado": true, "hasta": time.Now().Add(time.Hour), "motivo": "Vacaciones"}, token)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, negocios.StatusPaused, decode(t, resp)["estado"])
	requireStatus(t, sendJSON(t, app, http.MethodGet, "/api/v1/lugares/"+b.ID, nil, ""), http.StatusNotFound)
	requireStatus(t, sendJSON(t, app, http.MethodGet, "/api/v1/lugares/"+b.ID+"/menu", nil, ""), http.StatusNotFound)
	assert.Equal(t, []string{negocios.AvisoPausado}, obs.vistos())

	// El observador falla: la reanudación queda, el aviso queda pendiente y la noche lo reintenta.
	oid, err := bson.ObjectIDFromHex(b.ID)
	require.NoError(t, err)
	_, err = env.Mongo.DB().Collection(negocios.Collection).UpdateOne(ctx, bson.D{{Key: "_id", Value: oid}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "pausado.hasta", Value: time.Now().Add(-time.Minute)}}}})
	require.NoError(t, err)
	obs.fallar(assert.AnError)
	n, err := env.PanelTarea.Run(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	stored, err := negocios.NewRepository(env.Mongo.DB()).FindByID(ctx, oid)
	require.NoError(t, err)
	assert.Equal(t, negocios.StatusApproved, stored.State)
	assert.Nil(t, stored.Paused)
	assert.Equal(t, []string{negocios.AvisoReanudado}, stored.AvisosPendientes)
	requireStatus(t, sendJSON(t, app, http.MethodGet, "/api/v1/lugares/"+b.ID, nil, ""), http.StatusOK)

	obs.fallar(nil)
	_, err = env.PanelTarea.Run(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{negocios.AvisoPausado, negocios.AvisoReanudado}, obs.vistos())
	stored, err = negocios.NewRepository(env.Mongo.DB()).FindByID(ctx, oid)
	require.NoError(t, err)
	assert.Empty(t, stored.AvisosPendientes)
}

func TestNegociosPanel_ReservasConfigYQR(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	u := env.CreateUser(t, "duena-reservas@wherever.test")
	b := env.AltaAprobada(t, u, "Reservas Local")
	token := merchantToken(t, u.ID, "jti-reservas")
	path := "/api/v1/negocios/" + b.ID + "/reservas-config"
	cfg := map[string]any{"acepta_reservas": true, "capacidad_por_horario": 30, "duracion_turno_min": 60,
		"anticipo": map[string]any{"tipo": "fijo", "valor": 0}, "politica_cancelacion": "flexible", "confirmacion": "manual", "permite_cambios": false}

	resp := withIfMatch(t, app, http.MethodPut, path, cfg, token, b.Version)
	requireStatus(t, resp, http.StatusOK)
	body := decode(t, resp)
	assert.Equal(t, float64(30), obj(t, body["reservas_config"])["capacidad_por_horario"])
	version := versionDe(t, body)

	oid, err := bson.ObjectIDFromHex(b.ID)
	require.NoError(t, err)
	stored, err := negocios.NewRepository(env.Mongo.DB()).FindByID(context.Background(), oid)
	require.NoError(t, err)
	assert.True(t, stored.AcceptsReservations, "copia acepta_reservas al campo de /lugares")
	require.NotNil(t, stored.ReservasConfig)
	assert.Equal(t, "manual", stored.ReservasConfig.Confirmacion)

	cfg["anticipo"] = map[string]any{"tipo": "por_persona", "valor": 5000}
	resp = withIfMatch(t, app, http.MethodPut, path, cfg, token, version)
	requireStatus(t, resp, http.StatusUnprocessableEntity)
	assert.Contains(t, readAll(t, resp), "cobros_no_activos")

	resp = sendJSON(t, app, http.MethodGet, "/api/v1/negocios/"+b.ID+"/qr", nil, token)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, "private, max-age=86400", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "wherever://checkin/"+stored.QRCheckin, decode(t, resp)["contenido"])
}

func TestNegociosPanel_BajaYSello(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	ctx := context.Background()
	obs := &observadorPrueba{}
	env.Panel.WithObservadores(negocios.Observadores{obs})
	u := env.CreateUser(t, "duena-baja@wherever.test")
	b := env.AltaAprobada(t, u, "Baja Local")
	token := merchantToken(t, u.ID, "jti-baja")
	admin := env.CreateAdmin(t, "admin-sello@wherever.test")
	adminToken := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-sello", middleware.RoleAdmin)
	oid, err := bson.ObjectIDFromHex(b.ID)
	require.NoError(t, err)

	// Sello otorgado y retirado.
	sello := "/api/v1/admin/negocios/" + b.ID + "/sello"
	requireStatus(t, sendJSON(t, app, http.MethodPut, sello, map[string]any{"otorgar": true, "motivo": "corto"}, adminToken), http.StatusUnprocessableEntity)
	resp := sendJSON(t, app, http.MethodPut, sello, map[string]any{"otorgar": true, "motivo": "Cumple la regla del Sello"}, adminToken)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, true, decode(t, resp)["sello_excelencia"])
	stored, err := negocios.NewRepository(env.Mongo.DB()).FindByID(ctx, oid)
	require.NoError(t, err)
	require.NotNil(t, stored.Sello)
	assert.Equal(t, admin.ID, stored.Sello.OtorgadoPor)
	resp = sendJSON(t, app, http.MethodPut, sello, map[string]any{"otorgar": false, "motivo": "Reseñas bajo revisión"}, adminToken)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, false, decode(t, resp)["sello_excelencia"])

	// Baja: confirmación exacta y sin reservas futuras.
	path := "/api/v1/negocios/" + b.ID
	requireStatus(t, sendJSON(t, app, http.MethodDelete, path, map[string]any{"confirmacion": "baja local"}, token), http.StatusUnprocessableEntity)
	env.Panel.WithReservasFuturas(reservasPrueba{futuras: true})
	resp = sendJSON(t, app, http.MethodDelete, path, map[string]any{"confirmacion": "Baja Local"}, token)
	requireStatus(t, resp, http.StatusConflict)
	assert.Contains(t, readAll(t, resp), "reservas_futuras")

	env.Panel.WithReservasFuturas(reservasPrueba{})
	resp = sendJSON(t, app, http.MethodDelete, path, map[string]any{"confirmacion": "Baja Local"}, token)
	requireStatus(t, resp, http.StatusNoContent)
	assert.Empty(t, readAll(t, resp))
	assert.Equal(t, []string{negocios.AvisoDadoDeBaja}, obs.vistos())
	requireStatus(t, sendJSON(t, app, http.MethodGet, "/api/v1/lugares/"+b.ID, nil, ""), http.StatusNotFound)
	requireStatus(t, sendJSON(t, app, http.MethodGet, path+"/panel", nil, token), http.StatusNotFound)

	// La baja cancela el expediente: INE y comprobante borrados.
	vigentes, err := env.Expediente.Vigentes(ctx, oid)
	require.NoError(t, err)
	for _, tipo := range expediente.TiposArchivo {
		assert.False(t, vigentes[tipo], tipo)
	}
	stored, err = negocios.NewRepository(env.Mongo.DB()).FindByID(ctx, oid)
	require.NoError(t, err)
	assert.NotNil(t, stored.DeletedAt)
	assert.Equal(t, "Baja Local", stored.Name, "el nombre queda para reseñas pasadas")
	assert.NotEmpty(t, stored.QRCheckin)
}
