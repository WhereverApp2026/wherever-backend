// Package consejos da al dueño un consejo del día (nv-75, US-142): un motor de reglas elige un
// hecho con datos agregados del negocio y la IA, si hay cupo, solo lo redacta. Uno por negocio y
// día, guardado en `consejos_negocio` (sin Redis).
package consejos

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Collection guarda un consejo por negocio y día local. No lleva `usuario_id`: no entra en ARCO;
// se borra con la baja del negocio.
const Collection = "consejos_negocio"

// Límites de la spec nv-75.
const (
	Zona            = "America/Mexico_City"
	FormatoDia      = "2006-01-02"
	VidaConsejo     = 30 * 24 * time.Hour
	PlazoGenerando  = time.Minute
	DiasVentana     = 28
	DiasMinimos     = 7
	DiasRecientes   = 14
	MinTextoIA      = 20
	MaxTextoIA      = 160
	CupoPorOmision  = 60
	estadisticasCol = "estadisticas_diarias" // nv-74, se lee por nombre
	resenasCol      = "resenas"              // nv-62
	cuponesCol      = "cupones"              // nv-61
	carteraCol      = "cupones_usuario"      // nv-61
)

// Acciones que la app traduce a una pantalla.
const (
	AccionCrearCupon         = "crear_cupon"
	AccionVerResenas         = "ver_resenas"
	AccionResponderResenas   = "responder_resenas"
	AccionEditarCupon        = "editar_cupon"
	AccionConfigurarReservas = "configurar_reservas"
	AccionCompletarPerfil    = "completar_perfil"
	AccionNinguna            = "ninguna"
)

// Acciones es la lista cerrada del esquema de salida.
var Acciones = []string{AccionCrearCupon, AccionVerResenas, AccionResponderResenas, AccionEditarCupon,
	AccionConfigurarReservas, AccionCompletarPerfil, AccionNinguna}

// Reglas del motor (se registran en el documento y en los logs).
const (
	ReglaFranjaFloja      = "franja_floja"
	ReglaAspectoEnBaja    = "aspecto_en_baja"
	ReglaResenasSinResp   = "resenas_sin_respuesta"
	ReglaCuponSinCanjes   = "cupon_sin_canjes"
	ReglaReservasApagadas = "reservas_apagadas"
	ReglaPerfil           = "completar_perfil"
	ReglaNinguna          = "ninguna"
)

// Fuentes del texto.
const (
	FuenteIA     = "ia"
	FuenteReglas = "reglas"
)

// Estados del documento: `generando` hace de candado (ux_consejo_dia decide quién genera).
const (
	EstadoGenerando = "generando"
	EstadoListo     = "listo"
)

// Consejo es un documento de `consejos_negocio`.
type Consejo struct {
	ID        bson.ObjectID `bson:"_id"`
	NegocioID bson.ObjectID `bson:"negocio_id"`
	Dia       string        `bson:"dia"`
	Texto     string        `bson:"texto"`
	Accion    string        `bson:"accion"`
	Fuente    string        `bson:"fuente"`
	Regla     string        `bson:"regla"`
	Estado    string        `bson:"estado"`
	CreadoEn  time.Time     `bson:"creado_en"`
	ExpiraEn  time.Time     `bson:"expira_en"`
}

// Indexes son los de la spec: uno por negocio y día, y la vida de 30 días.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{Collection: Collection, Models: []mongo.IndexModel{
		{Keys: bson.D{{Key: "negocio_id", Value: 1}, {Key: "dia", Value: 1}}, Options: options.Index().SetName("ux_consejo_dia").SetUnique(true)},
		{Keys: bson.D{{Key: "expira_en", Value: 1}}, Options: options.Index().SetName("ttl_consejos").SetExpireAfterSeconds(0)},
	}}}
}
