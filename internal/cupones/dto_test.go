package cupones

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func TestDTO_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	ok := CrearRequest{Titulo: "2x1 en cafés", Tipo: TipoDosPorUno, VigenciaInicio: "2026-11-16T09:00:00-06:00", VigenciaFin: "2026-11-30T23:00:00-06:00"}
	with := func(f func(*CrearRequest)) CrearRequest { r := ok; f(&r); return r }
	lat, lng, fuera := 20.05, -99.34, 91.0
	cases := []struct {
		name string
		dto  any
		ok   bool
	}{
		{"mínima", ok, true},
		{"relámpago sin fechas", CrearRequest{Titulo: "Ya", Tipo: TipoRegalo, Relampago: true}, true},
		{"sin fechas ni relámpago", with(func(r *CrearRequest) { r.VigenciaInicio, r.VigenciaFin = "", "" }), false},
		{"fecha sin zona", with(func(r *CrearRequest) { r.VigenciaFin = "2026-11-30T23:00:00" }), false},
		{"título vacío", with(func(r *CrearRequest) { r.Titulo = "" }), false},
		{"título de 41", with(func(r *CrearRequest) { r.Titulo = strings.Repeat("a", 41) }), false},
		{"tipo raro", with(func(r *CrearRequest) { r.Tipo = "puntos" }), false},
		{"condiciones de 201", with(func(r *CrearRequest) { r.Condiciones = strings.Repeat("a", 201) }), false},
		{"días válidos", with(func(r *CrearRequest) { r.DiasValidos = []int{0, 6} }), true},
		{"día 7", with(func(r *CrearRequest) { r.DiasValidos = []int{7} }), false},
		{"días repetidos", with(func(r *CrearRequest) { r.DiasValidos = []int{1, 1} }), false},
		{"horas", with(func(r *CrearRequest) { r.HorasValidas = &Horas{Desde: "22:00", Hasta: "02:00"} }), true},
		{"hora mala", with(func(r *CrearRequest) { r.HorasValidas = &Horas{Desde: "25:00", Hasta: "02:00"} }), false},
		{"límite negativo", with(func(r *CrearRequest) { r.LimiteUsos = -1 }), false},
		{"editar vacío", EditarRequest{}, true},
		{"editar estado pausado", EditarRequest{Estado: ptr("pausado")}, true},
		{"editar estado agotado", EditarRequest{Estado: ptr("agotado")}, false},
		{"editar límite 0", EditarRequest{LimiteUsos: ptr(0)}, true},
		{"validar", ValidarRequest{Codigo: "ABC234"}, true},
		{"validar vacío", ValidarRequest{}, false},
		{"cerca", CercaQuery{Lat: &lat, Lng: &lng, RadioKm: 5, Limite: 20}, true},
		{"cerca sin lat", CercaQuery{Lng: &lng, RadioKm: 5, Limite: 20}, false},
		{"cerca lat 91", CercaQuery{Lat: &fuera, Lng: &lng, RadioKm: 5, Limite: 20}, false},
		{"cerca radio 21", CercaQuery{Lat: &lat, Lng: &lng, RadioKm: 21, Limite: 20}, false},
		{"cerca radio 0.05", CercaQuery{Lat: &lat, Lng: &lng, RadioKm: 0.05, Limite: 20}, false},
		{"mis filtro", MisQuery{Filtro: "por_vencer", Limite: 20}, true},
		{"mis filtro raro", MisQuery{Filtro: "todos", Limite: 20}, false},
		{"mis q de 61", MisQuery{Filtro: "usados", Q: strings.Repeat("a", 61), Limite: 20}, false},
		{"estadísticas", EstadisticasQuery{Desde: "2026-11-01", Hasta: "2026-11-14"}, true},
		{"estadísticas fecha mala", EstadisticasQuery{Desde: "01/11/2026"}, false},
		{"sugerir", SugerirRequest{Tipo: TipoPorcentaje, Valor: ptr[int64](10), Descripcion: "Café de olla"}, true},
		{"sugerir descripción larga", SugerirRequest{Tipo: TipoRegalo, Descripcion: strings.Repeat("a", 201)}, false},
		{"lista negocio estado raro", ListaNegocioQuery{Estado: "borrado", Limite: 20}, false},
	}
	for _, c := range cases {
		err := v.Struct(c.dto)
		assert.Equal(t, c.ok, err == nil, "%s: %v", c.name, err)
	}
}

