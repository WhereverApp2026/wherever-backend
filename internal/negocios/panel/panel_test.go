package panel

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/alta"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func ptr[T any](v T) *T { return &v }

func reservasValida() ReservasConfigRequest {
	return ReservasConfigRequest{AceptaReservas: ptr(true), CapacidadPorHorario: 40, DuracionTurnoMin: 90,
		Anticipo: AnticipoRequest{Tipo: "fijo"}, PoliticaCancelacion: "moderada", Confirmacion: "automatica", PermiteCambios: ptr(true)}
}

func TestDTOs_Validador(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	assert.NoError(t, v.Struct(reservasValida()))

	malas := map[string]func(r *ReservasConfigRequest){
		"capacidad 0":     func(r *ReservasConfigRequest) { r.CapacidadPorHorario = 0 },
		"capacidad 501":   func(r *ReservasConfigRequest) { r.CapacidadPorHorario = 501 },
		"turno 10":        func(r *ReservasConfigRequest) { r.DuracionTurnoMin = 10 },
		"turno 300":       func(r *ReservasConfigRequest) { r.DuracionTurnoMin = 300 },
		"anticipo raro":   func(r *ReservasConfigRequest) { r.Anticipo.Tipo = "gratis" },
		"política rara":   func(r *ReservasConfigRequest) { r.PoliticaCancelacion = "nunca" },
		"sin confirmar":   func(r *ReservasConfigRequest) { r.Confirmacion = "" },
		"sin acepta":      func(r *ReservasConfigRequest) { r.AceptaReservas = nil },
		"anticipo < 0":    func(r *ReservasConfigRequest) { r.Anticipo.Valor = -1 },
		"permite sin val": func(r *ReservasConfigRequest) { r.PermiteCambios = nil },
	}
	for nombre, cambio := range malas {
		r := reservasValida()
		cambio(&r)
		assert.Error(t, v.Struct(r), nombre)
	}

	assert.Error(t, v.Struct(SelloRequest{Otorgar: ptr(true), Motivo: "corto"}), "motivo de 10 a 500")
	assert.Error(t, v.Struct(SelloRequest{Motivo: "Motivo suficientemente largo"}), "otorgar obligatorio")
	assert.NoError(t, v.Struct(SelloRequest{Otorgar: ptr(false), Motivo: "Motivo suficientemente largo"}))
	assert.Error(t, v.Struct(PausaRequest{}), "pausado obligatorio")
	assert.Error(t, v.Struct(PausaRequest{Pausado: ptr(true), Motivo: strings.Repeat("a", 121)}))
	assert.Error(t, v.Struct(PerfilRequest{Nombre: ptr("ab")}))
	assert.Error(t, v.Struct(PerfilRequest{Ubicacion: &alta.Ubicacion{Type: "Point", Coordinates: []float64{1}}}))
	assert.NoError(t, v.Struct(PerfilRequest{Descripcion: ptr("Nueva descripción")}))
	assert.Error(t, v.Struct(BajaRequest{}))
}

func TestReservasConfig_Combinaciones(t *testing.T) {
	r := reservasValida()
	cfg, err := reservasConfig(r, nil)
	require.NoError(t, err)
	assert.Equal(t, negocios.ReservasConfig{AceptaReservas: true, CapacidadPorHorario: 40, DuracionTurnoMin: 90,
		Anticipo: negocios.Anticipo{Tipo: "fijo"}, PoliticaCancelacion: "moderada", Confirmacion: "automatica", PermiteCambios: true}, cfg)

	r.DuracionTurnoMin = 50
	_, err = reservasConfig(r, nil)
	assert.Error(t, err, "múltiplo de 15")

	r = reservasValida()
	r.Anticipo = AnticipoRequest{Tipo: "porcentaje", Valor: 101}
	_, err = reservasConfig(r, &negocios.Cobros{Estado: negocios.CobrosActivo})
	assert.Error(t, err, "porcentaje hasta 100")

	r.Anticipo.Valor = 20
	_, err = reservasConfig(r, nil)
	var ae *apperr.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, "cobros_no_activos", ae.Message)
	_, err = reservasConfig(r, &negocios.Cobros{Estado: "pendiente"})
	assert.Error(t, err)
	_, err = reservasConfig(r, &negocios.Cobros{Estado: negocios.CobrosActivo})
	assert.NoError(t, err)
}

