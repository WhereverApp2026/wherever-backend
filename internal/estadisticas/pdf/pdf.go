// Package pdf es un escritor mínimo de PDF 1.4 (nv-74): texto en Helvetica con WinAnsi (para
// acentos), tablas y líneas, en hojas carta con salto de página. Sin dependencias: la
// constitución §3 pide evaluar cada librería y la conocida (`go-pdf/fpdf`) está archivada.
package pdf

import (
	"bytes"
	"fmt"
	"strings"
)

// Medidas de la hoja carta en puntos.
const (
	anchoHoja  = 612.0
	altoHoja   = 792.0
	margen     = 54.0
	anchoUtil  = anchoHoja - 2*margen
	anchoLetra = 0.5 // ancho promedio de Helvetica, en fracción del tamaño
	interlinea = 1.4
	altoFila   = 16.0
	tamTitulo  = 18.0
	tamSub     = 13.0
	tamTexto   = 11.0
	tamTabla   = 10.0
	fuente     = "F1"
	negrita    = "F2"
	// caracteresPorLinea es lo que cabe en el ancho útil a tamTexto (91,6).
	caracteresPorLinea = 91
)

// Documento acumula páginas; se arma con Bytes.
type Documento struct {
	paginas []*bytes.Buffer
	y       float64
}

// Nuevo crea un documento con una página vacía.
func Nuevo() *Documento {
	d := &Documento{}
	d.nuevaPagina()
	return d
}

// Paginas dice cuántas páginas lleva.
func (d *Documento) Paginas() int { return len(d.paginas) }

func (d *Documento) nuevaPagina() {
	d.paginas = append(d.paginas, &bytes.Buffer{})
	d.y = altoHoja - margen
}

// espacio baja `alto` puntos y abre otra página si no cabe.
func (d *Documento) espacio(alto float64) {
	if d.y-alto < margen {
		d.nuevaPagina()
	}
	d.y -= alto
}

func (d *Documento) actual() *bytes.Buffer { return d.paginas[len(d.paginas)-1] }

func (d *Documento) texto(x float64, f string, tam float64, s string) {
	fmt.Fprintf(d.actual(), "BT /%s %.1f Tf %.2f %.2f Td (%s) Tj ET\n", f, tam, x, d.y, Escapar(WinAnsi(s)))
}

// Titulo escribe una línea grande en negrita.
func (d *Documento) Titulo(s string) {
	d.espacio(tamTitulo * interlinea)
	d.texto(margen, negrita, tamTitulo, s)
}

// Subtitulo escribe una línea mediana en negrita con un poco de aire arriba.
func (d *Documento) Subtitulo(s string) {
	d.espacio(tamSub * interlinea * 1.5)
	d.texto(margen, negrita, tamSub, s)
}

// Parrafo escribe texto partido en líneas que caben en el ancho útil.
func (d *Documento) Parrafo(s string) {
	for _, linea := range Partir(s, caracteresPorLinea) {
		d.espacio(tamTexto * interlinea)
		d.texto(margen, fuente, tamTexto, linea)
	}
}

// Tabla escribe el encabezado en negrita con una línea debajo y una fila por renglón. Cada
// celda se recorta a su ancho (en puntos).
func (d *Documento) Tabla(anchos []float64, encabezado []string, filas [][]string) {
	fila := func(f string, celdas []string) {
		d.espacio(altoFila)
		x := margen
		for i, c := range celdas {
			if i >= len(anchos) {
				break
			}
			d.texto(x, f, tamTabla, Recortar(c, int(anchos[i]/(tamTabla*anchoLetra))-1))
			x += anchos[i]
		}
	}
	fila(negrita, encabezado)
	fmt.Fprintf(d.actual(), "0.5 w %.2f %.2f m %.2f %.2f l S\n", margen, d.y-4, margen+suma(anchos), d.y-4)
	for _, f := range filas {
		fila(fuente, f)
	}
}

func suma(xs []float64) float64 {
	t := 0.0
	for _, x := range xs {
		t += x
	}
	return t
}

// Partir divide s en líneas de hasta max caracteres por palabras.
func Partir(s string, maximo int) []string {
	palabras := strings.Fields(s)
	if len(palabras) == 0 {
		return []string{""}
	}
	out := []string{}
	linea := ""
	for _, p := range palabras {
		switch {
		case linea == "":
			linea = p
		case len([]rune(linea))+1+len([]rune(p)) <= maximo:
			linea += " " + p
		default:
			out = append(out, linea)
			linea = p
		}
	}
	return append(out, linea)
}

// Recortar deja s en max caracteres con «…» al final si sobra.
func Recortar(s string, maximo int) string {
	r := []rune(s)
	if maximo < 1 || len(r) <= maximo {
		return s
	}
	return string(r[:maximo-1]) + "…"
}

// especiales son los caracteres de WinAnsi fuera de Latin-1 que usa el reporte.
var especiales = map[rune]byte{'€': 0x80, '…': 0x85, '‘': 0x91, '’': 0x92, '“': 0x93, '”': 0x94, '•': 0x95, '–': 0x96, '—': 0x97}

// WinAnsi pasa s a bytes de WinAnsiEncoding; lo que no existe ahí sale como «?».
func WinAnsi(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r < 0x80 || (r >= 0xA0 && r <= 0xFF):
			out = append(out, byte(r)) // #nosec G115 -- r < 0x100 por la condición del case
		case especiales[r] != 0:
			out = append(out, especiales[r])
		default:
			out = append(out, '?')
		}
	}
	return out
}

// Escapar protege paréntesis y barras de una cadena literal de PDF.
func Escapar(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		if c == '(' || c == ')' || c == '\\' {
			sb.WriteByte('\\')
		}
		sb.WriteByte(c)
	}
	return sb.String()
}

// Bytes arma el archivo: catálogo, páginas, dos fuentes Type1 y una página con su contenido
// por hoja, con la tabla `xref` de desplazamientos.
func (d *Documento) Bytes() []byte {
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")
	offsets := []int{}
	obj := func(cuerpo string) {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", len(offsets), cuerpo)
	}
	kids := make([]string, len(d.paginas))
	for i := range d.paginas {
		kids[i] = fmt.Sprintf("%d 0 R", 5+2*i)
	}
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(d.paginas)))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")
	for i, p := range d.paginas {
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /%s 3 0 R /%s 4 0 R >> >> /Contents %d 0 R >>",
			anchoHoja, altoHoja, fuente, negrita, 6+2*i))
		obj(fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", p.Len(), p.String()))
	}
	inicioXref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, inicioXref)
	return out.Bytes()
}
