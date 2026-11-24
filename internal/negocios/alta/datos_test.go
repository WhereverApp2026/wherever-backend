package alta

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

func TestDatosCrear(t *testing.T) {
	now := time.Date(2026, 11, 23, 18, 0, 0, 0, time.UTC)
	var b negocios.Business
	anio := 2001
	assert.Empty(t, datosCrear(&b, CrearRequest{Experiencias: []string{"desayuno"}, AnioApertura: &anio}, now))
	assert.Equal(t, []string{"desayuno"}, b.Experiencias)
	assert.Equal(t, 2001, b.AnioApertura)
	assert.Empty(t, datosCrear(&negocios.Business{}, CrearRequest{}, now), "son opcionales")
	futuro := 2027
	assert.Len(t, datosCrear(&negocios.Business{}, CrearRequest{Experiencias: []string{}, AnioApertura: &futuro}, now), 2)
}

func TestPaso2_WhatsAppYRedes(t *testing.T) {
	wa, redes := " +52 773-123-4567 ", negocios.Redes{SitioWeb: " https://cafe.mx "}
	p := Paso{Numero: 2, Dos: &Paso2Request{WhatsApp: &wa, Redes: &redes,
		Ubicacion: Ubicacion{Type: "Point", Coordinates: []float64{-99.34, 20.05}}}}
	set, _, ok := p.datos()
	require.True(t, ok)
	m := map[string]any{}
	for _, e := range set {
		m[e.Key] = e.Value
	}
	assert.Equal(t, "+527731234567", m["whatsapp"])
	assert.Equal(t, negocios.Redes{SitioWeb: "https://cafe.mx"}, m["redes"])

	mal := "7731234567"
	_, fields := contactoPaso2(&Paso2Request{WhatsApp: &mal, Redes: &negocios.Redes{Instagram: "@cafe"}})
	assert.Len(t, fields, 2)
	set, fields = contactoPaso2(&Paso2Request{})
	assert.Empty(t, set, "sin WhatsApp ni redes no se tocan")
	assert.Empty(t, fields)
}

func TestSetDatosCambios(t *testing.T) {
	wa := ""
	set := setDatosCambios(bson.D{}, &negocios.CambiosPendientes{Experiencias: []string{"paseo"}, AnioApertura: 1990, WhatsApp: &wa,
		Redes: &negocios.Redes{Instagram: "cafe"}})
	keys := []string{}
	for _, e := range set {
		keys = append(keys, e.Key)
	}
	assert.Equal(t, []string{"experiencias", "anio_apertura", "whatsapp", "redes"}, keys)
	assert.Empty(t, setDatosCambios(bson.D{}, &negocios.CambiosPendientes{}))
}

func TestColaItem_MuestraDatosYCambios(t *testing.T) {
	wa := "+527731234567"
	b := negocios.Business{ID: bson.NewObjectID(), AnioApertura: 1990, Redes: &negocios.Redes{},
		CambiosPendientes: &negocios.CambiosPendientes{Campos: []string{"whatsapp"}, WhatsApp: &wa, EnviadoEn: time.Now()}}
	it := colaItem(b, time.Now())
	assert.Equal(t, 1990, it.AnioApertura)
	assert.Nil(t, it.Redes, "redes vacías no salen")
	require.NotNil(t, it.Cambios)
	assert.Equal(t, &wa, it.Cambios.WhatsApp)
	assert.NotNil(t, it.EnviadoEn)
}
