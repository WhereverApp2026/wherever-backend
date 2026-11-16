package reservas

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func TestMensajeRequest_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	cases := []struct {
		name  string
		texto string
		ok    bool
	}{
		{"normal", "Tu mesa está lista, te esperamos", true},
		{"500 caracteres", strings.Repeat("a", 500), true},
		{"500 con acentos", strings.Repeat("á", 500), true},
		{"vacío", "", false},
		{"solo espacios", "   \n\t ", false},
		{"501 caracteres", strings.Repeat("a", 501), false},
		{"HTML", "<b>hola</b>", false},
		{"script", "hola <script>alert(1)</script>", false},
		{"mayor que suelto", "llegas > 10 min", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := MensajeRequest{Texto: tc.texto}
			req.Normalizar()
			err := v.Struct(&req)
			if tc.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestMensajeRequest_Recorta(t *testing.T) {
	req := MensajeRequest{Texto: "  hola  "}
	req.Normalizar()
	assert.Equal(t, "hola", req.Texto)
}

func TestReserva_Contactable(t *testing.T) {
	now := time.Date(2026, 11, 16, 18, 0, 0, 0, time.UTC)
	cases := []struct {
		estado string
		hora   time.Time
		ok     bool
	}{
		{EstadoConfirmada, now.Add(3 * time.Hour), true},
		{EstadoConfirmada, now.Add(-time.Hour), true},
		{EstadoAsistio, now.Add(-23 * time.Hour), true},
		{EstadoNoAsistio, now.Add(-time.Hour), true},
		{EstadoAsistio, now.Add(-24 * time.Hour), false},
		{EstadoNoAsistio, now.Add(-48 * time.Hour), false},
		{EstadoBloqueada, now.Add(time.Hour), false},
		{EstadoPendiente, now.Add(time.Hour), false},
		{EstadoCancelada, now.Add(time.Hour), false},
		{EstadoExpirada, now.Add(time.Hour), false},
	}
	for _, tc := range cases {
		r := Reserva{Estado: tc.estado, FechaHora: tc.hora}
		assert.Equal(t, tc.ok, r.Contactable(now), "%s a %s", tc.estado, tc.hora.Sub(now))
	}
}

func TestReserva_EnVentanaRecordatorio(t *testing.T) {
	now := time.Date(2026, 11, 16, 18, 0, 0, 0, time.UTC)
	avisada := now.Add(-time.Minute)
	cases := []struct {
		name string
		r    Reserva
		ok   bool
	}{
		{"a 1 h 50 min", Reserva{Estado: EstadoConfirmada, FechaHora: now.Add(110 * time.Minute)}, true},
		{"justo a 2 h", Reserva{Estado: EstadoConfirmada, FechaHora: now.Add(2 * time.Hour)}, true},
		{"a 2 h y 1 s", Reserva{Estado: EstadoConfirmada, FechaHora: now.Add(2*time.Hour + time.Second)}, false},
		{"a 3 h", Reserva{Estado: EstadoConfirmada, FechaHora: now.Add(3 * time.Hour)}, false},
		{"ya empezó", Reserva{Estado: EstadoConfirmada, FechaHora: now}, false},
		{"ya avisada", Reserva{Estado: EstadoConfirmada, FechaHora: now.Add(time.Hour), RecordatorioEn: &avisada}, false},
		{"cancelada", Reserva{Estado: EstadoCancelada, FechaHora: now.Add(time.Hour)}, false},
		{"pendiente", Reserva{Estado: EstadoPendiente, FechaHora: now.Add(time.Hour)}, false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.ok, tc.r.EnVentanaRecordatorio(now), tc.name)
	}
}

func TestEsperaMensaje(t *testing.T) {
	now := time.Date(2026, 11, 16, 18, 0, 0, 0, time.UTC)
	assert.Equal(t, 3600, esperaMensaje(now.Add(-23*time.Hour), now), "el más viejo sale en 1 h")
	assert.Equal(t, 1, esperaMensaje(now.Add(-24*time.Hour), now), "nunca menos de 1 s")
	assert.Equal(t, 2, esperaMensaje(now.Add(-24*time.Hour+1500*time.Millisecond), now), "redondea hacia arriba")
}
