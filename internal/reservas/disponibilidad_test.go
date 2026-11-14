package reservas

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

var cdmx = negocios.MexicoCity()

// lunes 16 de noviembre de 2026.
func lunes() time.Time { return time.Date(2026, 11, 16, 0, 0, 0, 0, cdmx) }

func local(h, m int) time.Time {
	return time.Date(2026, 11, 16, h, m, 0, 0, cdmx).UTC()
}

func TestTurnos_DentroDelHorarioCadaTurno(t *testing.T) {
	horarios := []negocios.Horario{{Dia: 1, Abre: "13:00", Cierra: "16:00"}, {Dia: 1, Abre: "19:00", Cierra: "20:30"}, {Dia: 2, Abre: "09:00", Cierra: "10:00"}}
	got := Turnos(horarios, 60, lunes(), cdmx)
	assert.Equal(t, []time.Time{local(13, 0), local(14, 0), local(15, 0), local(19, 0)}, got, "el último turno termina antes del cierre")
}

func TestTurnos_DiaCerradoOVacio(t *testing.T) {
	assert.Empty(t, Turnos([]negocios.Horario{{Dia: 1, Abre: "09:00", Cierra: "18:00", Cerrado: true}}, 30, lunes(), cdmx))
	assert.Empty(t, Turnos([]negocios.Horario{{Dia: 3, Abre: "09:00", Cierra: "18:00"}}, 30, lunes(), cdmx), "otro día")
	assert.Empty(t, Turnos([]negocios.Horario{{Dia: 1, Abre: "9", Cierra: "18:00"}}, 30, lunes(), cdmx), "hora mal formada")
	assert.Empty(t, Turnos([]negocios.Horario{{Dia: 1, Abre: "09:00", Cierra: "18:00"}}, 0, lunes(), cdmx), "sin turno")
}

func TestTurnos_CruzaMedianocheYSinRepetir(t *testing.T) {
	horarios := []negocios.Horario{{Dia: 1, Abre: "23:00", Cierra: "01:00"}, {Dia: 1, Abre: "23:00", Cierra: "00:00"}}
	got := Turnos(horarios, 60, lunes(), cdmx)
	require.Len(t, got, 2)
	assert.Equal(t, local(23, 0), got[0])
	assert.Equal(t, local(23, 0).Add(time.Hour), got[1], "el turno de las 00:00 es del día en que abre")
}

func TestTurnos_SinHorarioDeVeranoEnCDMX(t *testing.T) {
	// Antes de 2022 el primer domingo de abril cambiaba la hora; hoy CDMX es UTC−6 todo el año.
	domingo := time.Date(2027, 4, 4, 0, 0, 0, 0, cdmx)
	got := Turnos([]negocios.Horario{{Dia: 0, Abre: "12:00", Cierra: "13:00"}}, 60, domingo, cdmx)
	require.Len(t, got, 1)
	assert.Equal(t, 18, got[0].Hour(), "12:00 en CDMX = 18:00 UTC")
}

func TestDisponibilidad_CupoBloqueosYPasado(t *testing.T) {
	turnos := []time.Time{local(13, 0), local(14, 0), local(15, 0), local(16, 0)}
	now := local(13, 30)
	vence := now.Add(5 * time.Minute)
	vencido := now.Add(-time.Minute)
	rs := []Reserva{
		{FechaHora: local(14, 0), Personas: 6, Estado: EstadoConfirmada},
		{FechaHora: local(14, 0), Personas: 2, Estado: EstadoPendiente},
		{FechaHora: local(14, 0), Personas: 1, Estado: EstadoBloqueada, BloqueoExpiraEn: &vence},
		{FechaHora: local(14, 0), Personas: 9, Estado: EstadoBloqueada, BloqueoExpiraEn: &vencido},
		{FechaHora: local(14, 0), Personas: 9, Estado: EstadoCancelada},
		{FechaHora: local(16, 0), Personas: 12, Estado: EstadoConfirmada},
	}
	bloqueos := []Bloqueo{{ID: bson.NewObjectID(), Desde: local(15, 30), Hasta: local(15, 45)}}
	got := Disponibilidad(turnos, 10, 60, 2, Ocupadas(rs, now), bloqueos, now)
	require.Len(t, got, 4)
	assert.Equal(t, Horario{Hora: local(13, 0), Disponible: false, LugaresLibres: 0}, got[0], "ya empezó")
	assert.Equal(t, Horario{Hora: local(14, 0), Disponible: false, LugaresLibres: 1}, got[1], "6+2+1; el apartado vencido y la cancelada no cuentan")
	assert.Equal(t, Horario{Hora: local(15, 0), Disponible: false, LugaresLibres: 0}, got[2], "el bloqueo se cruza con el turno")
	assert.Equal(t, Horario{Hora: local(16, 0), Disponible: false, LugaresLibres: 0}, got[3], "nunca negativo")

	libre := Disponibilidad([]time.Time{local(14, 0)}, 10, 60, 1, Ocupadas(rs, now), nil, now)
	assert.True(t, libre[0].Disponible, "alcanza para 1")
}

func TestEsTurno(t *testing.T) {
	turnos := []time.Time{local(13, 0), local(14, 0)}
	assert.True(t, EsTurno(turnos, local(14, 0)))
	assert.False(t, EsTurno(turnos, local(14, 30)))
}
