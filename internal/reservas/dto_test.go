package reservas

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func TestDTO_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	ok := CrearRequest{NegocioID: bson.NewObjectID().Hex(), FechaHora: "2026-11-16T20:00:00Z", Personas: 2}
	with := func(f func(*CrearRequest)) CrearRequest { r := ok; f(&r); return r }
	cero, veintiuno, dos := 0, 21, 2
	malaHora := "mañana"
	cases := []struct {
		name string
		dto  any
		ok   bool
	}{
		{"mínima", ok, true},
		{"con zona", with(func(r *CrearRequest) { r.FechaHora = "2026-11-16T14:00:00-06:00" }), true},
		{"sin negocio", with(func(r *CrearRequest) { r.NegocioID = "" }), false},
		{"negocio no ObjectId", with(func(r *CrearRequest) { r.NegocioID = "abc" }), false},
		{"hora sin zona", with(func(r *CrearRequest) { r.FechaHora = "2026-11-16T20:00:00" }), false},
		{"0 personas", with(func(r *CrearRequest) { r.Personas = 0 }), false},
		{"21 personas", with(func(r *CrearRequest) { r.Personas = 21 }), false},
		{"nota larga", with(func(r *CrearRequest) { r.Nota = strings.Repeat("a", 141) }), false},
		{"parada no ObjectId", with(func(r *CrearRequest) { r.ParadaID = "x" }), false},
		{"cambio vacío", CambiarRequest{}, true},
		{"cambio a 2", CambiarRequest{Personas: &dos}, true},
		{"cambio a 0", CambiarRequest{Personas: &cero}, false},
		{"cambio a 21", CambiarRequest{Personas: &veintiuno}, false},
		{"cambio hora mala", CambiarRequest{FechaHora: &malaHora}, false},
		{"disponibilidad", DisponibilidadQuery{Fecha: "2026-11-16", Personas: 2}, true},
		{"disponibilidad sin fecha", DisponibilidadQuery{Personas: 2}, false},
		{"disponibilidad fecha mala", DisponibilidadQuery{Fecha: "16/11/2026", Personas: 2}, false},
		{"mis reservas", NewMiasQuery(), true},
		{"mis reservas cuándo raro", MiasQuery{Cuando: "ayer", Limite: 20}, false},
		{"agenda", AgendaQuery{Fecha: "2026-11-16", Estado: EstadoPendiente, Q: "ana"}, true},
		{"agenda estado raro", AgendaQuery{Estado: "x"}, false},
		{"estado confirmar", EstadoRequest{Estado: EstadoConfirmada}, true},
		{"estado cancelar no va aquí", EstadoRequest{Estado: EstadoCancelada}, false},
		{"escanear", EscanearRequest{Codigo: "ABC234"}, true},
		{"escanear corto", EscanearRequest{Codigo: "ABC"}, false},
		{"escanear símbolos", EscanearRequest{Codigo: "AB-234"}, false},
		{"bloqueo", BloqueoRequest{Desde: "2026-11-16T20:00:00Z", Hasta: "2026-11-16T23:00:00Z", Motivo: MotivoLleno}, true},
		{"bloqueo motivo raro", BloqueoRequest{Desde: "2026-11-16T20:00:00Z", Hasta: "2026-11-16T23:00:00Z", Motivo: "x"}, false},
		{"cancelar motivo largo", CancelarRequest{Motivo: strings.Repeat("a", 141)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Struct(tc.dto)
			if tc.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestUUIDPattern(t *testing.T) {
	assert.True(t, uuidPattern.MatchString("2f1c8a4e-1b7d-4c2a-9e3f-5a6b7c8d9e0f"))
	assert.False(t, uuidPattern.MatchString("2f1c8a4e"))
	assert.False(t, uuidPattern.MatchString(""))
}
