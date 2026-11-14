package cupones

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

func ptr[T any](v T) *T { return &v }

func TestValidarValor_PorTipo(t *testing.T) {
	cases := []struct {
		tipo  string
		valor *int64
		ok    bool
	}{
		{TipoPorcentaje, ptr[int64](1), true},
		{TipoPorcentaje, ptr[int64](100), true},
		{TipoPorcentaje, ptr[int64](0), false},
		{TipoPorcentaje, ptr[int64](101), false},
		{TipoPorcentaje, nil, false},
		{TipoMonto, ptr[int64](5000), true},
		{TipoMonto, ptr[int64](0), false},
		{TipoMonto, nil, false},
		{TipoDosPorUno, nil, true},
		{TipoDosPorUno, ptr[int64](2), false},
		{TipoRegalo, nil, true},
		{TipoRegalo, ptr[int64](1), false},
	}
	for _, c := range cases {
		err := ValidarValor(c.tipo, c.valor)
		assert.Equal(t, c.ok, err == nil, "%s %v", c.tipo, c.valor)
	}
}

func TestValidarVigenciaYHoras(t *testing.T) {
	now := time.Now()
	assert.NoError(t, ValidarVigencia(now, now.Add(time.Hour)))
	assert.Error(t, ValidarVigencia(now, now))
	assert.Error(t, ValidarVigencia(now.Add(time.Hour), now))
	assert.NoError(t, ValidarHoras(nil))
	assert.NoError(t, ValidarHoras(&Horas{Desde: "22:00", Hasta: "02:00"}))
	assert.Error(t, ValidarHoras(&Horas{Desde: "10:00", Hasta: "10:00"}))
}

// mx arma una hora local de México; el 2026-11-16 es lunes (1).
func mx(dia, hora, minuto int) time.Time {
	return time.Date(2026, 11, dia, hora, minuto, 0, 0, negocios.MexicoCity())
}

func TestEnHorario_DiasYHoras(t *testing.T) {
	loc := negocios.MexicoCity()
	assert.True(t, EnHorario(nil, nil, mx(16, 3, 0), loc), "sin reglas = siempre")
	assert.True(t, EnHorario([]int{1}, nil, mx(16, 12, 0), loc), "lunes")
	assert.False(t, EnHorario([]int{2}, nil, mx(16, 12, 0), loc), "solo martes")
	h := &Horas{Desde: "13:00", Hasta: "17:00"}
	assert.True(t, EnHorario(nil, h, mx(16, 13, 0), loc))
	assert.True(t, EnHorario(nil, h, mx(16, 16, 59), loc))
	assert.False(t, EnHorario(nil, h, mx(16, 17, 0), loc), "hasta es exclusivo")
	assert.False(t, EnHorario(nil, h, mx(16, 12, 59), loc))
	// La hora UTC no cuenta: 20:00 UTC son las 14:00 en México.
	assert.True(t, EnHorario(nil, h, time.Date(2026, 11, 16, 20, 0, 0, 0, time.UTC), loc))
}

func TestEnHorario_CruzaMedianoche(t *testing.T) {
	loc := negocios.MexicoCity()
	h := &Horas{Desde: "22:00", Hasta: "02:00"}
	assert.True(t, EnHorario(nil, h, mx(16, 23, 30), loc))
	assert.True(t, EnHorario(nil, h, mx(17, 1, 59), loc))
	assert.False(t, EnHorario(nil, h, mx(17, 2, 0), loc))
	assert.False(t, EnHorario(nil, h, mx(16, 21, 59), loc))
	// Solo lunes (1): la madrugada del martes cuenta como lunes; la del lunes, como domingo.
	assert.True(t, EnHorario([]int{1}, h, mx(17, 1, 0), loc))
	assert.False(t, EnHorario([]int{1}, h, mx(16, 1, 0), loc))
	assert.True(t, EnHorario([]int{1}, h, mx(16, 22, 0), loc))
	assert.False(t, EnHorario([]int{1}, h, mx(17, 22, 0), loc))
}

func TestGenerarCodigo_AlfabetoYLargo(t *testing.T) {
	assert.Len(t, Alfabeto, 32)
	assert.NotContains(t, Alfabeto, "0")
	assert.NotContains(t, Alfabeto, "O")
	assert.NotContains(t, Alfabeto, "1")
	assert.NotContains(t, Alfabeto, "I")
	vistos := map[string]bool{}
	for range 200 {
		c, err := codigoAleatorio()
		require.NoError(t, err)
		require.Len(t, c, LargoCodigo)
		require.True(t, CodigoValido(c), c)
		vistos[c] = true
	}
	assert.Greater(t, len(vistos), 195, "códigos repetidos de más")
	c, err := GenerarCodigo(bytes.NewReader([]byte{0, 31, 32, 255, 1, 30}))
	require.NoError(t, err)
	assert.Equal(t, "2Z2Z3Y", c, "byte & 31: 32 vuelve a 2")
	_, err = GenerarCodigo(bytes.NewReader([]byte{1, 2}))
	assert.Error(t, err, "sin bytes suficientes")
}

