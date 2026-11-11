package asistente

import (
	"context"
	_ "embed" // instrucciones fijas versionadas en prompts/
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendador"
)

// instruccionesItinerario son las instrucciones fijas del paso 2 (US-042).
//
//go:embed prompts/itinerario.txt
var instruccionesItinerario string

// Límites del itinerario que propone la IA.
const (
	MinParadasIA        = 2
	MaxParadasIA        = 4
	MinMinutosIA        = 15
	MaxMinutosIA        = 240
	maxTokensItinerario = 512
	minutosDia          = 24 * 60
)

// ParadaIA es una parada propuesta por el modelo: un id de la lista, la hora y la estancia.
type ParadaIA struct {
	ID      string `json:"id"`
	Llega   string `json:"llega"`
	Minutos int    `json:"minutos"`
}

// Itinerario es la salida del paso 2. Validar revisa solo la forma (un error se reintenta una
// vez); ids, horarios y presupuesto los revisa Verificar contra el catálogo, que reemplaza en
// lugar de reintentar y así mide la tasa de alucinación.
type Itinerario struct {
	Paradas []ParadaIA `json:"paradas"`
}

// Validar cumple ia.Validable.
func (it *Itinerario) Validar() error {
	var errs []error
	if n := len(it.Paradas); n < MinParadasIA || n > MaxParadasIA {
		errs = append(errs, fmt.Errorf("paradas: %d fuera de %d a %d", n, MinParadasIA, MaxParadasIA))
	}
	for i, p := range it.Paradas {
		if _, err := time.Parse(formatoHora, p.Llega); err != nil {
			errs = append(errs, fmt.Errorf("paradas[%d].llega %q", i, p.Llega))
		}
		if p.Minutos < MinMinutosIA || p.Minutos > MaxMinutosIA {
			errs = append(errs, fmt.Errorf("paradas[%d].minutos %d", i, p.Minutos))
		}
	}
	return errors.Join(errs...)
}

func esquemaItinerario() map[string]any {
	parada := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":      map[string]any{"type": "string"},
			"llega":   map[string]any{"type": "string"},
			"minutos": map[string]any{"type": "integer", "minimum": MinMinutosIA, "maximum": MaxMinutosIA},
		},
		"required":             []string{"id", "llega", "minutos"},
		"additionalProperties": false,
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"paradas": map[string]any{"type": "array", "minItems": MinParadasIA, "maxItems": MaxParadasIA, "items": parada},
		},
		"required":             []string{"paradas"},
		"additionalProperties": false,
	}
}

// GenerarItinerario pide a la IA de 2 a 4 paradas elegidas de los candidatos.
func GenerarItinerario(ctx context.Context, m ia.Modelo, recs []recomendador.Recomendacion, p Pedido, in Interpretacion) (Itinerario, error) {
	var out Itinerario
	_, err := generarValidado(ctx, m, PeticionItinerario(recs, p, in), &out)
	return out, err
}

// PeticionItinerario arma la petición del paso 2. Al modelo van solo datos públicos de los
// candidatos (id, nombre, giro, precio y horario del día), la hora de inicio y el presupuesto;
// lo que se entendió de la frase va en el turno de usuario. Nada del perfil: alergias y dieta
// ya filtraron los candidatos, y el ánimo solo llega si la persona lo consintió (el service lo
// borra antes). Ni ubicación ni ids de personas.
func PeticionItinerario(recs []recomendador.Recomendacion, p Pedido, in Interpretacion) ia.Peticion {
	ini := p.Inicio.In(zona)
	var b strings.Builder
	fmt.Fprintf(&b, "Inicio: %s a las %s. Presupuesto por persona: %d MXN.\n",
		diasLargo[ini.Weekday()], ini.Format(formatoHora), p.PresupuestoMax)
	b.WriteString("Candidatos (id | nombre | giro | precio por persona | horario de ese día):\n")
	for _, r := range recs {
		n := r.Negocio
		fmt.Fprintf(&b, "%s | %s | %s | %d | %s\n", n.ID.Hex(), linea(n.Name), n.Category, n.AveragePrice, horarioDelDia(n.ScheduleMin, ini))
	}
	return ia.Peticion{
		Instrucciones: instruccionesItinerario,
		Contexto:      b.String(),
		TextoUsuario:  busqueda(in),
		Esquema:       esquemaItinerario(),
		MaxTokens:     maxTokensItinerario,
	}
}

// busqueda resume lo que se entendió de la frase, sin la frase.
func busqueda(in Interpretacion) string {
	partes := []string{}
	if in.Compania != "" && in.Compania != SinDato {
		partes = append(partes, "compañía "+in.Compania)
	}
	if in.Animo != "" && in.Animo != SinDato {
		partes = append(partes, "ánimo "+in.Animo)
	}
	if len(in.PalabrasClave) > 0 {
		partes = append(partes, strings.Join(in.PalabrasClave, ", "))
	}
	if len(partes) == 0 {
		return "La persona busca: un plan para salir."
	}
	return "La persona busca: " + strings.Join(partes, "; ") + "."
}

// linea quita saltos de línea y barras de un nombre para que no rompa la tabla.
func linea(s string) string {
	return strings.Join(strings.Fields(strings.NewReplacer("|", " ", "\n", " ", "\r", " ").Replace(s)), " ")
}

// horarioDelDia lista las ventanas del día del inicio (domingo = 0, como negocios.OpenAt).
func horarioDelDia(ventanas []negocios.VentanaMin, dia time.Time) string {
	var out []string
	for _, v := range ventanas {
		if v.Dia == int(dia.Weekday()) {
			out = append(out, minutoHora(v.Desde)+"-"+minutoHora(v.Hasta))
		}
	}
	if len(out) == 0 {
		return "cerrado"
	}
	return strings.Join(out, ", ")
}

// minutoHora: 1440 → "24:00" y 1560 → "02:00" (pasada la medianoche, del día siguiente).
func minutoHora(m int) string {
	if m > minutosDia {
		m -= minutosDia
	}
	return fmt.Sprintf("%02d:%02d", m/60, m%60)
}
