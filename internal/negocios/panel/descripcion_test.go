package panel

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

func TestValidarPalabras_Acepta(t *testing.T) {
	got, err := ValidarPalabras([]string{" tacos  al pastor ", "música en vivo", "pet-friendly"})
	require.NoError(t, err)
	assert.Equal(t, []string{"tacos al pastor", "música en vivo", "pet-friendly"}, got)

	got, err = ValidarPalabras([]string{"café", "24\nhoras", "Ñoño"})
	require.NoError(t, err)
	assert.Equal(t, []string{"café", "24 horas", "Ñoño"}, got, "los saltos se vuelven espacios")
}

func TestValidarPalabras_Rechaza(t *testing.T) {
	for nombre, palabras := range map[string][]string{
		"dos":           {"tacos", "salsa"},
		"cuatro":        {"tacos", "salsa", "cerveza", "música"},
		"ninguna":       nil,
		"una letra":     {"a", "salsa", "cerveza"},
		"vacía":         {"   ", "salsa", "cerveza"},
		"más de 30":     {strings.Repeat("a", 31), "salsa", "cerveza"},
		"url":           {"https://x.mx", "salsa", "cerveza"},
		"correo":        {"yo@correo.mx", "salsa", "cerveza"},
		"puntuación":    {"tacos!", "salsa", "cerveza"},
		"empieza guion": {"-tacos", "salsa", "cerveza"},
	} {
		_, err := ValidarPalabras(palabras)
		var e *apperr.Error
		require.True(t, errors.As(err, &e), nombre)
		assert.Equal(t, apperr.KindValidation, e.Kind, nombre)
	}
}

func TestValidarPalabras_TreintaRunasConAcentos(t *testing.T) {
	_, err := ValidarPalabras([]string{strings.Repeat("á", 30), "salsa", "cerveza"})
	assert.NoError(t, err, "el largo se mide en runas, no en bytes")
}

func TestPlantillaDescripcion(t *testing.T) {
	assert.Equal(t, "Restaurante con tacos, salsa y cerveza. Te esperamos para que lo vivas en persona.",
		PlantillaDescripcion("restaurante", []string{"tacos", "salsa", "cerveza"}))
	assert.True(t, strings.HasPrefix(PlantillaDescripcion("", []string{"a1", "b2", "c3"}), "Negocio con a1"))
	assert.True(t, strings.HasPrefix(PlantillaDescripcion("ópera", []string{"a1", "b2", "c3"}), "Ópera con"))
	larga := PlantillaDescripcion(strings.Repeat("x", 400), []string{"a1", "b2", "c3"})
	assert.Equal(t, MaxDescripcion, utf8.RuneCountInString(larga))
}

func TestContextoDescripcion_SinDatosDelDueño(t *testing.T) {
	assert.Equal(t, "Categoría: cafetería. Rango de precio: $$ de $$$$.", contextoDescripcion("cafetería", 2))
	assert.Equal(t, "Categoría: bar. Rango de precio: sin dato.", contextoDescripcion("bar", 0))
}

func TestSalidaDescripcion_Validar(t *testing.T) {
	ok := salidaDescripcion{Descripcion: "Taquería familiar\ncon tacos al pastor,\r\n salsas de la casa y cerveza fría para compartir."}
	require.NoError(t, ok.Validar())
	assert.NotContains(t, ok.Descripcion, "\n")
	assert.Equal(t, "Taquería familiar con tacos al pastor, salsas de la casa y cerveza fría para compartir.", ok.Descripcion)

	larga := salidaDescripcion{Descripcion: strings.Repeat("ñ", 320)}
	require.NoError(t, larga.Validar())
	assert.Equal(t, MaxDescripcion, utf8.RuneCountInString(larga.Descripcion), "se recorta a 300 por runa")

	assert.Error(t, (&salidaDescripcion{Descripcion: "Muy corta."}).Validar())
	assert.Error(t, (&salidaDescripcion{Descripcion: "Visítanos y escríbenos a contacto@negocio.mx para reservar tu mesa con anticipación."}).Validar())
}
