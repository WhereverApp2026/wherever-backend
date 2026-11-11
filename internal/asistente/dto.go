package asistente

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/planes"
)

// MaxFrase es el largo máximo de la frase, en caracteres.
const MaxFrase = 300

// GenerarRequest es el cuerpo de POST /planes/generar. La ubicación sirve para buscar
// candidatos y nunca viaja al modelo. Sin `fecha` el plan empieza a la hora que diga la frase
// o ahora; sin `personas` ni `movilidad` se toman de la frase y de las preferencias.
type GenerarRequest struct {
	Frase     string     `json:"frase" validate:"required,max=300"`
	Lat       *float64   `json:"lat" validate:"required,latitude"`
	Lng       *float64   `json:"lng" validate:"required,longitude"`
	Fecha     *time.Time `json:"fecha"`
	Personas  int        `json:"personas" validate:"omitempty,min=1,max=20"`
	Movilidad string     `json:"movilidad" validate:"omitempty,oneof=a_pie bicicleta auto transporte_publico"`
}

// OtraIdeaRequest es el cuerpo de POST /planes/generar/otra-idea: la misma petición más los
// negocios ya propuestos, que no se repiten.
type OtraIdeaRequest struct {
	GenerarRequest
	Excluir []string `json:"excluir" validate:"required,min=1,max=30,dive,objectid"`
}

// GenerarResponse es el plan sin guardar (la app lo guarda con POST /planes, nv-30). El
// respaldo por reglas (nv-33) responde con esta misma forma y `fuente: reglas`.
type GenerarResponse struct {
	Fuente     string     `json:"fuente"`
	Aviso      string     `json:"aviso"`
	Parametros Parametros `json:"parametros"`
	Plan       PlanDTO    `json:"plan"`
}

// Parametros es lo que se entendió de la frase. El ánimo solo vuelve con el consentimiento
// de estado de ánimo (privacidad.estado_animo).
type Parametros struct {
	Compania       string   `json:"compania"`
	ConNinos       bool     `json:"con_ninos"`
	ConMascota     bool     `json:"con_mascota"`
	PresupuestoMax int      `json:"presupuesto_max"`
	Hora           string   `json:"hora"`
	Animo          string   `json:"animo,omitempty"`
	PalabrasClave  []string `json:"palabras_clave"`
}

// PlanDTO es planes.Plan sin los campos de un plan guardado (id, versión, fechas de control).
type PlanDTO struct {
	Nombre          string            `json:"nombre"`
	Fecha           time.Time         `json:"fecha"`
	Personas        int               `json:"personas"`
	Presupuesto     int               `json:"presupuesto"`
	Movilidad       string            `json:"movilidad"`
	Paradas         []ParadaDTO       `json:"paradas"`
	Traslados       []planes.Traslado `json:"traslados"`
	CostoTotal      int               `json:"costo_total"`
	CostoPorPersona int               `json:"costo_por_persona"`
	Origen          string            `json:"origen"`
}

// ParadaDTO es la parada con los datos públicos del negocio y su «por qué» (nv-23).
type ParadaDTO struct {
	planes.Parada
	Nombre    string `json:"nombre"`
	Categoria string `json:"categoria"`
	PorQue    string `json:"por_que"`
}
