package panel

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// textos devuelve el texto de cada <text> del SVG y falla si no es XML válido.
func textos(t *testing.T, svg string) []string {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(svg))
	var out []string
	var dentro bool
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return out
		}
		require.NoError(t, err, svg)
		switch v := tok.(type) {
		case xml.StartElement:
			assert.NotEqual(t, "script", strings.ToLower(v.Name.Local), "el SVG no lleva scripts")
			for _, a := range v.Attr {
				assert.False(t, strings.HasPrefix(strings.ToLower(a.Name.Local), "on"), "sin atributos de evento: %s", a.Name.Local)
			}
			dentro = v.Name.Local == "text"
		case xml.CharData:
			if dentro {
				out = append(out, string(v))
			}
		case xml.EndElement:
			dentro = false
		}
	}
}

func TestInsigniaSVG_NombreYAnio(t *testing.T) {
	got := textos(t, InsigniaSVG("Fonda Doña Meche", 2026))
	assert.Equal(t, []string{"SELLO DE EXCELENCIA", "Fonda Doña Meche", "WHEREVER · 2026"}, got)
}

func TestInsigniaSVG_EscapaElNombre(t *testing.T) {
	for _, nombre := range []string{
		`<script>alert(1)</script>`,
		`Tacos "El Güero" & Hijos`,
		`x" onload="alert(1)`,
		`</text><image href="x.png"/><text>`,
	} {
		svg := InsigniaSVG(nombre, 2026)
		got := textos(t, svg)
		require.Len(t, got, 3, nombre)
		assert.Equal(t, nombre, got[1], "el nombre se ve tal cual, como texto")
		assert.NotContains(t, svg, "<script", nombre)
		assert.NotContains(t, svg, "<image", nombre)
	}
}

func TestInsigniaSVG_RecortaYJuntaEspacios(t *testing.T) {
	largo := strings.Repeat("Restaurante ", 6)
	got := textos(t, InsigniaSVG(largo, 2026))[1]
	assert.Equal(t, insigniaNombreMax, len([]rune(got)))
	assert.True(t, strings.HasSuffix(got, "…"))
	assert.Equal(t, "Café Centro", textos(t, InsigniaSVG("  Café \n  Centro ", 2026))[1])
}

func TestNewInsignia_DatosDelSello(t *testing.T) {
	b := negocios.Business{ID: bson.NewObjectID(), Name: "Mezcalería Agave", Slug: "mezcaleria-agave", Version: 7,
		ExcellenceSeal: true, Sello: &negocios.Sello{Otorgado: true, OtorgadoEn: time.Date(2026, 11, 3, 18, 0, 0, 0, time.UTC)}}
	r := newInsignia(b)
	assert.Equal(t, b.ID.Hex(), r.NegocioID)
	assert.Equal(t, "2026-11-03", r.OtorgadoEn)
	assert.Equal(t, 2026, r.Anio)
	assert.Equal(t, "sello-mezcaleria-agave.svg", r.Archivo())
	assert.Equal(t, 7, r.Version())
	assert.Contains(t, r.SVG, "Mezcalería Agave")
}
