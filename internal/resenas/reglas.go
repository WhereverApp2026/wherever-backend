package resenas

import (
	"math"
	"slices"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/planes"
)

// Chips es el catálogo fijo de chips de una reseña: al no haber texto libre no pasan por la
// moderación.
var Chips = []string{
	"buena_comida", "buen_servicio", "buen_precio", "ambiente_agradable", "limpio", "rapido",
	"porciones_grandes", "accesible", "pet_friendly", "ideal_grupos",
	"lento", "caro", "ruidoso", "poco_limpio", "mala_atencion",
}

// MotivosReporte son los motivos predefinidos de un reporte.
var MotivosReporte = []string{"ofensiva", "falsa", "spam", "datos_personales", "otro"}

// ChipsValidos dice si todos los chips son del catálogo y no se repiten.
func ChipsValidos(chips []string) bool {
	if len(chips) > MaxChips {
		return false
	}
	vistos := map[string]bool{}
	for _, c := range chips {
		if !slices.Contains(Chips, c) || vistos[c] {
			return false
		}
		vistos[c] = true
	}
	return true
}

// Dia es la fecha local (America/Mexico_City) con que un check-in cuenta como visita del día.
func Dia(t time.Time, loc *time.Location) string { return t.In(loc).Format(time.DateOnly) }

// DistanciaM es la distancia en metros, redondeada, entre dos puntos.
func DistanciaM(lat1, lng1, lat2, lng2 float64) int {
	return int(math.Round(planes.Haversine(lat1, lng1, lat2, lng2) * 1000))
}

// SaltoImposible dice si ir de un negocio al otro en ese tiempo exige más de 300 km/h. Abajo
// de 1 km no se revisa: dos negocios vecinos se visitan seguidos.
func SaltoImposible(km float64, transcurrido time.Duration) bool {
	if km <= SaltoMinKm {
		return false
	}
	horas := transcurrido.Hours()
	if horas <= 0 {
		return true
	}
	return km/horas > VelocidadMaxKmh
}

// MotivoUbicacion decide si un check-in por ubicación es válido; vacío = válido.
func MotivoUbicacion(distanciaM, precisionM int) string {
	if precisionM > PrecisionMaxM || distanciaM > RadioMaxM {
		return MotivoFueraDeRadio
	}
	return ""
}

// Editable dice si la reseña sigue en su plazo de edición (48 h desde que se creó).
func Editable(creada, now time.Time) bool { return now.Sub(creada) <= PlazoEdicion }

// Promedio es suma/n con un decimal (como `$round` de MongoDB: mitades al par); 0 si n = 0.
func Promedio(suma, n int64) float64 {
	if n <= 0 {
		return 0
	}
	return math.RoundToEven(float64(suma)/float64(n)*10) / 10
}

// diasEn convierte días en una duración.
func diasEn(n int) time.Duration { return time.Duration(n) * 24 * time.Hour }
