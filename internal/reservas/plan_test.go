package reservas

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func TestTurnoDe_LaLlegadaCaeEnSuTurno(t *testing.T) {
	turnos := Turnos([]negocios.Horario{{Dia: 1, Abre: "13:00", Cierra: "16:00"}}, 60, lunes(), cdmx)

	got, ok := TurnoDe(turnos, 60, local(14, 0))
	require.True(t, ok)
	assert.Equal(t, local(14, 0), got, "llegar al inicio del turno")

	got, ok = TurnoDe(turnos, 60, local(14, 35))
	require.True(t, ok)
	assert.Equal(t, local(14, 0), got, "llegar a media hora: el turno que la contiene")

	_, ok = TurnoDe(turnos, 60, local(16, 0))
	assert.False(t, ok, "el cierre ya no tiene turno")
	_, ok = TurnoDe(turnos, 60, local(12, 59))
	assert.False(t, ok, "antes de abrir")
	_, ok = TurnoDe(nil, 60, local(14, 0))
	assert.False(t, ok, "sin turnos")
}

func TestMismaParada_SoloReservasQueOcupan(t *testing.T) {
	now := time.Now()
	plan, parada, otra := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	nueva := Reserva{PlanID: &plan, ParadaID: &parada}
	vencida := now.Add(-time.Minute)

	assert.True(t, MismaParada([]Reserva{{PlanID: &plan, ParadaID: &parada, Estado: EstadoConfirmada}}, nueva, now))
	assert.False(t, MismaParada([]Reserva{{PlanID: &plan, ParadaID: &parada, Estado: EstadoCancelada}}, nueva, now), "cancelada no cuenta")
	assert.False(t, MismaParada([]Reserva{{PlanID: &plan, ParadaID: &parada, Estado: EstadoBloqueada, BloqueoExpiraEn: &vencida}}, nueva, now), "apartado vencido")
	assert.False(t, MismaParada([]Reserva{{PlanID: &plan, ParadaID: &otra, Estado: EstadoConfirmada}}, nueva, now), "otra parada")
	assert.False(t, MismaParada([]Reserva{{Estado: EstadoConfirmada}}, nueva, now), "reserva suelta")
	assert.False(t, MismaParada([]Reserva{{PlanID: &plan, ParadaID: &parada, Estado: EstadoConfirmada}}, Reserva{}, now), "la nueva sin plan")
}

func TestConReserva_EstadoDeLaParada(t *testing.T) {
	cases := map[string]string{
		EstadoBloqueada:  ParadaPendiente,
		EstadoPendiente:  ParadaReservada,
		EstadoConfirmada: ParadaReservada,
		EstadoCancelada:  ParadaNoReservada,
		EstadoExpirada:   ParadaNoReservada,
	}
	for estado, want := range cases {
		got := conReserva(PlanResultado{}, ReservaResponse{Estado: estado})
		assert.Equal(t, want, got.Estado, estado)
		require.NotNil(t, got.Reserva)
	}
	assert.Equal(t, EstadoCancelada, conReserva(PlanResultado{}, ReservaResponse{Estado: EstadoCancelada}).Motivo)
}

func TestMotivo_TraduceLosRechazos(t *testing.T) {
	cases := []struct {
		err    error
		estado string
		motivo string
	}{
		{errYaReservada, ParadaYaReservada, ""},
		{errOcupado(), ParadaSinCupo, ""},
		{invalid("fecha_hora", "no es un horario del negocio"), ParadaFueraHorario, ""},
		{apperr.Validation("no_acepta_reservas"), ParadaNoAcepta, ""},
		{errNegocioNotFound(), ParadaNoAcepta, "negocio_no_disponible"},
		{apperr.Conflict("demasiados_apartados"), ParadaNoReservada, "demasiados_apartados"},
		{apperr.Validation("anticipo_no_disponible"), ParadaNoReservada, "anticipo_no_disponible"},
	}
	for _, c := range cases {
		got, err := motivo(PlanResultado{}, c.err)
		require.NoError(t, err, c.err.Error())
		assert.Equal(t, c.estado, got.Estado, c.err.Error())
		assert.Equal(t, c.motivo, got.Motivo, c.err.Error())
	}
}

func TestMotivo_ErrorDelServicioCortaLaPeticion(t *testing.T) {
	_, err := motivo(PlanResultado{}, errStore())
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.KindUnavailable, appErr.Kind)

	_, err = motivo(PlanResultado{}, errors.New("redis"))
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.KindUnavailable, appErr.Kind, "un error crudo no se filtra")
}

func TestPlanRequest_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	assert.NoError(t, v.Struct(PlanRequest{}), "vacío: las personas del plan")
	assert.NoError(t, v.Struct(PlanRequest{Personas: 20, Nota: "cumpleaños"}))
	assert.Error(t, v.Struct(PlanRequest{Personas: 21}))
	assert.Error(t, v.Struct(PlanRequest{Personas: -1}))
}
