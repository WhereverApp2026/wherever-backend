package ia_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
)

type eleccion struct {
	IDs        []string `json:"ids"`
	candidatos map[string]bool
}

func (e *eleccion) Validar() error {
	if len(e.IDs) == 0 {
		return fmt.Errorf("faltan ids")
	}
	for _, id := range e.IDs {
		if !e.candidatos[id] {
			return fmt.Errorf("id fuera de la lista")
		}
	}
	return nil
}

func nuevaEleccion() *eleccion {
	return &eleccion{candidatos: map[string]bool{"a1": true, "b2": true}}
}

func TestEsquema_SalidaValida(t *testing.T) {
	e := nuevaEleccion()
	require.NoError(t, ia.Decodificar([]byte(`{"ids":["a1","b2"]}`), e))
	assert.Equal(t, []string{"a1", "b2"}, e.IDs)
}

func TestEsquema_Rechazos(t *testing.T) {
	casos := map[string]string{
		"campo faltante":     `{}`,
		"id fuera de lista":  `{"ids":["zz"]}`,
		"texto extra":        `{"ids":["a1"]} y algo más`,
		"JSON truncado":      `{"ids":["a1"`,
		"campo no declarado": `{"ids":["a1"],"nota":"x"}`,
	}
	for nombre, salida := range casos {
		t.Run(nombre, func(t *testing.T) {
			assert.ErrorIs(t, ia.Decodificar([]byte(salida), nuevaEleccion()), ia.ErrRespuestaInvalida)
		})
	}
}
