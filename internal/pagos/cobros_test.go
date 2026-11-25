package pagos

import (
	"encoding/json"
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

func TestCobrosQuery_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	ok := NewCobrosQuery()
	ok.Desde, ok.Hasta, ok.Estado = "2026-11-01", "2026-11-30", EstadoReembolsadoParcial
	require.NoError(t, v.Struct(ok))
	require.NoError(t, v.Struct(NewCobrosQuery()))

	for name, mal := range map[string]func(*CobrosQuery){
		"estado desconocido": func(q *CobrosQuery) { q.Estado = "pagado" },
		"desde con hora":     func(q *CobrosQuery) { q.Desde = "2026-11-01T00:00:00Z" },
		"hasta inválida":     func(q *CobrosQuery) { q.Hasta = "30/11/2026" },
		"limite 0":           func(q *CobrosQuery) { q.Limite = 0 },
		"limite 51":          func(q *CobrosQuery) { q.Limite = 51 },
	} {
		q := NewCobrosQuery()
		mal(&q)
		err := v.Struct(q)
		var ae *apperr.Error
		require.True(t, errors.As(err, &ae), name)
		assert.Equal(t, apperr.KindValidation, ae.Kind, name)
	}
}

func TestRangoCobros_DiasCompletosEnLaZonaDelNegocio(t *testing.T) {
	loc := negocios.MexicoCity()
	d, h, err := RangoCobros("2026-11-01", "2026-11-01", loc)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC), *d, "medianoche de la Ciudad de México")
	assert.Equal(t, time.Date(2026, 11, 2, 6, 0, 0, 0, time.UTC), *h, "hasta incluye el día completo")

	d, h, err = RangoCobros("", "", loc)
	require.NoError(t, err)
	assert.Nil(t, d)
	assert.Nil(t, h)

	_, _, err = RangoCobros("2026-11-10", "2026-11-09", loc)
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae))
	assert.Equal(t, apperr.KindValidation, ae.Kind)
}

func TestTotales_SumanElDesglose(t *testing.T) {
	ps := []Pago{
		{MontoCentavos: 10000, ComisionCentavos: 1000},
		{MontoCentavos: 10000, ComisionCentavos: 1000, Reembolsos: []Reembolso{{MontoCentavos: 5000}}},
	}
	assert.Equal(t, CobrosTotales{Pagos: 2, Anticipo: 20000, Reembolsos: 5000, Comision: 1500, Neto: 13500}, Totales(ps))
	assert.Equal(t, CobrosTotales{}, Totales(nil))
}

func TestFiltroCobros_EstadoYRango(t *testing.T) {
	neg := bson.NewObjectID()
	d := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	f := FiltroCobros{Negocio: neg, Desde: &d}
	assert.Equal(t, bson.D{{Key: "negocio_id", Value: neg}, {Key: "creado_en", Value: bson.D{{Key: "$gte", Value: d}}}}, f.filtro(""))
	assert.Equal(t, bson.D{{Key: "negocio_id", Value: neg}, {Key: "creado_en", Value: bson.D{{Key: "$gte", Value: d}}}, {Key: "estado", Value: EstadoAprobado}}, f.filtro(EstadoAprobado))
}

// La fila no lleva datos de quien pagó ni ids de Stripe.
func TestCobroFila_SinDatosDeQuienPago(t *testing.T) {
	semana := time.Date(2026, 11, 16, 6, 0, 0, 0, time.UTC)
	p := Pago{
		ID: bson.NewObjectID(), ReservaID: bson.NewObjectID(), UsuarioID: bson.NewObjectID(), PaymentIntentID: "pi_secreto",
		MontoCentavos: 15000, ComisionCentavos: 1500, Estado: EstadoAprobado, Tarjeta: &Tarjeta{Marca: "visa", Ultimos4: "4242"},
		Reembolsos: []Reembolso{}, LiquidacionSemana: &semana,
	}
	raw, err := json.Marshal(cobroFila(p, negocios.MexicoCity()))
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.ElementsMatch(t, []string{"pago_id", "reserva_id", "fecha_reserva", "fecha_cobro", "anticipo", "reembolsos", "comision", "neto", "estado", "tarjeta", "liquidacion_semana"}, keysOf(got))
	assert.Equal(t, "2026-11-16", got["liquidacion_semana"])
	assert.Equal(t, float64(13500), got["neto"])
	assert.NotContains(t, string(raw), "pi_secreto")
	assert.NotContains(t, string(raw), p.UsuarioID.Hex())
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
