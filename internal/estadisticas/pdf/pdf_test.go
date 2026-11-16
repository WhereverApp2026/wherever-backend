package pdf_test

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/estadisticas/pdf"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/estadisticastest"
)

func TestPDF_CabeceraXrefYTexto(t *testing.T) {
	doc := pdf.Nuevo()
	doc.Titulo("Café Ñandú (centro)")
	doc.Parrafo("Atención — reseñas \\ de noviembre")
	doc.Tabla([]float64{200, 100}, []string{"Cifra", "Valor"}, [][]string{{"Reservas", "12"}})
	b := doc.Bytes()

	require.True(t, bytes.HasPrefix(b, []byte("%PDF-1.4\n")))
	require.True(t, bytes.HasSuffix(b, []byte("%%EOF\n")))
	assert.Contains(t, string(b), "/Encoding /WinAnsiEncoding")

	// Cada desplazamiento de la xref apunta al inicio de su objeto.
	m := regexp.MustCompile(`startxref\n(\d+)\n`).FindSubmatch(b)
	require.NotNil(t, m)
	inicio, err := strconv.Atoi(string(m[1]))
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(b[inicio:], []byte("xref\n")))
	filas := regexp.MustCompile(`(\d{10}) 00000 n `).FindAllSubmatch(b[inicio:], -1)
	require.Len(t, filas, 6, "catálogo, páginas, dos fuentes, página y contenido")
	for i, f := range filas {
		off, err := strconv.Atoi(string(f[1]))
		require.NoError(t, err)
		assert.True(t, bytes.HasPrefix(b[off:], []byte(strconv.Itoa(i+1)+" 0 obj")), "objeto %d", i+1)
	}

	// Acentos en WinAnsi (é = 0xE9) y paréntesis escapados.
	assert.Contains(t, string(b), "Caf\xe9 \xd1and\xfa \\(centro\\)")
	texto := estadisticastest.Texto(b)
	assert.Contains(t, texto, "Café Ñandú (centro)")
	assert.Contains(t, texto, "Atención — reseñas \\ de noviembre")
	assert.Contains(t, texto, "Reservas")
}

func TestPDF_SaltoDePagina(t *testing.T) {
	doc := pdf.Nuevo()
	for i := range 80 {
		doc.Parrafo("Renglón " + strconv.Itoa(i))
	}
	require.Equal(t, 2, doc.Paginas())
	b := doc.Bytes()
	assert.Contains(t, string(b), "/Count 2")
	assert.Equal(t, 2, strings.Count(string(b), "/Type /Page /Parent"))
	texto := estadisticastest.Texto(b)
	assert.Contains(t, texto, "Renglón 0\n")
	assert.Contains(t, texto, "Renglón 79\n")
}

func TestPDF_PartirYRecortar(t *testing.T) {
	assert.Equal(t, []string{"uno dos", "tres"}, pdf.Partir("uno dos tres", 8))
	assert.Equal(t, []string{""}, pdf.Partir("  ", 8))
	assert.Equal(t, "abcd…", pdf.Recortar("abcdefgh", 5))
	assert.Equal(t, "abc", pdf.Recortar("abc", 5))
	assert.Equal(t, []byte("a?b"), pdf.WinAnsi("a☃b"), "lo que no existe en WinAnsi sale como ?")
}
