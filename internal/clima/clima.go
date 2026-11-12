// Package clima trae el clima actual de Open-Meteo (spec nv-25): sin llave, con plazo de
// 3 s y caché de 30 min en memoria por instancia. Si el servicio está lento o caído, el clima
// sale «no disponible» y quien lo usa sigue sin él (el factor del recomendador queda neutro).
package clima

import (
	"context"
	"errors"
	"time"
)

// ErrProveedor es una respuesta del proveedor que no se pudo usar.
var ErrProveedor = errors.New("clima: respuesta inválida del proveedor")

// Estados según el código WMO de Open-Meteo.
const (
	EstadoDespejado = "despejado"
	EstadoNublado   = "nublado"
	EstadoNiebla    = "niebla"
	EstadoLlovizna  = "llovizna"
	EstadoLluvia    = "lluvia"
	EstadoNieve     = "nieve"
	EstadoTormenta  = "tormenta"
)

// Umbrales de mal clima: con esto, salir a una terraza deja de ser buena idea.
const (
	calorC        = 32.0
	frioC         = 8.0
	precipitacion = 0.2 // mm en el último intervalo
)

// Lectura es lo que entrega el proveedor.
type Lectura struct {
	TemperaturaC    float64
	Codigo          int
	EsDeDia         bool
	PrecipitacionMM float64
	ObservadoEn     time.Time
}

// Proveedor lee el clima actual de una coordenada. Lo implementa OpenMeteo.
type Proveedor interface {
	Actual(ctx context.Context, lat, lng float64) (Lectura, error)
}

// Clima es la respuesta de GET /contexto/clima y el bloque `clima` de GET /inicio. Sin
// servicio solo lleva `disponible: false`.
type Clima struct {
	Disponible      bool       `json:"disponible"`
	TemperaturaC    *float64   `json:"temperatura_c,omitempty"`
	Estado          string     `json:"estado,omitempty"`
	EsDeDia         *bool      `json:"es_de_dia,omitempty"`
	PrecipitacionMM *float64   `json:"precipitacion_mm,omitempty"`
	Adverso         *bool      `json:"adverso,omitempty"`
	ObservadoEn     *time.Time `json:"observado_en,omitempty"`
}

// NoDisponible es el clima sin servicio.
func NoDisponible() Clima { return Clima{} }

// De convierte una lectura del proveedor.
func De(l Lectura) Clima {
	estado := EstadoDe(l.Codigo)
	adverso := Adverso(estado, l.TemperaturaC, l.PrecipitacionMM)
	observado := l.ObservadoEn.UTC()
	return Clima{
		Disponible: true, TemperaturaC: &l.TemperaturaC, Estado: estado, EsDeDia: &l.EsDeDia,
		PrecipitacionMM: &l.PrecipitacionMM, Adverso: &adverso, ObservadoEn: &observado,
	}
}

// rangosWMO lleva cada rango de códigos WMO (0–99) a un estado corto.
var rangosWMO = []struct {
	desde, hasta int
	estado       string
}{
	{0, 0, EstadoDespejado},
	{1, 3, EstadoNublado},
	{45, 48, EstadoNiebla},
	{51, 57, EstadoLlovizna},
	{61, 67, EstadoLluvia},
	{71, 77, EstadoNieve},
	{80, 82, EstadoLluvia},
	{85, 86, EstadoNieve},
	{95, 99, EstadoTormenta},
}

// EstadoDe traduce el código WMO; uno desconocido cuenta como nublado.
func EstadoDe(codigo int) string {
	for _, r := range rangosWMO {
		if codigo >= r.desde && codigo <= r.hasta {
			return r.estado
		}
	}
	return EstadoNublado
}

// Adverso dice si el clima castiga los lugares al aire libre.
func Adverso(estado string, temperaturaC, precipitacionMM float64) bool {
	switch estado {
	case EstadoLlovizna, EstadoLluvia, EstadoNieve, EstadoTormenta:
		return true
	}
	return precipitacionMM >= precipitacion || temperaturaC >= calorC || temperaturaC <= frioC
}
