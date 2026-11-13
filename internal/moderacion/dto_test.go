package moderacion_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func TestDTO_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	bajo, alto := 39.0, 101.0
	muchas := make([]string, moderacion.MaxPalabras+1)
	for i := range muchas {
		muchas[i] = "x"
	}
	casos := []struct {
		nombre string
		dto    any
		ok     bool
	}{
		{"aprobar sin motivo", moderacion.DecisionRequest{Decision: "aprobado"}, true},
		{"rechazar con motivo", moderacion.DecisionRequest{Decision: "rechazado", Motivo: "spam"}, true},
		{"rechazar sin motivo", moderacion.DecisionRequest{Decision: "rechazado"}, false},
		{"decisión desconocida", moderacion.DecisionRequest{Decision: "pendiente"}, false},
		{"motivo largo", moderacion.DecisionRequest{Decision: "rechazado", Motivo: strings.Repeat("a", 501)}, false},
		{"reglas válidas", moderacion.ReglasRequest{Cuarentena: 40, Rechazo: 80, Palabras: []string{"fentanilo"}}, true},
		{"cuarentena bajo 40", moderacion.ReglasRequest{Cuarentena: 39, Rechazo: 80}, false},
		{"palabra vacía", moderacion.ReglasRequest{Cuarentena: 40, Rechazo: 80, Palabras: []string{""}}, false},
		{"demasiadas palabras", moderacion.ReglasRequest{Cuarentena: 40, Rechazo: 80, Palabras: muchas}, false},
		{"patch vacío", moderacion.ReglasPatch{}, true},
		{"patch cuarentena bajo 40", moderacion.ReglasPatch{Cuarentena: &bajo}, false},
		{"patch rechazo sobre 100", moderacion.ReglasPatch{Rechazo: &alto}, false},
		{"lista por defecto", moderacion.NuevaListaQuery(), true},
		{"estado desconocido", moderacion.ListaQuery{Estado: "borrado", Limit: 10}, false},
		{"límite alto", moderacion.ListaQuery{Limit: 51}, false},
	}
	for _, c := range casos {
		err := v.Struct(c.dto)
		if c.ok {
			assert.NoError(t, err, c.nombre)
		} else {
			assert.Error(t, err, c.nombre)
		}
	}
}
