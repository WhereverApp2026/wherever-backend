package pagos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

func TestComision_RedondeaAlCentavo(t *testing.T) {
	assert.Equal(t, int64(1000), Comision(10000, 10))
	assert.Equal(t, int64(5), Comision(45, 10)) // 4.5 → 5
	assert.Equal(t, int64(0), Comision(0, 10))
	assert.Equal(t, int64(0), Comision(5000, 0))
}

func TestDesglose_ComisionProporcionalAlReembolso(t *testing.T) {
	p := Pago{MontoCentavos: 20000, ComisionCentavos: 2000}
	assert.Equal(t, Desglose{Anticipo: 20000, Comision: 2000, Neto: 18000}, DesgloseDe(p))

	p.Reembolsos = []Reembolso{{MontoCentavos: 10000}}
	assert.Equal(t, Desglose{Anticipo: 20000, Reembolsos: 10000, Comision: 1000, Neto: 9000}, DesgloseDe(p))

	p.Reembolsos = append(p.Reembolsos, Reembolso{MontoCentavos: 15000})
	assert.Equal(t, Desglose{Anticipo: 20000, Reembolsos: 20000}, DesgloseDe(p), "lo reembolsado no pasa del monto")

	total := DesgloseDe(Pago{MontoCentavos: 100, ComisionCentavos: 10}).Sumar(DesgloseDe(Pago{MontoCentavos: 300, ComisionCentavos: 30}))
	assert.Equal(t, Desglose{Anticipo: 400, Comision: 40, Neto: 360}, total)
}

func TestSemanaInicio_LunesDeMexico(t *testing.T) {
	loc := negocios.MexicoCity()
	// Domingo 2026-11-15 23:30 en México = lunes 05:30 UTC; la semana sigue siendo la del lunes 9.
	domingo := time.Date(2026, 11, 15, 23, 30, 0, 0, loc)
	assert.Equal(t, time.Date(2026, 11, 9, 0, 0, 0, 0, loc).UTC(), SemanaInicio(domingo, loc))
	lunes := time.Date(2026, 11, 16, 0, 0, 0, 0, loc)
	assert.Equal(t, lunes.UTC(), SemanaInicio(lunes, loc))
	assert.Equal(t, time.Date(2026, 11, 23, 0, 0, 0, 0, loc).UTC(), ProximoLunes(lunes, loc))
}

func TestConservarHasta_CincoAnios(t *testing.T) {
	t0 := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	assert.Equal(t, time.Date(2031, 11, 14, 10, 0, 0, 0, time.UTC), ConservarHasta(t0))
}

func TestDecodeEstricto_RechazaCamposDeTarjeta(t *testing.T) {
	var req CobrarRequest
	require.NoError(t, DecodeEstricto(nil, &req))
	require.NoError(t, DecodeEstricto([]byte(" {} "), &req))
	assert.Error(t, DecodeEstricto([]byte(`{"numero":"4242424242424242","cvc":"123"}`), &req))
	assert.Error(t, DecodeEstricto([]byte(`{} {}`), &req))
	assert.Error(t, DecodeEstricto([]byte(`[`), &req))
}

func TestEstadoCuenta(t *testing.T) {
	var a accountObj
	assert.Equal(t, negocios.CobrosPendiente, estadoCuenta(a))
	a.PayoutsEnabled, a.Capabilities.Transfers = true, "active"
	assert.Equal(t, negocios.CobrosActivo, estadoCuenta(a))
	a.Requirements.DisabledReason = "requirements.past_due"
	assert.Equal(t, negocios.CobrosRestringido, estadoCuenta(a))
}

func TestVerificarFirma(t *testing.T) {
	assert.ErrorIs(t, VerificarFirma([]byte("{}"), "", "whsec"), ErrFirma)
	assert.ErrorIs(t, VerificarFirma([]byte("{}"), "t=1,v1=00", ""), ErrFirma)
	assert.ErrorIs(t, VerificarFirma([]byte("{}"), "t=1,v1=00", "whsec"), ErrFirma)
}

func TestPago_Abierto(t *testing.T) {
	assert.True(t, Pago{Estado: EstadoRequierePago}.Abierto())
	assert.True(t, Pago{Estado: EstadoProcesando}.Abierto())
	assert.False(t, Pago{Estado: EstadoAprobado}.Abierto())
}
