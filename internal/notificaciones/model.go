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
)

// Tipos de aviso (spec nv-40).
const (
	KindFriendRequest  = "solicitud_amistad"
	KindFriendAccepted = "amistad_aceptada"
	KindPlanInvite     = "invitacion_plan"
	KindVoteOpened     = "votacion_abierta"
	KindVoteClosed     = "votacion_cerrada"
	// KindGroupMessage avisa que hay mensajes nuevos en el chat de un grupo (nv-57); uno por
	// grupo y persona cada 5 minutos, sin texto ni autor.
	KindGroupMessage = "mensaje_grupo"
)

// Límites de la spec.
const (
	MaxDevices = 5
	// Retention es la vida de un aviso: el índice TTL lo borra después.
	Retention = 90 * 24 * time.Hour
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
	LastUsed  time.Time     `bson:"ultimo_uso"`
	CreatedAt time.Time     `bson:"creado_en"`
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
