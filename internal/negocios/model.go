package negocios

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Collection es el nombre de la colección (A2/A5; constitución §5: español, plural).
const Collection = "negocios"

// Estados de A5 §1. Solo `aprobado` es visible en el catálogo público.
const (
	StatusDraft    = "borrador"
	StatusReview   = "en_revision"
	StatusApproved = "aprobado"
	StatusRejected = "rechazado"
	StatusPaused   = "pausado"
)

// Planes de A5. La semilla crea todos en `gratis`.
const (
	PlanFree    = "gratis"
	PlanPremium = "premium"
)

// Límites del modelo y de la semilla.
const (
	StepComplete = 6
	MaxFacets    = 50

	MaxImportRows = 100
	MinPhotos     = 3
	MaxPhotos     = 10
	MaxTags       = 20
	MaxServices   = 20
	MaxDiet       = 3
	MaxAllergens  = 9
)

// Location es un GeoJSON Point `[longitud, latitud]`.
type Location struct {
	Type        string    `bson:"type"`
	Coordinates []float64 `bson:"coordinates"`
}

// Direccion es `direccion` de A2 §1.
type Direccion struct {
	Calle     string `bson:"calle" json:"calle"`
	Numero    string `bson:"numero" json:"numero"`
	Colonia   string `bson:"colonia" json:"colonia"`
	Municipio string `bson:"municipio" json:"municipio"`
	Estado    string `bson:"estado" json:"estado"`
	CP        string `bson:"cp" json:"cp"`
}

// Horario es un tramo de un día. Dia usa la convención de Go: 0=domingo, 1=lunes.
type Horario struct {
	Dia     int    `bson:"dia" json:"dia"`
	Abre    string `bson:"abre" json:"abre"`
	Cierra  string `bson:"cierra" json:"cierra"`
	Cerrado bool   `bson:"cerrado" json:"cerrado"`
}

// VentanaMin es una ventana en minutos desde las 00:00. `Hasta` puede superar 1440 para
// cruzar medianoche; lo consume NV-21.
type VentanaMin struct {
	Dia   int `bson:"dia" json:"dia"`
	Desde int `bson:"desde" json:"desde"`
	Hasta int `bson:"hasta" json:"hasta"`
}

// Foto es una URL de la galería. Tipo: fachada, interior o producto. ImagenID es la imagen
// publicada de nv-45 cuando la sube el dueño (nv-55); la semilla no lo lleva.
type Foto struct {
	ImagenID string `bson:"imagen_id,omitempty" json:"-"`
	URL      string `bson:"url" json:"url"`
	Tipo     string `bson:"tipo" json:"tipo"`
	Orden    int    `bson:"orden" json:"orden"`
}

// Accesibilidad sustituye las banderas `accesible` y `mascotas` del stub.
type Accesibilidad struct {
	SillaRuedas bool `bson:"silla_ruedas" json:"silla_ruedas"`
	AptoNinos   bool `bson:"apto_ninos" json:"apto_ninos"`
	PetFriendly bool `bson:"pet_friendly" json:"pet_friendly"`
}

// Aspectos es el desglose de la calificación de A2.
type Aspectos struct {
	Sabor         float64 `bson:"sabor" json:"sabor"`
	Atencion      float64 `bson:"atencion" json:"atencion"`
	Ambiente      float64 `bson:"ambiente" json:"ambiente"`
	Espera        float64 `bson:"espera" json:"espera"`
	PrecioCalidad float64 `bson:"precio_calidad" json:"precio_calidad"`
}

// Calificacion es un objeto, no un flotante suelto.
type Calificacion struct {
	Promedio     float64  `bson:"promedio" json:"promedio"`
	TotalResenas int      `bson:"total_resenas" json:"total_resenas"`
	Aspectos     Aspectos `bson:"aspectos" json:"aspectos"`
}

// Revision es el control de aprobación de A5.
type Revision struct {
	EnviadoEn     *time.Time     `bson:"enviado_en,omitempty" json:"enviado_en,omitempty"`
	RevisadoPor   *bson.ObjectID `bson:"revisado_por,omitempty" json:"revisado_por,omitempty"`
	RevisadoEn    *time.Time     `bson:"revisado_en,omitempty" json:"revisado_en,omitempty"`
	MotivoRechazo string         `bson:"motivo_rechazo,omitempty" json:"motivo_rechazo,omitempty"`
}