func TestCodigoValidoYNormalizar(t *testing.T) {
	assert.Equal(t, "ABC234", NormalizarCodigo("  abc234 "))
	assert.True(t, CodigoValido("ABC234"))
	assert.False(t, CodigoValido("ABC23"))
	assert.False(t, CodigoValido("ABC2340"))
	assert.False(t, CodigoValido("ABCO34"), "O no está en el alfabeto")
	assert.False(t, CodigoValido("abc234"))
}

func TestCoincidePrefijo(t *testing.T) {
	r := ResumenNormalizado("2x1 en Café", "La Fonda Tulense")
	assert.Equal(t, "2x1 en cafe la fonda tulense", r)
	assert.True(t, CoincidePrefijo(r, ""))
	assert.True(t, CoincidePrefijo(r, "2x1"))
	assert.True(t, CoincidePrefijo(r, "CAFÉ"))
	assert.True(t, CoincidePrefijo(r, "tul"))
	assert.True(t, CoincidePrefijo(r, "la fon"))
	assert.False(t, CoincidePrefijo(r, "ulense"), "solo prefijos de palabra")
	assert.False(t, CoincidePrefijo(r, "pizza"))
}

func TestFiltroCartera(t *testing.T) {
	now := time.Now().UTC()
	v := filtroCartera("vigentes", now)
	assert.Equal(t, []string{CarteraGuardado, CarteraEnCanje}, v.Estados)
	assert.Nil(t, v.Hasta)
	p := filtroCartera("por_vencer", now)
	require.NotNil(t, p.Hasta)
	assert.Equal(t, 24*time.Hour, p.Hasta.Sub(now))
	u := filtroCartera("usados", now)
	assert.Equal(t, []string{CarteraCanjeado}, u.Estados)
	assert.True(t, u.Desc)
	assert.Equal(t, "canjeado_en", u.Campo)
}

func TestCupon_VigenteYAgotado(t *testing.T) {
	now := time.Now()
	c := Cupon{Estado: EstadoActivo, VigenciaInicio: now.Add(-time.Hour), VigenciaFin: now.Add(time.Hour)}
	assert.True(t, c.Vigente(now))
	c.Estado = EstadoProgramado
	assert.True(t, c.Vigente(now), "programado ya iniciado se muestra aunque el trabajo se atrase")
	c.Estado = EstadoPausado
	assert.False(t, c.Vigente(now))
	c.Estado = EstadoActivo
	assert.False(t, c.Vigente(now.Add(2*time.Hour)), "fin exclusivo")
	assert.False(t, c.Agotado())
	c.LimiteUsos, c.UsosActuales = 2, 2
	assert.True(t, c.Agotado())
	c.LimiteUsos = 0
	assert.False(t, c.Agotado(), "0 = sin límite")
}

func TestPeriodoEstadisticas(t *testing.T) {
	now := time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)
	d, h, err := Periodo(EstadisticasQuery{}, now)
	require.NoError(t, err)
	assert.Equal(t, "2026-10-16", Dia(d))
	assert.Equal(t, "2026-11-14", Dia(h))
	_, _, err = Periodo(EstadisticasQuery{Desde: "2026-11-10", Hasta: "2026-11-01"}, now)
	assert.Error(t, err)
	_, _, err = Periodo(EstadisticasQuery{Desde: "2026-01-01", Hasta: "2026-11-01"}, now)
	assert.Error(t, err, "más de 90 días")
	_, _, err = Periodo(EstadisticasQuery{Desde: "2026-08-17", Hasta: "2026-11-14"}, now)
	assert.NoError(t, err, "90 días justos")
}

func TestTituloIA_Validar(t *testing.T) {
	ok := tituloIA{Titulo: "  2x1 en cafés de olla  "}
	require.NoError(t, ok.Validar())
	assert.Equal(t, "2x1 en cafés de olla", ok.Titulo)
	assert.Error(t, (&tituloIA{Titulo: ""}).Validar())
	assert.Error(t, (&tituloIA{Titulo: strings.Repeat("á", 41)}).Validar())
	assert.Error(t, (&tituloIA{Titulo: "Escribe a ana@correo.mx"}).Validar())
	assert.Error(t, (&tituloIA{Titulo: "Llama al 7731234567"}).Validar())
}

func TestContextoTitulo(t *testing.T) {
	assert.Equal(t, "Categoría: cafe. Tipo de cupón: monto. Valor: 50.00 pesos.", contextoTitulo("cafe", SugerirRequest{Tipo: TipoMonto, Valor: ptr[int64](5000)}))
	assert.Equal(t, "Categoría: bar. Tipo de cupón: porcentaje. Valor: 15 %.", contextoTitulo("bar", SugerirRequest{Tipo: TipoPorcentaje, Valor: ptr[int64](15)}))
	assert.Contains(t, contextoTitulo("bar", SugerirRequest{Tipo: TipoRegalo}), "sin valor")
}
