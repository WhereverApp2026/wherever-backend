// Package asistente arma un plan a partir de una frase libre (nv-31): la IA interpreta la
// frase, el motor de reglas (nv-23) elige los candidatos y planes.Recalcular (nv-29) calcula
// traslados y costos. El itinerario con IA y su verificación llegan con nv-32; el respaldo
// por reglas con nv-33.
package asistente

import (
	"context"
	_ "embed" // instrucciones fijas versionadas en prompts/
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
)

// instruccionesInterpretar son las instrucciones fijas del paso 1. La frase nunca se mezcla
// con ellas: va sola en el turno de usuario.
//
//go:embed prompts/interpretar.txt
var instruccionesInterpretar string

// Valores cerrados de la interpretación.
const (
	SinDato = "sin_dato"

	MaxPresupuesto    = 20000 // MXN por persona
	MaxPalabrasClave  = 5
	MaxLargoPalabra   = 30
	maxTokensSalida   = 256
	formatoHora       = "15:04"
	diasSemanaPalabra = "domingo lunes martes miércoles jueves viernes sábado"
)

var (
	companias = []string{"solo", "pareja", "amigos", "familia", SinDato}
	animos    = []string{"relajado", "activo", "romantico", "festivo", SinDato}
	diasLargo = strings.Fields(diasSemanaPalabra)
)

// Interpretacion es la salida validada del paso 1 (US-041).
type Interpretacion struct {
	Compania       string   `json:"compania"`
	ConNinos       bool     `json:"con_ninos"`
	ConMascota     bool     `json:"con_mascota"`
	PresupuestoMax int      `json:"presupuesto_max"`
	Hora           string   `json:"hora"`
	Animo          string   `json:"animo"`
	PalabrasClave  []string `json:"palabras_clave"`
}

// Validar rechaza valores fuera del catálogo y normaliza las palabras clave. ia.Decodificar
// la llama; un error cuenta como respuesta inválida y se reintenta una vez.
func (in *Interpretacion) Validar() error {
	var errs []error
	if !slices.Contains(companias, in.Compania) {
		errs = append(errs, fmt.Errorf("compania %q", in.Compania))
	}
	if !slices.Contains(animos, in.Animo) {
		errs = append(errs, fmt.Errorf("animo %q", in.Animo))
	}
	if in.PresupuestoMax < 0 || in.PresupuestoMax > MaxPresupuesto {
		errs = append(errs, fmt.Errorf("presupuesto_max %d", in.PresupuestoMax))
	}
	if in.Hora != "" {
		if _, err := time.Parse(formatoHora, in.Hora); err != nil {
			errs = append(errs, fmt.Errorf("hora %q", in.Hora))
		}
	}
	palabras, err := limpiarPalabras(in.PalabrasClave)
	if err != nil {
		errs = append(errs, err)
	}
	in.PalabrasClave = palabras
	return errors.Join(errs...)
}

func limpiarPalabras(raw []string) ([]string, error) {
	if len(raw) > MaxPalabrasClave {
		return nil, fmt.Errorf("palabras_clave: %d de máximo %d", len(raw), MaxPalabrasClave)
	}
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" || utf8.RuneCountInString(p) > MaxLargoPalabra {
			return nil, fmt.Errorf("palabras_clave: %q", p)
		}
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out, nil
}

// esquemaInterpretar es el JSON Schema que se pide al proveedor (responseJsonSchema).
func esquemaInterpretar() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"compania":        map[string]any{"type": "string", "enum": companias},
			"con_ninos":       map[string]any{"type": "boolean"},
			"con_mascota":     map[string]any{"type": "boolean"},
			"presupuesto_max": map[string]any{"type": "integer", "minimum": 0, "maximum": MaxPresupuesto},
			"hora":            map[string]any{"type": "string"},
			"animo":           map[string]any{"type": "string", "enum": animos},
			"palabras_clave": map[string]any{
				"type": "array", "maxItems": MaxPalabrasClave,
				"items": map[string]any{"type": "string", "maxLength": MaxLargoPalabra},
			},
		},
		"required":             []string{"compania", "con_ninos", "con_mascota", "presupuesto_max", "hora", "animo", "palabras_clave"},
		"additionalProperties": false,
	}
}

// Interpretar convierte la frase en parámetros. Hacia el modelo van solo las instrucciones
// fijas, el día y la hora local (sin datos de la persona) y la frase anonimizada.
func Interpretar(ctx context.Context, m ia.Modelo, frase string, ahora time.Time) (Interpretacion, error) {
	var out Interpretacion
	_, err := ia.GenerarValidado(ctx, m, PeticionInterpretar(frase, ahora), &out)
	return out, err
}

// PeticionInterpretar arma la petición del paso 1. Exportada para probar qué viaja al modelo.
func PeticionInterpretar(frase string, ahora time.Time) ia.Peticion {
	return ia.Peticion{
		Instrucciones: instruccionesInterpretar,
		Contexto:      fmt.Sprintf("Hoy es %s; son las %s.", diasLargo[ahora.Weekday()], ahora.Format(formatoHora)),
		TextoUsuario:  ia.Anonimizar(strings.Join(strings.Fields(frase), " ")),
		Esquema:       esquemaInterpretar(),
		MaxTokens:     maxTokensSalida,
	}
}
