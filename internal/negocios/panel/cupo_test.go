package panel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

var horarioCupo = []negocios.Horario{{Dia: 1, Abre: "13:30", Cierra: "16:00"}, {Dia: 5, Abre: "22:00", Cierra: "01:00"}, {Dia: 3, Abre: "09:00", Cierra: "18:00", Cerrado: true}}

func cupoReq(dia int, hora string, lugares int) CupoHoraRequest {
	return CupoHoraRequest{Dia: ptr(dia), Hora: hora, Lugares: ptr(lugares)}
}

func campoInvalido(t *testing.T, err error, campo string) {
	t.Helper()
	var ae *apperr.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperr.KindValidation, ae.Kind)
	require.NotEmpty(t, ae.Fields)
	assert.Equal(t, campo, ae.Fields[0].Field)
}

func TestValidarCupo_DentroDelHorarioSinRepetirYOrdenado(t *testing.T) {
	got, err := validarCupo([]CupoHoraRequest{cupoReq(6, "00:00", 4), cupoReq(1, "15:00", 0), cupoReq(1, "13:00", 8)}, horarioCupo, 0)
	require.NoError(t, err)
	assert.Equal(t, []negocios.CupoHora{{Dia: 1, Hora: "13:00", Lugares: 8}, {Dia: 1, Hora: "15:00", Lugares: 0}, {Dia: 6, Hora: "00:00", Lugares: 4}}, got,
		"13:00 se cruza con la apertura de 13:30; la madrugada del viernes cae en sábado")

	vacio, err := validarCupo([]CupoHoraRequest{}, horarioCupo, 0)
	require.NoError(t, err)
	assert.Empty(t, vacio, "lista vacía = todo con la capacidad general")
}

func TestValidarCupo_Rechazos(t *testing.T) {
	_, err := validarCupo([]CupoHoraRequest{cupoReq(1, "16:00", 5)}, horarioCupo, 0)
	campoInvalido(t, err, "cupo_por_hora[0].hora")
	_, err = validarCupo([]CupoHoraRequest{cupoReq(3, "10:00", 5)}, horarioCupo, 0)
	campoInvalido(t, err, "cupo_por_hora[0].hora")
	_, err = validarCupo([]CupoHoraRequest{cupoReq(1, "14:00", 5), cupoReq(1, "14:00", 6)}, horarioCupo, 0)
	campoInvalido(t, err, "cupo_por_hora[1].hora")
	for _, mala := range []string{"14:30", "1400", "24:00", "4:00"} {
		_, err = validarCupo([]CupoHoraRequest{cupoReq(1, mala, 5)}, horarioCupo, 0)
		campoInvalido(t, err, "cupo_por_hora[0].hora")
	}
	_, err = validarCupo([]CupoHoraRequest{cupoReq(1, "14:00", 31)}, horarioCupo, 30)
	campoInvalido(t, err, "cupo_por_hora[0].lugares")
}

func TestCupoRequest_Validador(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	assert.NoError(t, v.Struct(CupoRequest{CupoPorHora: []CupoHoraRequest{cupoReq(0, "12:00", 0)}}))
	assert.Error(t, v.Struct(CupoRequest{}), "sin lista")
	assert.Error(t, v.Struct(CupoRequest{CupoPorHora: []CupoHoraRequest{cupoReq(7, "12:00", 1)}}), "día 7")
	assert.Error(t, v.Struct(CupoRequest{CupoPorHora: []CupoHoraRequest{cupoReq(1, "12:00", 501)}}), "más de 500")
	assert.Error(t, v.Struct(CupoRequest{CupoPorHora: []CupoHoraRequest{{Dia: ptr(1), Hora: "12:00"}}}), "sin lugares")
	assert.Error(t, v.Struct(CupoRequest{CupoPorHora: make([]CupoHoraRequest, 169)}), "más de 7 × 24")
}

func TestTablaCupo_HorasAbiertasCupoYAfluencia(t *testing.T) {
	traffic := make([][]int, 7)
	for d := range traffic {
		traffic[d] = make([]int, 24)
	}
	traffic[1][14] = 70
	b := negocios.Business{Schedules: horarioCupo, Traffic: traffic, Version: 4,
		ReservasConfig: &negocios.ReservasConfig{CapacidadPorHorario: 12, CupoPorHora: []negocios.CupoHora{{Dia: 1, Hora: "14:00", Lugares: 5}}}}
	got := tablaCupo(b)
	assert.Equal(t, 12, got.CapacidadPorHorario)
	assert.Equal(t, 4, got.Version)
	require.Len(t, got.Dias, 7)
	lunes := got.Dias[1].Horas
	require.Len(t, lunes, 3, "13, 14 y 15")
	assert.Equal(t, 12, lunes[0].Lugares)
	assert.Equal(t, 5, lunes[1].Lugares)
	require.NotNil(t, lunes[1].Afluencia)
	assert.Equal(t, negocios.CrowdPeak, *lunes[1].Afluencia)
	assert.Equal(t, negocios.CrowdLow, *lunes[0].Afluencia)
	assert.Empty(t, got.Dias[3].Horas, "miércoles cerrado")
	assert.Len(t, got.Dias[5].Horas, 2, "22 y 23 del viernes")
	assert.Len(t, got.Dias[6].Horas, 1, "00 del sábado")
	assert.Nil(t, got.Dias[6].Horas[0].Afluencia, "sábado sin curva")

	sinCfg := tablaCupo(negocios.Business{Schedules: horarioCupo})
	assert.Equal(t, 0, sinCfg.Dias[1].Horas[0].Lugares)
}

func TestReservasConfig_MesasMaxPersonasYCupoConservado(t *testing.T) {
	previa := &negocios.ReservasConfig{Mesas: 8, MaxPersonas: 50, CupoPorHora: []negocios.CupoHora{{Dia: 1, Hora: "14:00", Lugares: 30}}}
	r := reservasValida()
	cfg, err := reservasConfig(r, nil, previa)
	require.NoError(t, err)
	assert.Equal(t, 8, cfg.Mesas, "sin mesas se queda lo guardado")
	assert.Equal(t, 50, cfg.MaxPersonas)
	assert.Equal(t, previa.CupoPorHora, cfg.CupoPorHora, "el cupo por hora no se pierde")

	r.Mesas, r.MaxPersonas = ptr(10), ptr(60)
	cfg, err = reservasConfig(r, nil, previa)
	require.NoError(t, err)
	assert.Equal(t, 10, cfg.Mesas)
	assert.Equal(t, 60, cfg.MaxPersonas)

	r.MaxPersonas = ptr(20)
	_, err = reservasConfig(r, nil, previa)
	campoInvalido(t, err, "capacidad_por_horario")
	r.CapacidadPorHorario = 20
	_, err = reservasConfig(r, nil, previa)
	campoInvalido(t, err, "max_personas")
}