// Pausa es la pausa temporal de A5; la semilla no la escribe.
type Pausa struct {
	Desde  *time.Time `bson:"desde,omitempty" json:"desde,omitempty"`
	Hasta  *time.Time `bson:"hasta,omitempty" json:"hasta,omitempty"`
	Motivo string     `bson:"motivo,omitempty" json:"motivo,omitempty"`
}

// Documentos dice qué parte del expediente subió el dueño (nv-55). El expediente vive en
// `negocios_documentos`, cifrado; aquí solo quedan banderas para el paso 6 y la cola.
type Documentos struct {
	INE         bool `bson:"ine" json:"ine"`
	Comprobante bool `bson:"comprobante" json:"comprobante"`
}

// CuentaMascara es la cuenta de depósito sin la CLABE completa (nv-55): la cifrada vive en
// `negocios_documentos` hasta que nv-60 la registre en Stripe.
type CuentaMascara struct {
	Ultimos4 string `bson:"clabe_ultimos4" json:"clabe_ultimos4"`
	Banco    string `bson:"banco" json:"banco"`
	Titular  string `bson:"titular" json:"titular"`
	Estado   string `bson:"estado" json:"estado"`
}

// Business es el documento de `negocios`: A2 §1 más los campos de control de A5 §1 que
// esta spec necesita. Los datos fiscales y de cobro llegan con A5.
type Business struct {
	ID                  bson.ObjectID      `bson:"_id"`
	OwnerID             bson.ObjectID      `bson:"propietario_id"`
	Slug                string             `bson:"slug"`
	Name                string             `bson:"nombre"`
	NormalizedName      string             `bson:"nombre_normalizado"`
	Category            string             `bson:"categoria"`
	Tags                []string           `bson:"etiquetas"`
	Description         string             `bson:"descripcion"`
	PriceLevel          int                `bson:"nivel_precio"`
	AveragePrice        int                `bson:"precio_promedio"`
	Location            Location           `bson:"ubicacion"`
	Address             Direccion          `bson:"direccion"`
	Phone               string             `bson:"telefono,omitempty"`
	Schedules           []Horario          `bson:"horarios"`
	ScheduleMin         []VentanaMin       `bson:"horario_min"`
	Photos              []Foto             `bson:"fotos"`
	Services            []string           `bson:"servicios"`
	Accessibility       Accesibilidad      `bson:"accesibilidad"`
	DietOptions         []string           `bson:"opciones_dieta"`
	Allergens           []string           `bson:"alergenos"`
	AcceptsReservations bool               `bson:"acepta_reservas"`
	Rating              Calificacion       `bson:"calificacion"`
	State               string             `bson:"estado"`
	Plan                string             `bson:"plan"`
	Verified            bool               `bson:"verificado"`
	ExcellenceSeal      bool               `bson:"sello_excelencia"`
	Coupons             int                `bson:"cupones_activos"`
	Traffic             [][]int            `bson:"afluencia,omitempty"`
	HighlightedUntil    *time.Time         `bson:"destacado_hasta,omitempty"`
	Paused              *Pausa             `bson:"pausado,omitempty"`
	DeletedAt           *time.Time         `bson:"eliminado_en,omitempty"`
	QRCheckin           string             `bson:"qr_checkin"`
	Step                int                `bson:"paso_alta"`
	Revision            Revision           `bson:"revision"`
	Documentos          Documentos         `bson:"documentos"`
	CuentaDeposito      *CuentaMascara     `bson:"cuenta_deposito,omitempty"`
	ReservasConfig      *ReservasConfig    `bson:"reservas_config,omitempty"`
	CambiosPendientes   *CambiosPendientes `bson:"cambios_pendientes,omitempty"`
	Sello               *Sello             `bson:"sello,omitempty"`
	Cobros              *Cobros            `bson:"cobros,omitempty"`
	AvisosPendientes    []string           `bson:"avisos_pendientes,omitempty"`
	Version             int                `bson:"version"`
	CreatedAt           time.Time          `bson:"creado_en"`
	UpdatedAt           time.Time          `bson:"actualizado_en"`
}
