package panel

import (
	"context"
	"fmt"
	"html"
	"strings"
	"unicode/utf8"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Insignia del Sello de Excelencia (nv-76): un SVG que el dueño imprime o comparte.
const (
	FormatoJSON = "json"
	FormatoSVG  = "svg"

	// insigniaNombreMax recorta nombres largos para que quepan en el círculo.
	insigniaNombreMax = 40
)

// InsigniaResponse es la insignia para la app; `svg` es el mismo dibujo de `formato=svg`.
type InsigniaResponse struct {
	NegocioID  string `json:"negocio_id"`
	Nombre     string `json:"nombre"`
	OtorgadoEn string `json:"otorgado_en"`
	Anio       int    `json:"anio"`
	SVG        string `json:"svg"`

	slug    string
	version int
}

// Archivo es el nombre del SVG descargado.
func (r InsigniaResponse) Archivo() string { return "sello-" + r.slug + ".svg" }

// Version es la del negocio, para el ETag.
func (r InsigniaResponse) Version() int { return r.version }

// Insignia da la insignia del negocio propio si tiene el Sello vigente. Ajeno, inexistente
// o sin Sello = 404, igual que el resto del panel.
func (s *Service) Insignia(ctx context.Context, claims *middleware.Claims, id string) (InsigniaResponse, error) {
	b, err := s.propio(ctx, claims, id)
	if err != nil {
		return InsigniaResponse{}, err
	}
	if !b.ExcellenceSeal || b.Sello == nil || !b.Sello.Otorgado {
		return InsigniaResponse{}, apperr.NotFound("el negocio no tiene el Sello de Excelencia")
	}
	return newInsignia(b), nil
}

func newInsignia(b negocios.Business) InsigniaResponse {
	otorgado := b.Sello.OtorgadoEn.UTC()
	return InsigniaResponse{
		NegocioID:  b.ID.Hex(),
		Nombre:     b.Name,
		OtorgadoEn: otorgado.Format("2006-01-02"),
		Anio:       otorgado.Year(),
		SVG:        InsigniaSVG(b.Name, otorgado.Year()),
		slug:       b.Slug,
		version:    b.Version,
	}
}

// InsigniaSVG dibuja la insignia. El nombre va escapado: un negocio llamado
// `<script>` o con comillas no puede agregar marcado ni atributos.
func InsigniaSVG(nombre string, anio int) string {
	nombre = strings.Join(strings.Fields(nombre), " ")
	if utf8.RuneCountInString(nombre) > insigniaNombreMax {
		nombre = string([]rune(nombre)[:insigniaNombreMax-1]) + "…"
	}
	tamano := 34
	if n := utf8.RuneCountInString(nombre); n > 22 {
		tamano = 24
	}
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 512 512" role="img" aria-label="Sello de Excelencia Wherever">`)
	b.WriteString(`<circle cx="256" cy="256" r="240" fill="#B8862B"/>`)
	b.WriteString(`<circle cx="256" cy="256" r="214" fill="#F4D58D" stroke="#8A6116" stroke-width="6"/>`)
	b.WriteString(`<circle cx="256" cy="256" r="190" fill="none" stroke="#8A6116" stroke-width="2" stroke-dasharray="6 8"/>`)
	b.WriteString(`<path d="M256 96l18 37 41 6-30 29 7 41-36-19-36 19 7-41-30-29 41-6z" fill="#8A6116"/>`)
	b.WriteString(`<g font-family="Arial, Helvetica, sans-serif" text-anchor="middle" fill="#3B2A08">`)
	b.WriteString(`<text x="256" y="286" font-size="30" font-weight="700" letter-spacing="2">SELLO DE EXCELENCIA</text>`)
	fmt.Fprintf(&b, `<text x="256" y="334" font-size="%d" font-weight="700">%s</text>`, tamano, html.EscapeString(nombre))
	fmt.Fprintf(&b, `<text x="256" y="380" font-size="22" letter-spacing="4">WHEREVER · %d</text>`, anio)
	b.WriteString(`</g></svg>`)
	return b.String()
}
