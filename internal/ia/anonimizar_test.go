package ia_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
)

func TestAnonimizar_QuitaDatosPersonales(t *testing.T) {
	casos := map[string]string{
		"correo":     "escríbeme a ana.lopez@example.com",
		"teléfono":   "llámame al +52 771 123 4567",
		"teléfono 2": "mi cel 7711234567",
		"enlace":     "mira https://example.com/perfil?id=9",
		"usuario":    "voy con @ana_lopez",
		"coordenada": "estoy en 20.05123, -99.34112",
	}
	secretos := map[string]string{
		"correo": "ana.lopez", "teléfono": "123 4567", "teléfono 2": "7711234567",
		"enlace": "example.com", "usuario": "ana_lopez", "coordenada": "20.05123",
	}
	for nombre, texto := range casos {
		t.Run(nombre, func(t *testing.T) {
			assert.NotContains(t, ia.Anonimizar(texto), secretos[nombre])
		})
	}
}

func TestAnonimizar_ConservaLoQueSirveAlPlan(t *testing.T) {
	texto := "cena con mi pareja a las 20:30, máximo 800 pesos, algo tranquilo en Tula"
	assert.Equal(t, texto, ia.Anonimizar(texto))
}
