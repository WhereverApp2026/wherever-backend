package embeddings_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/embeddings"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// negocioCompleto trae datos privados y de control que nunca deben vectorizarse.
func negocioCompleto() negocios.Business {
	ahora := time.Now()
	return negocios.Business{
		ID: bson.NewObjectID(), OwnerID: bson.NewObjectID(), Slug: "cafe-luna-secreto",
		Name: "Café Luna", Category: "cafeteria", Tags: []string{"café", "postres"},
		Description: "Café de especialidad con terraza.",
		Phone:       "+5215512345678", QRCheckin: "qr-privado-123",
		Address:  negocios.Direccion{Calle: "Calle Privada", Numero: "77", Colonia: "Colonia Oculta", CP: "06000"},
		Location: negocios.Location{Type: "Point", Coordinates: []float64{-99.1332, 19.4326}},
		Revision: negocios.Revision{MotivoRechazo: "nota interna del revisor"}, CreatedAt: ahora, UpdatedAt: ahora,
	}
}

func textoDe(b negocios.Business) string {
	d := embeddings.Documento(embeddings.DesdeNegocio(b))
	return d.Titulo + " " + d.Texto
}

func TestPlantilla_SoloCamposPublicos(t *testing.T) {
	b := negocioCompleto()
	d := embeddings.Documento(embeddings.DesdeNegocio(b))

	assert.Equal(t, "Café Luna", d.Titulo)
	assert.Equal(t, "Cafeterías. Etiquetas: café, postres. Café de especialidad con terraza.", d.Texto)
}

func TestPlantilla_SinDatosPrivadosNiDeControl(t *testing.T) {
	b := negocioCompleto()
	texto := textoDe(b)

	for _, privado := range []string{b.Phone, "5512345678", b.QRCheckin, b.Slug, b.OwnerID.Hex(), b.ID.Hex(),
		"Calle Privada", "Colonia Oculta", "06000", "19.4326", "-99.1332", "nota interna"} {
		assert.NotContains(t, texto, privado)
	}
}

func TestPlantilla_DescripcionConContactoSeAnonimiza(t *testing.T) {
	b := negocioCompleto()
	b.Description = "Reserva al 55 1234 5678, escribe a dueno@example.com o visita https://ejemplo.test/menu"
	texto := textoDe(b)

	assert.NotContains(t, texto, "1234")
	assert.NotContains(t, texto, "dueno@example.com")
	assert.NotContains(t, texto, "ejemplo.test")
	assert.True(t, strings.Contains(texto, "[correo]"))
}

func TestVersion_CambiaConTextoModeloYPlantilla(t *testing.T) {
	b := negocioCompleto()
	d := embeddings.Documento(embeddings.DesdeNegocio(b))
	v := embeddings.Version("gemini/gemini-embedding-2", d)

	assert.Equal(t, v, embeddings.Version("gemini/gemini-embedding-2", d), "mismo texto, misma versión")
	assert.True(t, strings.HasPrefix(v, "v1:gemini/gemini-embedding-2:"))
	assert.NotEqual(t, v, embeddings.Version("otro/modelo", d))
	b.Description = "Otra descripción"
	assert.NotEqual(t, v, embeddings.Version("gemini/gemini-embedding-2", embeddings.Documento(embeddings.DesdeNegocio(b))))
	b = negocioCompleto()
	b.Phone = "+5215599999999"
	assert.Equal(t, v, embeddings.Version("gemini/gemini-embedding-2", embeddings.Documento(embeddings.DesdeNegocio(b))),
		"un cambio en un campo privado no cambia el texto")
}
