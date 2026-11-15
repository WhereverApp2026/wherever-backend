package reportesnegocio

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

type moderadorFijo moderacion.Veredicto

func (m moderadorFijo) Evaluar(context.Context, moderacion.Contenido) moderacion.Resultado {
	return moderacion.Resultado{Veredicto: moderacion.Veredicto(m)}
}

func TestDTO_MotivoDetalleYEstado(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	assert.NoError(t, v.Struct(CrearRequest{Motivo: MotivoHorario}))
	assert.Error(t, v.Struct(CrearRequest{Motivo: "inventado"}))
	long := make([]byte, MaxDetalle+1)
	for i := range long {
		long[i] = 'a'
	}
	assert.Error(t, v.Struct(CrearRequest{Motivo: MotivoOtro, Detalle: string(long)}))
	assert.NoError(t, v.Struct(ResolverRequest{Estado: EstadoCorregido}))
	assert.Error(t, v.Struct(ResolverRequest{Estado: EstadoAbierto}), "no se reabre por PATCH")
	assert.Error(t, v.Struct(ColaQuery{Limit: 101}))
}

func TestRevisarDetalle_OtroYModeracion(t *testing.T) {
	s := NewService(nil, nil, nil, nil)
	assert.Error(t, s.revisarDetalle(context.Background(), MotivoOtro, ""), "otro sin detalle")
	assert.NoError(t, s.revisarDetalle(context.Background(), MotivoHorario, ""))
	s.WithModeracion(moderadorFijo(moderacion.VeredictoRechazado))
	assert.Error(t, s.revisarDetalle(context.Background(), MotivoPrecio, "texto ofensivo"))
	s.WithModeracion(moderadorFijo(moderacion.VeredictoCuarentena))
	assert.NoError(t, s.revisarDetalle(context.Background(), MotivoPrecio, "subió el precio"), "dudoso se guarda: lo revisa el administrador")
}
