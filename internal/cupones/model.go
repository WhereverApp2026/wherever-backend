// Package cupones son los cupones de los negocios (nv-61, hoja J6): el dueño los crea y los
// valida; la persona los guarda en su cartera y los canjea con un código de un solo uso.
package cupones

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Colecciones de J6.
const (
	Collection         = "cupones"
	CarteraCollection  = "cupones_usuario"
	MetricasCollection = "cupones_metricas"
)

// Tipos de cupón.
const (
	TipoPorcentaje = "porcentaje"
	TipoMonto      = "monto"
	TipoDosPorUno  = "dos_por_uno"
	TipoRegalo     = "regalo"
)

// Estados del cupón.
const (
	EstadoProgramado = "programado"
	EstadoActivo     = "activo"
	EstadoPausado    = "pausado"
	EstadoAgotado    = "agotado"
	EstadoVencido    = "vencido"
)

// Estados de la cartera.
const (
	CarteraGuardado   = "guardado"
	CarteraEnCanje    = "en_canje"
	CarteraCanjeado   = "canjeado"
	CarteraVencido    = "vencido"
	duracionRelampago = 3 * time.Hour
	// VidaCodigo es lo que vive el código de canje y la ventana del reintento del negocio.
	VidaCodigo = 5 * time.Minute
)

// Horas es la ventana del día en `HH:MM` de México; `Hasta` menor que `Desde` cruza medianoche.
type Horas struct {
	Desde string `bson:"desde" json:"desde" validate:"required,hhmm"`
	Hasta string `bson:"hasta" json:"hasta" validate:"required,hhmm"`
}

// Location es un punto GeoJSON (copia de la del negocio, para `ix_cupones_cerca`).
type Location struct {
	Type        string    `bson:"type"`
	Coordinates []float64 `bson:"coordinates"`
}

// Cupon es un documento de `cupones` (J6 §1).
type Cupon struct {
	ID                bson.ObjectID `bson:"_id"`
	NegocioID         bson.ObjectID `bson:"negocio_id"`
	Titulo            string        `bson:"titulo"`
	Tipo              string        `bson:"tipo"`
	Valor             int64         `bson:"valor"`
	Condiciones       string        `bson:"condiciones"`
	VigenciaInicio    time.Time     `bson:"vigencia_inicio"`
	VigenciaFin       time.Time     `bson:"vigencia_fin"`
	Relampago         bool          `bson:"relampago"`
	DiasValidos       []int         `bson:"dias_validos"`
	HorasValidas      *Horas        `bson:"horas_validas,omitempty"`
	LimiteUsos        int           `bson:"limite_usos"`
	UsosPorUsuario    int           `bson:"usos_por_usuario"`
	UsosActuales      int           `bson:"usos_actuales"`
	Ubicacion         Location      `bson:"ubicacion"`
	Categoria         string        `bson:"categoria"`
	NegocioNombre     string        `bson:"negocio_nombre"`
	Estado            string        `bson:"estado"`
	PausadoPorNegocio bool          `bson:"pausado_por_negocio,omitempty"`
	Vistas            int           `bson:"vistas"`
	Guardados         int           `bson:"guardados"`
	Version           int           `bson:"version"`
	CreadoEn          time.Time     `bson:"creado_en"`
	ActualizadoEn     time.Time     `bson:"actualizado_en"`
}

// Vigente dice si el cupón se puede mostrar y usar ahora (estado y fechas; no días ni horas).
func (c Cupon) Vigente(now time.Time) bool {
	return (c.Estado == EstadoActivo || c.Estado == EstadoProgramado) && !now.Before(c.VigenciaInicio) && now.Before(c.VigenciaFin)
}

// Resumen es la copia del cupón en la cartera: no cambia si el negocio edita el título.
type Resumen struct {
	Titulo             string `bson:"titulo"`
	Tipo               string `bson:"tipo"`
	Valor              int64  `bson:"valor"`
	NegocioNombre      string `bson:"negocio_nombre"`
	ResumenNormalizado string `bson:"resumen_normalizado"`
}

// Cartera es un documento de `cupones_usuario` (J6 §2).
type Cartera struct {
	ID               bson.ObjectID  `bson:"_id"`
	UsuarioID        bson.ObjectID  `bson:"usuario_id"`
	CuponID          bson.ObjectID  `bson:"cupon_id"`
	NegocioID        bson.ObjectID  `bson:"negocio_id"`
	Resumen          Resumen        `bson:"resumen"`
	Estado           string         `bson:"estado"`
	VenceEn          time.Time      `bson:"vence_en"`
	CodigoCanje      string         `bson:"codigo_canje,omitempty"`
	CanjeExpiraEn    *time.Time     `bson:"canje_expira_en,omitempty"`
	CodigoCanjeUsado string         `bson:"codigo_canje_usado,omitempty"`
	CanjeadoEn       *time.Time     `bson:"canjeado_en,omitempty"`
	ValidadoPor      *bson.ObjectID `bson:"validado_por,omitempty"`
	CreadoEn         time.Time      `bson:"creado_en"`
	ActualizadoEn    time.Time      `bson:"actualizado_en"`
}

// Metrica es un documento de `cupones_metricas`: un cupón en un día UTC (US-097).
type Metrica struct {
	CuponID   bson.ObjectID `bson:"cupon_id"`
	Dia       string        `bson:"dia"`
	Vistas    int           `bson:"vistas"`
	Guardados int           `bson:"guardados"`
	Canjes    int           `bson:"canjes"`
}
