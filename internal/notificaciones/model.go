// Package notificaciones es el centro de avisos dentro de la app y el registro de teléfonos
// (nv-40, hoja A6). Los módulos avisan con un Notifier; el push por FCM llega con nv-41.
package notificaciones

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Colecciones (A6).
const (
	Collection        = "notificaciones"
	DevicesCollection = "dispositivos"
	// GroupedCollection reserva el siguiente aviso agrupado de cada persona y objeto (nv-83).
	GroupedCollection = "notificaciones_agrupadas"
)

// Tipos de aviso (spec nv-40).
const (
	KindFriendRequest  = "solicitud_amistad"
	KindFriendAccepted = "amistad_aceptada"
	KindPlanInvite     = "invitacion_plan"
	KindVoteOpened     = "votacion_abierta"
	KindVoteClosed     = "votacion_cerrada"
	// KindBusinessReviewed avisa al dueño que su negocio se aprobó o rechazó (nv-55).
	KindBusinessReviewed = "negocio_revisado"
	// KindGroupMessage avisa que hay mensajes nuevos en el chat de un grupo (nv-57); uno por
	// grupo y persona cada 5 minutos, sin texto ni autor.
	KindGroupMessage = "mensaje_grupo"
	// Avisos de momentos (nv-63): solo el id del momento, nunca la imagen ni el texto.
	KindMomentTagged   = "etiqueta_momento"
	KindMomentThanked  = "momento_agradecido"
	KindMomentFeatured = "momento_destacado"
	// Avisos de reservas (nv-73), canal `reservas`: solo ids de la reserva, el negocio y el
	// mensaje; la app pide el texto con GET /reservas/{id}.
	KindReservaMensaje      = "reserva_mensaje"
	KindReservaRecordatorio = "reserva_recordatorio"
	// Avisos sociales (nv-83), uno por persona, tipo y objeto cada GroupEvery: solo el tipo y
	// el id del momento, plan o reseña, nunca quién ni el texto.
	KindLike         = "me_gusta"
	KindComment      = "comentario"
	KindReviewUseful = "resena_util"
	// KindDirectMessage es el mensaje directo de un amigo (nv-87), agrupado por conversación.
	KindDirectMessage = "mensaje_directo"
)

// Límites de la spec.
const (
	MaxDevices = 5
	// Retention es la vida de un aviso: el índice TTL lo borra después.
	Retention = 90 * 24 * time.Hour
	// GroupEvery es el lapso de los avisos agrupados (nv-83): un toque más dentro de él no avisa.
	GroupEvery = 10 * time.Minute
)

// PlatformAndroid es la única plataforma con push por ahora (sin iOS, nv-41).
const PlatformAndroid = "android"

// Notification es un documento de `notificaciones`. `datos` solo lleva ids (de la persona,
// el grupo, la votación…), nunca nombres ni texto: la app arma el mensaje.
type Notification struct {
	ID        bson.ObjectID     `bson:"_id"`
	UserID    bson.ObjectID     `bson:"usuario_id"`
	Kind      string            `bson:"tipo"`
	Data      map[string]string `bson:"datos"`
	ReadAt    *time.Time        `bson:"leida_en,omitempty"`
	CreatedAt time.Time         `bson:"creada_en"`
	ExpiresAt time.Time         `bson:"expira_en"`
}

// Device es un teléfono registrado. El token de FCM se guarda porque hace falta para enviar
// (nv-41); `token_hash` (SHA-256) es la llave única para buscarlo sin compararlo en claro.
type Device struct {
	ID        bson.ObjectID `bson:"_id"`
	UserID    bson.ObjectID `bson:"usuario_id"`
	Token     string        `bson:"token"`
	TokenHash string        `bson:"token_hash"`
	Platform  string        `bson:"plataforma"`
	// AppVersion y Channels los ajusta PUT /dispositivos/{id} (nv-71). Channels nil = todos.
	AppVersion string    `bson:"app_version,omitempty"`
	Channels   *[]string `bson:"canales_activos,omitempty"`
	LastUsed   time.Time `bson:"ultimo_uso"`
	CreatedAt  time.Time `bson:"creado_en"`
}

// Canales de notificación del teléfono (nv-71), los mismos de `preferencias.notificaciones`.
const (
	ChannelBookings   = "reservas"
	ChannelCoupons    = "cupones"
	ChannelGroups     = "grupos"
	ChannelReviews    = "resenas"
	ChannelPromotions = "promociones"
)

// Allows dice si el teléfono deja pasar el canal. `reservas` y los avisos sin canal siempre
// pasan: la hoja A1 dice que las reservas no se apagan del todo.
func (d Device) Allows(channel string) bool {
	if d.Channels == nil || channel == "" || channel == ChannelBookings {
		return true
	}
	for _, c := range *d.Channels {
		if c == channel {
			return true
		}
	}
	return false
}

// Notifier es lo que usan los demás módulos para avisar. Avisar nunca cambia la respuesta de
// la acción que lo originó: los errores se registran y se tragan aquí.
type Notifier interface {
	Emit(ctx context.Context, to bson.ObjectID, kind string, data map[string]string)
}

// Noop no avisa; es el valor por defecto de los módulos que aún no lo reciben.
type Noop struct{}

// Emit no hace nada.
func (Noop) Emit(context.Context, bson.ObjectID, string, map[string]string) {}