func TestVigencia_RelampagoIgnoraFechas(t *testing.T) {
	now := time.Date(2026, 11, 14, 18, 0, 0, 0, time.UTC)
	i, f, err := vigencia(CrearRequest{Relampago: true, VigenciaInicio: "2020-01-01T00:00:00Z", VigenciaFin: "2020-01-02T00:00:00Z"}, now)
	require.NoError(t, err)
	assert.Equal(t, now, i)
	assert.Equal(t, now.Add(3*time.Hour), f)
	_, _, err = vigencia(CrearRequest{VigenciaInicio: "2026-11-20T00:00:00Z", VigenciaFin: "2026-11-19T00:00:00Z"}, now)
	assert.Error(t, err, "inicio después del fin")
	_, _, err = vigencia(CrearRequest{VigenciaInicio: "2026-11-01T00:00:00Z", VigenciaFin: "2026-11-02T00:00:00Z"}, now)
	assert.Error(t, err, "ya terminó")
}

func TestEditar_ReglasEnMemoria(t *testing.T) {
	now := time.Date(2026, 11, 14, 18, 0, 0, 0, time.UTC)
	base := Cupon{Tipo: TipoMonto, Valor: 5000, Estado: EstadoActivo, VigenciaInicio: now.Add(-time.Hour), VigenciaFin: now.Add(48 * time.Hour), UsosActuales: 3}

	e := &editado{c: base}
	require.Error(t, e.contenido(EditarRequest{Tipo: ptr(TipoPorcentaje)}), "porcentaje sin valor")
	e = &editado{c: base}
	require.NoError(t, e.contenido(EditarRequest{Tipo: ptr(TipoRegalo)}), "regalo descarta el monto")
	assert.Equal(t, int64(0), e.c.Valor)
	e = &editado{c: base}
	require.NoError(t, e.contenido(EditarRequest{Titulo: ptr("Nuevo")}))
	assert.Len(t, e.set, 1, "sin tipo ni valor no se tocan")

	e = &editado{c: base}
	assert.Error(t, e.limite(EditarRequest{LimiteUsos: ptr(2)}), "menor que los usos")
	require.NoError(t, e.limite(EditarRequest{LimiteUsos: ptr(3)}))
	require.NoError(t, e.estado(EditarRequest{}, EstadoActivo, now))
	require.NotEmpty(t, e.set)
	last := e.set[len(e.set)-1]
	assert.Equal(t, "estado", last.Key)
	assert.Equal(t, EstadoAgotado, last.Value, "límite igual a los usos = agotado")

	e = &editado{c: base}
	require.NoError(t, e.estado(EditarRequest{Estado: ptr(EstadoPausado)}, EstadoActivo, now))
	assert.Equal(t, EstadoPausado, e.set[0].Value)
	e = &editado{c: Cupon{Estado: EstadoPausado, PausadoPorNegocio: true}}
	require.NoError(t, e.estado(EditarRequest{}, EstadoPausado, now))
	assert.Empty(t, e.set, "sin pedir estado, la pausa del negocio se conserva")
	e = &editado{c: Cupon{Estado: EstadoVencido}}
	assert.Error(t, e.estado(EditarRequest{Estado: ptr(EstadoActivo)}, EstadoVencido, now))
	e = &editado{c: base}
	assert.Error(t, e.fechas(EditarRequest{VigenciaFin: ptr("2026-11-01T00:00:00Z")}, now), "fin pasado")
	require.NoError(t, e.fechas(EditarRequest{VigenciaFin: ptr("2026-12-01T00:00:00Z")}, now))
}
