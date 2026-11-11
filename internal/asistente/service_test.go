package asistente

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func float(v float64) *float64 { return &v }

func valido() GenerarRequest {
	return GenerarRequest{Frase: "tacos con la banda", Lat: float(19.43), Lng: float(-99.13)}
}

func TestGenerarRequest_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	require.NoError(t, v.Struct(valido()))

	casos := map[string]func(r *GenerarRequest){
		"frase vacía":        func(r *GenerarRequest) { r.Frase = "   "; normalizarFrase(r) },
		"frase de 301":       func(r *GenerarRequest) { r.Frase = strings.Repeat("ñ", MaxFrase+1) },
		"sin lat":            func(r *GenerarRequest) { r.Lat = nil },
		"lat fuera":          func(r *GenerarRequest) { r.Lat = float(91) },
		"lng fuera":          func(r *GenerarRequest) { r.Lng = float(-181) },
		"personas 0 ok":      nil,
		"personas 21":        func(r *GenerarRequest) { r.Personas = 21 },
		"movilidad inválida": func(r *GenerarRequest) { r.Movilidad = "bici" },
	}
	for nombre, mutar := range casos {
		t.Run(nombre, func(t *testing.T) {
			r := valido()
			if mutar == nil {
				assert.NoError(t, v.Struct(r))
				return
			}
			mutar(&r)
			var ae *apperr.Error
			require.ErrorAs(t, v.Struct(r), &ae)
			assert.Equal(t, http.StatusUnprocessableEntity, ae.Kind.Status())
		})
	}
	r := valido()
	r.Frase = strings.Repeat("ñ", MaxFrase)
	assert.NoError(t, v.Struct(r), "300 caracteres (no bytes) caben")
}

func TestFechaPedidaEInicio(t *testing.T) {
	f, err := fechaPedida(nil, sabadoTarde)
	require.NoError(t, err)
	assert.True(t, f.IsZero())
	manana := sabadoTarde.Add(24 * time.Hour)
	f, err = fechaPedida(&manana, sabadoTarde)
	require.NoError(t, err)
	assert.True(t, manana.Equal(f))
	for _, mala := range []time.Time{sabadoTarde.Add(-time.Hour), sabadoTarde.Add(31 * 24 * time.Hour)} {
		_, err = fechaPedida(&mala, sabadoTarde)
		var ae *apperr.Error
		require.ErrorAs(t, err, &ae)
		assert.Equal(t, http.StatusUnprocessableEntity, ae.Kind.Status())
	}

	assert.Equal(t, time.Date(2026, 11, 14, 20, 0, 0, 0, zona), inicioDe("20:00", sabadoTarde))
	assert.Equal(t, time.Date(2026, 11, 15, 10, 0, 0, 0, zona), inicioDe("10:00", sabadoTarde), "si ya pasó, mañana")
	assert.Equal(t, time.Date(2026, 11, 14, 14, 35, 0, 0, zona), inicioDe("", sabadoTarde.Add(2*time.Minute)))
}

func TestPersonasYMovilidad(t *testing.T) {
	assert.Equal(t, 4, personasDe(4, Interpretacion{Compania: "pareja"}))
	assert.Equal(t, 2, personasDe(0, Interpretacion{Compania: "pareja"}))
	assert.Equal(t, 1, personasDe(0, Interpretacion{Compania: "amigos"}))
}
