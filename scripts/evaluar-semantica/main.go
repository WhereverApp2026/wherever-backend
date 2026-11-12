// Command evaluar-semantica mide la búsqueda semántica (spec nv-51): 30 frases con sus
// lugares esperados sobre un catálogo ficticio de 30 negocios (conjunto.json). Compara el
// top-5 del motor de reglas, de la búsqueda semántica y del motor con el factor
// `afinidad_semantica`, e imprime la tabla en Markdown para docs/ml-evaluacion.md.
//
// Usa Gemini real: necesita GEMINI_API_KEY e IA_EMBEDDINGS_ID en el entorno (la llave sale
// de SSM y nunca se imprime). Son 31 llamadas: un lote de 30 negocios y una por frase, con
// una pausa para quedar debajo del límite por minuto. No usa MongoDB ni Redis.
//
//	go run ./scripts/evaluar-semantica > /tmp/evaluacion.md
package main

import (
	"context"
	_ "embed" // conjunto.json viaja con el comando
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/embeddings"
	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendador"
	"github.com/WhereverApp2026/wherever-backend/internal/semantica"
)

//go:embed conjunto.json
var conjuntoJSON []byte

const (
	top    = 5
	pausa  = 1500 * time.Millisecond
	plazo  = 3 * time.Minute
	tiempo = 20 * time.Second
)

type conjunto struct {
	Negocios []struct {
		Slug         string   `json:"slug"`
		Nombre       string   `json:"nombre"`
		Categoria    string   `json:"categoria"`
		Etiquetas    []string `json:"etiquetas"`
		Descripcion  string   `json:"descripcion"`
		Calificacion float64  `json:"calificacion"`
	} `json:"negocios"`
	Frases []struct {
		Frase     string   `json:"frase"`
		Esperados []string `json:"esperados"`
	} `json:"frases"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "evaluar-semantica:", err)
		os.Exit(1)
	}
}

func run() error {
	llave, modelo := os.Getenv(config.KeyGeminiAPIKey), os.Getenv(config.KeyIAEmbeddingsID)
	if llave == "" || modelo == "" {
		return errors.New("faltan " + config.KeyGeminiAPIKey + " o " + config.KeyIAEmbeddingsID)
	}
	var c conjunto
	if err := json.Unmarshal(conjuntoJSON, &c); err != nil {
		return err
	}
	g, err := ia.NewGeminiVectores(ia.GeminiVectoresConfig{APIKey: llave, ModeloID: modelo, Timeout: tiempo})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), plazo)
	defer cancel()

	visibles, vectores, err := vectorizar(ctx, g, c)
	if err != nil {
		return err
	}
	pesos, err := recomendador.PesosPorDefecto()
	if err != nil {
		return err
	}
	// Lunes 13:00 en Ciudad de México: todos abiertos, el horario no decide.
	ahora := time.Date(2026, 11, 9, 13, 0, 0, 0, negocios.MexicoCity())
	var filas []fila
	for _, f := range c.Frases {
		time.Sleep(pausa)
		q, err := g.Consulta(ctx, f.Frase)
		if err != nil {
			return err
		}
		sims := map[bson.ObjectID]float64{}
		for id, v := range vectores {
			sims[id] = semantica.Punto(q, v)
		}
		afin := semantica.Afinidades(sims)
		filas = append(filas, fila{
			frase: f.Frase, esperados: f.Esperados,
			reglas:    slugs(semantica.Reglas(visibles, f.Frase, ahora, pesos, top)),
			semantica: slugs(semantica.PorAfinidad(visibles, afin, top)),
			hibrido:   slugs(semantica.Hibrido(visibles, f.Frase, afin, ahora, pesos, top)),
		})
	}
	imprimir(g.Nombre(), filas)
	return nil
}

// vectorizar manda el catálogo en un solo lote.
func vectorizar(ctx context.Context, g *ia.GeminiVectores, c conjunto) ([]negocios.Business, map[bson.ObjectID][]float32, error) {
	visibles, docs := catalogo(c)
	vs, err := g.Documentos(ctx, docs)
	if err != nil {
		return nil, nil, err
	}
	out := make(map[bson.ObjectID][]float32, len(vs))
	for i, v := range vs {
		out[visibles[i].ID] = v
	}
	return visibles, out, nil
}

// catalogo arma los negocios (abiertos todo el día) y sus documentos con la plantilla de nv-50.
func catalogo(c conjunto) ([]negocios.Business, []ia.Documento) {
	todoElDia := make([]negocios.VentanaMin, 0, 7)
	for d := range 7 {
		todoElDia = append(todoElDia, negocios.VentanaMin{Dia: d, Desde: 0, Hasta: 24 * 60})
	}
	bs := make([]negocios.Business, len(c.Negocios))
	docs := make([]ia.Documento, len(c.Negocios))
	for i, n := range c.Negocios {
		bs[i] = negocios.Business{
			ID: bson.NewObjectID(), Slug: n.Slug, Name: n.Nombre, Category: n.Categoria, Tags: n.Etiquetas,
			Description: n.Descripcion, ScheduleMin: todoElDia,
			Rating: negocios.Calificacion{Promedio: n.Calificacion, TotalResenas: 20},
		}
		docs[i] = embeddings.Documento(embeddings.DesdeNegocio(bs[i]))
	}
	return bs, docs
}

type fila struct {
	frase                      string
	esperados                  []string
	reglas, semantica, hibrido []string
}

func slugs(bs []negocios.Business) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Slug
	}
	return out
}

// acierto dice si algún esperado está en el top; primero, si el primero es esperado.
func acierto(esperados, lista []string) (top5, primero bool) {
	for i, s := range lista {
		if slices.Contains(esperados, s) {
			return true, i == 0
		}
	}
	return false, false
}

func imprimir(modelo string, filas []fila) {
	var a [3][2]int
	fmt.Printf("Modelo: `%s`. Frases: %d. Fecha: %s.\n\n", modelo, len(filas), time.Now().Format("2006-01-02"))
	fmt.Println("| # | Frase | Esperado | Reglas | Semántica | Reglas + afinidad |")
	fmt.Println("|---|---|---|---|---|---|")
	for i, f := range filas {
		celdas := make([]string, 3)
		for j, lista := range [][]string{f.reglas, f.semantica, f.hibrido} {
			t5, p := acierto(f.esperados, lista)
			celdas[j] = marca(t5, p)
			if t5 {
				a[j][0]++
			}
			if p {
				a[j][1]++
			}
		}
		fmt.Printf("| %d | %s | %s | %s | %s | %s |\n", i+1, f.frase, strings.Join(f.esperados, ", "), celdas[0], celdas[1], celdas[2])
	}
	n := len(filas)
	fmt.Println()
	fmt.Println("| Método | Aciertos top-5 | Aciertos top-1 |")
	fmt.Println("|---|---|---|")
	for j, nombre := range []string{"Reglas (sin semántica)", "Semántica", "Reglas + afinidad"} {
		fmt.Printf("| %s | %d/%d (%.0f %%) | %d/%d (%.0f %%) |\n", nombre, a[j][0], n, pct(a[j][0], n), a[j][1], n, pct(a[j][1], n))
	}
}

func marca(top5, primero bool) string {
	switch {
	case primero:
		return "✅ 1.º"
	case top5:
		return "✅"
	default:
		return "❌"
	}
}

func pct(a, n int) float64 {
	if n == 0 {
		return 0
	}
	return 100 * float64(a) / float64(n)
}
