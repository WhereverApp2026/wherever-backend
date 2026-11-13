package moderacion_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
)

var silencio = slog.New(slog.DiscardHandler)

// reglasQueFallan simula una fuente de reglas caída (nv-43 las leerá de Mongo).
type reglasQueFallan struct{}

func (reglasQueFallan) Reglas(context.Context) (moderacion.Reglas, error) {
	return moderacion.Reglas{}, errors.New("sin base")
}

func TestUmbrales_Bordes(t *testing.T) {
	r := moderacion.ReglasPorDefecto()
	casos := []struct {
		puntaje float64
		want    moderacion.Veredicto
	}{
		{0, moderacion.VeredictoAprobado},
		{39.99, moderacion.VeredictoAprobado},
		{40, moderacion.VeredictoCuarentena},
		{79.99, moderacion.VeredictoCuarentena},
		{80, moderacion.VeredictoRechazado},
		{100, moderacion.VeredictoRechazado},
	}
	for _, c := range casos {
		require.Equal(t, c.want, r.Clasificar(c.puntaje), "puntaje %v", c.puntaje)
	}
}

func TestUmbrales_ReglasInvalidas(t *testing.T) {
	for _, r := range []moderacion.Reglas{
		{Cuarentena: 0, Rechazo: 80},
		{Cuarentena: 80, Rechazo: 80},
		{Cuarentena: 40, Rechazo: 101},
	} {
		require.ErrorIs(t, r.Validar(), moderacion.ErrReglasInvalid)
	}
	require.NoError(t, moderacion.ReglasPorDefecto().Validar())
}

func TestUmbrales_ReglasInvalidasDejanEnCuarentena(t *testing.T) {
	texto := iaLimpio(t)
	m := moderacion.New(nil, moderacion.NewTextoIA(texto), moderacion.Fijas{Cuarentena: 90, Rechazo: 50}, silencio)
	res := m.Evaluar(context.Background(), moderacion.Contenido{Texto: "Muy buen café"})
	require.Equal(t, moderacion.VeredictoCuarentena, res.Veredicto)
	require.Equal(t, moderacion.FallaReglas, res.Falla)
	require.Empty(t, texto.Peticiones(), "sin reglas válidas no se gasta una llamada")
}

func TestUmbrales_FuenteCaidaDejaEnCuarentena(t *testing.T) {
	m := moderacion.New(nil, nil, reglasQueFallan{}, silencio)
	res := m.Evaluar(context.Background(), moderacion.Contenido{Texto: "Hola"})
	require.Equal(t, moderacion.VeredictoCuarentena, res.Veredicto)
	require.False(t, res.Publicable())
}

func TestUmbrales_ContenidoVacio(t *testing.T) {
	m := moderacion.New(nil, nil, moderacion.Fijas(moderacion.ReglasPorDefecto()), silencio)
	res := m.Evaluar(context.Background(), moderacion.Contenido{})
	require.Equal(t, moderacion.VeredictoCuarentena, res.Veredicto)
	require.Equal(t, moderacion.FallaVacio, res.Falla)
}
