package moderacion

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// ListaQuery filtra la cola. Estado por defecto: pendiente.
type ListaQuery struct {
	Estado string `query:"estado" validate:"omitempty,oneof=pendiente aprobado rechazado"`
	Tipo   string `query:"tipo" validate:"omitempty,max=30"`
	Cursor string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limit  int    `query:"limite" validate:"min=1,max=50"`
}

// NuevaListaQuery trae los valores por defecto.
func NuevaListaQuery() ListaQuery {
	return ListaQuery{Estado: EstadoPendiente, Limit: pagination.DefaultLimit}
}

// DecisionRequest es el cuerpo de PATCH /admin/moderacion/{id}. El motivo es obligatorio al
// rechazar.
type DecisionRequest struct {
	Decision string `json:"decision" validate:"required,oneof=aprobado rechazado"`
	Motivo   string `json:"motivo" validate:"required_if=Decision rechazado,max=500"`
}

// ReglasRequest crea las reglas (POST): todo obligatorio.
type ReglasRequest struct {
	Cuarentena float64  `json:"cuarentena" validate:"required,min=40,max=100"`
	Rechazo    float64  `json:"rechazo" validate:"required,min=40,max=100"`
	Palabras   []string `json:"palabras" validate:"max=200,dive,required,max=60"`
}

// ReglasPatch ajusta las reglas (PATCH): lo que falte se queda igual. Palabras reemplaza la
// lista completa.
type ReglasPatch struct {
	Cuarentena *float64  `json:"cuarentena" validate:"omitempty,min=40,max=100"`
	Rechazo    *float64  `json:"rechazo" validate:"omitempty,min=40,max=100"`
	Palabras   *[]string `json:"palabras" validate:"omitempty,max=200,dive,required,max=60"`
}

// aplicar deja en r lo que trae el ajuste.
func (p ReglasPatch) aplicar(r Reglas) Reglas {
	if p.Cuarentena != nil {
		r.Cuarentena = *p.Cuarentena
	}
	if p.Rechazo != nil {
		r.Rechazo = *p.Rechazo
	}
	if p.Palabras != nil {
		r.Palabras = *p.Palabras
	}
	return r
}

// Resumen es una fila de la lista.
type Resumen struct {
	ID         string    `json:"id"`
	Tipo       string    `json:"tipo"`
	Referencia string    `json:"referencia"`
	Estado     string    `json:"estado"`
	Veredicto  Veredicto `json:"veredicto"`
	Falla      string    `json:"falla,omitempty"`
	PuntajeMax float64   `json:"puntaje_max"`
	CreadoEn   time.Time `json:"creado_en"`
}

// Decision es quién decidió, cuándo y por qué.
type Decision struct {
	Estado     string    `json:"estado"`
	Motivo     string    `json:"motivo,omitempty"`
	RevisorID  string    `json:"revisor_id"`
	DecididoEn time.Time `json:"decidido_en"`
}

// Detalle es un elemento con sus puntajes y su decisión (nil si sigue pendiente).
type Detalle struct {
	Resumen
	Texto       string             `json:"texto,omitempty"`
	ImagenClave string             `json:"imagen_clave,omitempty"`
	Puntajes    map[string]float64 `json:"puntajes"`
	Decision    *Decision          `json:"decision"`
	Version     int                `json:"version"`
}

// ReglasView son las reglas vigentes. Version 0 = por defecto, nunca guardadas.
type ReglasView struct {
	Cuarentena     float64    `json:"cuarentena"`
	Rechazo        float64    `json:"rechazo"`
	Palabras       []string   `json:"palabras"`
	Version        int        `json:"version"`
	PorDefecto     bool       `json:"por_defecto"`
	ActualizadoPor string     `json:"actualizado_por,omitempty"`
	ActualizadoEn  *time.Time `json:"actualizado_en,omitempty"`
}

func nuevoResumen(it Item) Resumen {
	return Resumen{
		ID: it.ID.Hex(), Tipo: it.Tipo, Referencia: it.Referencia, Estado: it.Estado,
		Veredicto: it.Veredicto, Falla: it.Falla, PuntajeMax: maximo(it.Puntajes), CreadoEn: it.CreadoEn,
	}
}

func nuevoDetalle(it Item) Detalle {
	d := Detalle{Resumen: nuevoResumen(it), Texto: it.Texto, ImagenClave: it.ImagenClave, Puntajes: it.Puntajes, Version: it.Version}
	if d.Puntajes == nil {
		d.Puntajes = map[string]float64{}
	}
	if it.RevisorID != nil && it.DecididoEn != nil {
		d.Decision = &Decision{Estado: it.Estado, Motivo: it.Motivo, RevisorID: it.RevisorID.Hex(), DecididoEn: *it.DecididoEn}
	}
	return d
}

func nuevaReglasView(d reglasDoc) ReglasView {
	at := d.ActualizadoEn
	palabras := d.Palabras
	if palabras == nil {
		palabras = []string{}
	}
	return ReglasView{Cuarentena: d.Cuarentena, Rechazo: d.Rechazo, Palabras: palabras, Version: d.Version, ActualizadoPor: d.ActualizadoPor.Hex(), ActualizadoEn: &at}
}

func reglasPorDefectoView() ReglasView {
	r := ReglasPorDefecto()
	return ReglasView{Cuarentena: r.Cuarentena, Rechazo: r.Rechazo, Palabras: r.Palabras, PorDefecto: true}
}
