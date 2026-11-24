// Package grupos reúne hasta 12 personas alrededor de un plan (nv-37, hojas J3 y A4):
// modelo, repositorio, service, handler y ARCO, y el enlace de invitación con su vista
// pública (nv-38). Las votaciones son de nv-39.
package grupos

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/planes"
)

// Collection guarda los grupos (J3).
const Collection = "grupos"

// MaxMembers es el tope de integrantes que no declinaron, contando al organizador.
const MaxMembers = 12

// Roles dentro del grupo.
const (
	RoleOrganizer = "organizador"
	RoleMember    = "integrante"
)

// Estados de un integrante.
const (
	StatusInvited   = "invitado"
	StatusConfirmed = "confirmado"
	StatusDeclined  = "declinado"
)

// Member es una persona del grupo. El organizador también va aquí, confirmado.
type Member struct {
	UserID    bson.ObjectID  `bson:"usuario_id"`
	Role      string         `bson:"rol"`
	Status    string         `bson:"estado"`
	InvitedBy *bson.ObjectID `bson:"invitado_por,omitempty"`
	Since     time.Time      `bson:"desde"`
	// SharedLocation es la ubicación que la persona compartió para el punto de encuentro
	// (nv-69), redondeada a 3 decimales (~110 m) y vigente hasta LocationExpires. Nunca sale
	// en ninguna respuesta.
	SharedLocation  *Coords    `bson:"ubicacion_compartida,omitempty"`
	LocationExpires *time.Time `bson:"ubicacion_vence,omitempty"`
	// Permission es `permiso_plan` (nv-89): `editar` deja cambiar el plan; sin el campo, `ver`.
	Permission string `bson:"permiso_plan,omitempty"`
}

// PlanPermission es el permiso efectivo sobre el plan: el organizador siempre edita; un
// integrante, lo que tenga guardado (`ver` por omisión).
func (m Member) PlanPermission() string {
	if m.Role == RoleOrganizer || m.Permission == planes.PermissionEdit {
		return planes.PermissionEdit
	}
	return planes.PermissionView
}

// Coords es una posición `lat`/`lng`.
type Coords struct {
	Lat float64 `bson:"lat"`
	Lng float64 `bson:"lng"`
}

// MeetingPoint es el punto de encuentro del grupo (nv-69).
type MeetingPoint struct {
	Name       string         `bson:"nombre"`
	Location   Coords         `bson:"ubicacion"`
	At         time.Time      `bson:"hora"`
	ProposedBy bson.ObjectID  `bson:"propuesto_por"`
	BusinessID *bson.ObjectID `bson:"negocio_id,omitempty"`
}

// Surprise es el modo sorpresa (nv-69): `para` no ve las paradas hasta `revelar_en`. Se
// decide al leer; no hay trabajo programado.
type Surprise struct {
	Active   bool          `bson:"activo"`
	For      bson.ObjectID `bson:"para_usuario_id"`
	RevealAt time.Time     `bson:"revelar_en"`
}

// HidesFrom dice si la sorpresa le oculta las paradas a userID en este momento.
func (s *Surprise) HidesFrom(userID bson.ObjectID, now time.Time) bool {
	return s != nil && s.Active && s.For == userID && now.Before(s.RevealAt)
}

// Visible dice si la ubicación compartida sigue vigente.
func (m Member) Visible(now time.Time) bool {
	return m.SharedLocation != nil && m.LocationExpires != nil && now.Before(*m.LocationExpires)
}

// Group es un documento de `grupos`. `ocupados` cuenta a los integrantes que no declinaron
// y solo cambia junto con `integrantes`, en la misma operación.
type Group struct {
	ID          bson.ObjectID `bson:"_id"`
	PlanID      bson.ObjectID `bson:"plan_id"`
	OrganizerID bson.ObjectID `bson:"organizador_id"`
	Members     []Member      `bson:"integrantes"`
	Occupied    int           `bson:"ocupados"`
	// CodeHash es el SHA-256 del código del enlace (nv-38); el código nunca se guarda.
	CodeHash    string     `bson:"codigo_hash,omitempty"`
	CodeExpires *time.Time `bson:"codigo_vence,omitempty"`
	// CodePermission es el permiso que recibe quien entra por el enlace (nv-89); sin el
	// campo, `ver`.
	CodePermission string `bson:"codigo_permiso,omitempty"`
	// LastMessage es el último mensaje del chat (nv-57); solo lo ven los confirmados.
	LastMessage *LastMessage `bson:"ultimo_mensaje,omitempty"`
	// Name es opcional; sin él, la vista usa el nombre del plan (nv-69).
	Name string `bson:"nombre,omitempty"`
	// Version sube con cada cambio del organizador (If-Match, nv-69); los grupos de antes no
	// la tienen y valen 1.
	Version      int           `bson:"version,omitempty"`
	DeletedAt    *time.Time    `bson:"eliminado_en,omitempty"`
	MeetingPoint *MeetingPoint `bson:"punto_encuentro,omitempty"`
	Surprise     *Surprise     `bson:"sorpresa,omitempty"`
	CreatedAt    time.Time     `bson:"creado_en"`
	UpdatedAt    time.Time     `bson:"actualizado_en"`
}

// CurrentVersion es la versión para If-Match (1 en los grupos de antes de nv-69).
func (g Group) CurrentVersion() int {
	if g.Version == 0 {
		return 1
	}
	return g.Version
}

// LastMessage resume el último mensaje del chat: texto recortado, autor (vacío en avisos del
// sistema) y fecha. Lo escribe chat con SetLastMessage y lo limpia al borrar ese mensaje.
type LastMessage struct {
	MessageID bson.ObjectID  `bson:"mensaje_id"`
	Text      string         `bson:"texto"`
	AuthorID  *bson.ObjectID `bson:"autor_id,omitempty"`
	At        time.Time      `bson:"fecha"`
}

// Member devuelve al integrante userID, si está.
func (g Group) Member(userID bson.ObjectID) (Member, bool) {
	for _, m := range g.Members {
		if m.UserID == userID {
			return m, true
		}
	}
	return Member{}, false
}
