package panel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// Los datos de nv-90 esperan revisión como nombre y dirección; vacíos quitan WhatsApp y redes.
func TestPendientes_DatosDelNegocio(t *testing.T) {
	now := time.Date(2026, 11, 23, 18, 0, 0, 0, time.UTC)
	b := negocios.Business{Name: "Café"}
	c, fields := pendientes(b, PerfilRequest{Experiencias: &[]string{"cena", "romantico"}, AnioApertura: ptr(1998),
		WhatsApp: ptr("+52 773 123 4567"), Redes: &negocios.Redes{Instagram: " cafe "}}, now)
	require.NotNil(t, c)
	assert.Empty(t, fields)
	assert.Equal(t, []string{CampoExperiencias, CampoAnioApertura, CampoWhatsApp, CampoRedes}, c.Campos)
	assert.Equal(t, "+527731234567", *c.WhatsApp)
	assert.Equal(t, "cafe", c.Redes.Instagram)
	assert.Equal(t, 1998, c.AnioApertura)

	c, _ = pendientes(b, PerfilRequest{WhatsApp: ptr("")}, now)
	require.NotNil(t, c)
	assert.Empty(t, *c.WhatsApp, "vacío quita el WhatsApp al aprobarse")

	_, fields = pendientes(b, PerfilRequest{Experiencias: &[]string{"brunch"}, AnioApertura: ptr(2030), WhatsApp: ptr("7731234567"),
		Redes: &negocios.Redes{SitioWeb: "javascript:alert(1)"}}, now)
	campos := []string{}
	for _, f := range fields {
		campos = append(campos, f.Field)
	}
	assert.ElementsMatch(t, []string{"experiencias", "anio_apertura", "whatsapp", "redes.sitio_web"}, campos)
}

func TestPerfilResponse_DatosYCambios(t *testing.T) {
	wa := "+527731234567"
	b := negocios.Business{Experiencias: []string{"cena"}, WhatsApp: wa,
		CambiosPendientes: &negocios.CambiosPendientes{Campos: []string{CampoWhatsApp}, WhatsApp: &wa}}
	out := perfilResponse(b)
	assert.Equal(t, []string{"cena"}, out.Experiencias)
	assert.Equal(t, wa, out.WhatsApp)
	require.NotNil(t, out.CambiosPendientes)
	assert.Equal(t, &wa, out.CambiosPendientes.WhatsApp)
}
