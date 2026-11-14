// Package expediente guarda el expediente del comerciante (spec nv-55, hoja A5): INE,
// comprobante de domicilio, datos fiscales y CLABE, siempre cifrados con la capa de nv-48
// (`DATA_KEYS`). Los archivos van cifrados al prefijo `privado/` del bucket de nv-44, que
// CloudFront no sirve. Cada lectura del administrador queda en `accesos_expediente`. Nada de
// esto viaja con el documento de `negocios`, que leen `/lugares` y la caché.
package expediente

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Colecciones (constitución §5: español, plural).
const (
	Collection         = "negocios_documentos"
	AccesosCollection  = "accesos_expediente"
	ExportName         = "negocios_expediente"
	prefijoPrivado     = "privado/negocios/"
	extensionCifrada   = ".enc"
	vidaBitacoraSegund = int32(5 * 365 * 24 * 60 * 60)
)

// Tipos del expediente. Los tres primeros son archivos; los dos últimos, JSON cifrado.
const (
	TipoINEFrente   = "ine_frente"
	TipoINEReverso  = "ine_reverso"
	TipoComprobante = "comprobante_domicilio"
	TipoFiscales    = "datos_fiscales"
	TipoCuenta      = "cuenta_deposito"
)

// Estados de un registro del expediente.
const (
	EstadoVigente   = "vigente"
	EstadoBorrado   = "borrado"
	EstadoBloqueado = "bloqueado" // datos fiscales tras la baja: solo el administrador, con motivo
)

// Plazos de conservación (spec nv-55, requisito 10).
const (
	RetencionINE      = 90 * 24 * time.Hour
	RetencionFiscales = 5 * 365 * 24 * time.Hour
)

// TiposArchivo son los documentos que se suben como archivo.
var TiposArchivo = []string{TipoINEFrente, TipoINEReverso, TipoComprobante}

// TiposLectura son los tipos que se pueden leer (archivos y JSON).
var TiposLectura = []string{TipoINEFrente, TipoINEReverso, TipoComprobante, TipoFiscales, TipoCuenta}

// Documento es un registro de `negocios_documentos`. `llave_s3` apunta al objeto cifrado;
// `datos_cifrados` guarda el JSON cifrado de fiscales o cuenta. Nunca hay nada en claro.
type Documento struct {
	ID              bson.ObjectID `bson:"_id"`
	NegocioID       bson.ObjectID `bson:"negocio_id"`
	PropietarioID   bson.ObjectID `bson:"propietario_id"`
	Tipo            string        `bson:"tipo"`
	LlaveS3         string        `bson:"llave_s3,omitempty"`
	ContentType     string        `bson:"content_type,omitempty"`
	Tamano          int64         `bson:"tamano"`
	DatosCifrados   string        `bson:"datos_cifrados,omitempty"`
	SubidoEn        time.Time     `bson:"subido_en"`
	Estado          string        `bson:"estado"`
	BorrarDespuesDe *time.Time    `bson:"borrar_despues_de,omitempty"`
	ConservarHasta  *time.Time    `bson:"conservar_hasta,omitempty"`
	Version         int           `bson:"version"`
}

// Acceso es una fila de `accesos_expediente`: quién leyó qué, cuándo y por qué. No se borra con
// la cuenta (es la prueba de quién leyó); el índice TTL la quita a los 5 años.
type Acceso struct {
	ID        bson.ObjectID `bson:"_id"`
	AdminID   bson.ObjectID `bson:"admin_id"`
	NegocioID bson.ObjectID `bson:"negocio_id"`
	Tipo      string        `bson:"tipo"`
	Motivo    string        `bson:"motivo"`
	IP        string        `bson:"ip"`
	RequestID string        `bson:"request_id"`
	LeidoEn   time.Time     `bson:"leido_en"`
}

// DatosFiscales son los datos de facturación de A5 §1 (`datos_fiscales`).
type DatosFiscales struct {
	RFC           string `json:"rfc"`
	RazonSocial   string `json:"razon_social"`
	RegimenFiscal string `json:"regimen_fiscal"`
	UsoCFDI       string `json:"uso_cfdi"`
	CPFiscal      string `json:"cp_fiscal"`
}

// Cuenta es la cuenta de depósito completa (cifrada hasta nv-60).
type Cuenta struct {
	CLABE   string `json:"clabe"`
	Banco   string `json:"banco"`
	Titular string `json:"titular"`
}

// Indexes son los índices de las dos colecciones. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{
		{Collection: Collection, Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "negocio_id", Value: 1}, {Key: "tipo", Value: 1}}, Options: options.Index().SetName("ux_documento").SetUnique(true)},
			{Keys: bson.D{{Key: "borrar_despues_de", Value: 1}}, Options: options.Index().SetName("ix_borrar").
				SetPartialFilterExpression(bson.D{{Key: "estado", Value: EstadoVigente}})},
			{Keys: bson.D{{Key: "conservar_hasta", Value: 1}}, Options: options.Index().SetName("ix_conservar").
				SetPartialFilterExpression(bson.D{{Key: "estado", Value: EstadoBloqueado}})},
			{Keys: bson.D{{Key: "propietario_id", Value: 1}}, Options: options.Index().SetName("ix_propietario")},
		}},
		{Collection: AccesosCollection, Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "negocio_id", Value: 1}, {Key: "leido_en", Value: -1}}, Options: options.Index().SetName("ix_acceso_negocio")},
			{Keys: bson.D{{Key: "leido_en", Value: 1}}, Options: options.Index().SetName("ttl_acceso").SetExpireAfterSeconds(vidaBitacoraSegund)},
		}},
	}
}