func TestAvance_ReglaDelSello(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	hace := func(d int) *time.Time { at := now.AddDate(0, 0, -d); return &at }
	buenas := &negocios.ResumenResenas{Verificadas90: 20, Promedio90: 4.5}

	a := Avance(hace(60), now, buenas)
	assert.True(t, a.Elegible)
	require.Len(t, a.Criterios, 4)
	assert.Equal(t, CriterioDias, a.Criterios[0].Nombre)
	assert.InDelta(t, 60, *a.Criterios[0].Actual, 0)

	assert.False(t, Avance(hace(59), now, buenas).Elegible, "menos de 60 días")
	assert.False(t, Avance(hace(90), now, &negocios.ResumenResenas{Verificadas90: 19, Promedio90: 5}).Elegible, "pocas reseñas")
	assert.False(t, Avance(hace(90), now, &negocios.ResumenResenas{Verificadas90: 30, Promedio90: 4.4}).Elegible, "promedio bajo")
	assert.False(t, Avance(hace(90), now, &negocios.ResumenResenas{Verificadas90: 30, Promedio90: 5, RechazadasModeracion90: 1}).Elegible, "rechazo de moderación")
	assert.False(t, Avance(nil, now, buenas).Elegible, "no aprobado")

	sinFuente := Avance(hace(90), now, nil)
	assert.False(t, sinFuente.Elegible, "sin reseñas no es elegible")
	assert.Nil(t, sinFuente.Criterios[1].Actual, "la app muestra «—»")
}

func TestConfirmaBaja_NombreExacto(t *testing.T) {
	assert.True(t, ConfirmaBaja("Fonda Doña Mary", "Fonda Doña Mary"))
	assert.True(t, ConfirmaBaja("Fonda Doña Mary", "  Fonda Doña Mary "))
	assert.False(t, ConfirmaBaja("Fonda Doña Mary", "fonda doña mary"))
	assert.False(t, ConfirmaBaja("Fonda Doña Mary", "Fonda Dona Mary"))
}

func TestContenidoQR_LlevaElCheckin(t *testing.T) {
	c := ContenidoQR("abc123")
	assert.Equal(t, "wherever://checkin/abc123", c)
	assert.Equal(t, "abc123", strings.TrimPrefix(c, esquemaQR))
}

func TestPendientes_SoloNombreDireccionUbicacion(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	b := negocios.Business{Name: "Antes"}
	c, fields := pendientes(b, PerfilRequest{Descripcion: ptr("x")}, now)
	assert.Nil(t, c)
	assert.Empty(t, fields)

	c, _ = pendientes(b, PerfilRequest{Nombre: ptr("Antes")}, now)
	assert.Nil(t, c, "el mismo nombre no espera revisión")

	c, fields = pendientes(b, PerfilRequest{Nombre: ptr(" Después ")}, now)
	require.NotNil(t, c)
	assert.Empty(t, fields)
	assert.Equal(t, "Después", c.Nombre)
	assert.Equal(t, []string{"nombre"}, c.Campos)

	b.CambiosPendientes = c
	c, fields = pendientes(b, PerfilRequest{Ubicacion: &alta.Ubicacion{Type: "Point", Coordinates: []float64{0, 0}}}, now)
	require.NotNil(t, c)
	assert.Equal(t, []string{"nombre", "ubicacion"}, c.Campos, "se suma a lo que ya esperaba")
	assert.NotEmpty(t, fields, "fuera de México")
}

func TestPerfilSet_RecalculaHorarioMin(t *testing.T) {
	h := []negocios.Horario{{Dia: 1, Abre: "09:00", Cierra: "18:00"}}
	set, fields := perfilSet(negocios.Business{PriceLevel: 2, AveragePrice: 150}, PerfilRequest{Horarios: &h, NivelPrecio: ptr(3), Telefono: ptr("abc")})
	assert.Equal(t, []string{"telefono"}, func() []string {
		out := []string{}
		for _, f := range fields {
			out = append(out, f.Field)
		}
		return out
	}())
	got := map[string]any{}
	for _, e := range set {
		got[e.Key] = e.Value
	}
	assert.Equal(t, negocios.DeriveScheduleMin(h), got["horario_min"])
	assert.Equal(t, 3, got["nivel_precio"])
	assert.Equal(t, 150, got["precio_promedio"], "conserva el precio promedio actual")
}

func TestConsultar_PlazoYError(t *testing.T) {
	ctx := context.Background()
	v := consultar(ctx, time.Second, func(context.Context) (int, error) { return 7, nil })
	require.NotNil(t, v)
	assert.Equal(t, 7, *v)
	assert.Nil(t, consultar(ctx, time.Second, func(context.Context) (int, error) { return 0, errors.New("caída") }))
	assert.Nil(t, consultar(ctx, 20*time.Millisecond, func(c context.Context) (int, error) {
		<-c.Done()
		return 1, nil
	}), "la fuente lenta deja null")
}

func TestHoy_ZonaDeLosNegocios(t *testing.T) {
	s := NewService(nil, nil, nil, nil, nil)
	// 2026-11-13 03:00 UTC es el 12 a las 21:00 en Ciudad de México.
	p := s.hoy(time.Date(2026, 11, 13, 3, 0, 0, 0, time.UTC))
	assert.Equal(t, time.Date(2026, 11, 12, 6, 0, 0, 0, time.UTC), p.Desde)
	assert.Equal(t, 24*time.Hour, p.Hasta.Sub(p.Desde))
}
