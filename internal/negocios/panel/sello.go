package panel

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// Regla del Sello de Excelencia (A5 US-154; la espera nv-62). No se otorga sola: el
// administrador decide con PUT /admin/negocios/{id}/sello.
const (
	SelloDiasAprobado  = 60
	SelloResenas90     = 20
	SelloPromedio90    = 4.5
	SelloRechazosMax90 = 0
)

// Nombres de los criterios del avance.
const (
	CriterioDias     = "dias_aprobado"
	CriterioResenas  = "resenas_verificadas_90d"
	CriterioPromedio = "promedio_90d"
	CriterioRechazos = "rechazos_moderacion_90d"
)

// Criterio es un requisito del Sello. `actual` es null si su fuente (nv-62) no está registrada.
type Criterio struct {
	Nombre string   `json:"nombre"`
	Actual *float64 `json:"actual"`
	Meta   float64  `json:"meta"`
	Cumple bool     `json:"cumple"`
}

// AvanceSello dice qué tan cerca está el negocio del Sello.
type AvanceSello struct {
	Elegible  bool       `json:"elegible"`
	Criterios []Criterio `json:"criterios"`
}

// Avance aplica la regla: ≥ 60 días aprobado, ≥ 20 reseñas verificadas en 90 días con
// promedio ≥ 4.5 y ningún contenido rechazado por moderación en ese periodo. Sin reseñas
// (fuente sin registrar o caída) no es elegible.
func Avance(aprobadoEn *time.Time, now time.Time, r *negocios.ResumenResenas) AvanceSello {
	var dias *float64
	if aprobadoEn != nil {
		d := float64(int(now.Sub(*aprobadoEn).Hours() / 24))
		dias = &d
	}
	var resenas, promedio, rechazos *float64
	if r != nil {
		n, p, x := float64(r.Verificadas90), r.Promedio90, float64(r.RechazadasModeracion90)
		resenas, promedio, rechazos = &n, &p, &x
	}
	criterios := []Criterio{
		minimo(CriterioDias, dias, SelloDiasAprobado),
		minimo(CriterioResenas, resenas, SelloResenas90),
		minimo(CriterioPromedio, promedio, SelloPromedio90),
		maximo(CriterioRechazos, rechazos, SelloRechazosMax90),
	}
	elegible := true
	for _, c := range criterios {
		elegible = elegible && c.Cumple
	}
	return AvanceSello{Elegible: elegible, Criterios: criterios}
}

func minimo(nombre string, actual *float64, meta float64) Criterio {
	return Criterio{Nombre: nombre, Actual: actual, Meta: meta, Cumple: actual != nil && *actual >= meta}
}

func maximo(nombre string, actual *float64, meta float64) Criterio {
	return Criterio{Nombre: nombre, Actual: actual, Meta: meta, Cumple: actual != nil && *actual <= meta}
}
