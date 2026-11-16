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

	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

// Recordatorio de 2 h (nv-73): una sola vez por reserva aunque corran dos barridos a la vez.
func TestReservasRecordatorio_UnoSoloYVentana(t *testing.T) {
	env := authEnv(t)
	b, _ := dueno(t, env, "duena-recordatorio@wherever.test", reservastest.Config{})
	pronto, _ := cliente(t, env, "pronto-recordatorio@wherever.test")
	lejos, _ := cliente(t, env, "lejos-recordatorio@wherever.test")
	cancelo, _ := cliente(t, env, "cancelo-recordatorio@wherever.test")
	now := time.Now().UTC()
	r := reservaSembrada(t, env, pronto, b, now.Add(110*time.Minute), reservas.EstadoConfirmada)
	reservaSembrada(t, env, lejos, b, now.Add(3*time.Hour), reservas.EstadoConfirmada)
	reservaSembrada(t, env, cancelo, b, now.Add(time.Hour), reservas.EstadoCancelada)

	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			_, _, err := env.Reservas.Barrer(context.Background())
			assert.NoError(t, err)
		})
	}
	wg.Wait()

	avisos := avisosDe(t, env, pronto.ID, notificaciones.KindReservaRecordatorio)
	require.Len(t, avisos, 1, "dos barridos, un aviso")
	assert.Equal(t, map[string]string{"reserva_id": r.ID.Hex(), "negocio_id": b.ID.Hex()}, avisos[0].Data)
	assert.Empty(t, avisosDe(t, env, lejos.ID, notificaciones.KindReservaRecordatorio), "a 3 h todavía no")
	assert.Empty(t, avisosDe(t, env, cancelo.ID, notificaciones.KindReservaRecordatorio), "cancelada nunca")

	guardada, err := reservas.NewRepository(env.Mongo.DB()).FindByID(context.Background(), r.ID)
	require.NoError(t, err)
	require.NotNil(t, guardada.RecordatorioEn)
	assert.Equal(t, r.Version, guardada.Version, "marcar el aviso no cambia la versión")

	n, err := env.Reservas.Recordar(context.Background())
	require.NoError(t, err)
	assert.Zero(t, n, "otra corrida no repite")
}

// Cambiar la hora borra `recordatorio_en`: el aviso saldrá otra vez con la hora nueva.
func TestReservasRecordatorio_CambioDeHoraLoReinicia(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, _ := dueno(t, env, "duena-recordatorio-cambio@wherever.test", reservastest.Config{PermiteCambios: true})
	u, token := cliente(t, env, "cliente-recordatorio-cambio@wherever.test")
	resp := reservar(t, app, token, llave(730), cuerpoReserva(b, reservastest.Manana(14), 2))
	requireStatus(t, resp, http.StatusCreated)
	id, err := bson.ObjectIDFromHex(str(t, decode(t, resp)["id"]))
	require.NoError(t, err)
	_, err = env.Mongo.DB().Collection(reservas.Collection).UpdateOne(context.Background(), bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "recordatorio_en", Value: time.Now().UTC()}}}})
	require.NoError(t, err)

	resp = sendWithHeaders(t, app, http.MethodPatch, "/api/v1/reservas/"+id.Hex(),
		map[string]any{"fecha_hora": reservastest.Manana(15).Format(time.RFC3339)}, token, map[string]string{"If-Match": `"1"`})
	requireStatus(t, resp, http.StatusOK)
	guardada, err := reservas.NewRepository(env.Mongo.DB()).FindByID(context.Background(), id)
	require.NoError(t, err)
	assert.Nil(t, guardada.RecordatorioEn, "la hora nueva vuelve a avisar")

	// Cambiar solo las personas lo conserva.
	_, err = env.Mongo.DB().Collection(reservas.Collection).UpdateOne(context.Background(), bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "recordatorio_en", Value: time.Now().UTC()}}}})
	require.NoError(t, err)
	resp = sendWithHeaders(t, app, http.MethodPatch, "/api/v1/reservas/"+id.Hex(), map[string]any{"personas": 3}, token, map[string]string{"If-Match": `"2"`})
	requireStatus(t, resp, http.StatusOK)
	guardada, err = reservas.NewRepository(env.Mongo.DB()).FindByID(context.Background(), id)
	require.NoError(t, err)
	assert.NotNil(t, guardada.RecordatorioEn)
	assert.Empty(t, avisosDe(t, env, u.ID, notificaciones.KindReservaRecordatorio))
}
