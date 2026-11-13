// Package grupos reúne hasta 12 personas alrededor de un plan (nv-37, hojas J3 y A4):
// modelo, repositorio, service, handler y ARCO, y el enlace de invitación con su vista
// pública (nv-38). Las votaciones son de nv-39.
package grupos

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
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
	// LastMessage es el último mensaje del chat (nv-57); solo lo ven los confirmados.
	LastMessage *LastMessage `bson:"ultimo_mensaje,omitempty"`
	CreatedAt   time.Time    `bson:"creado_en"`
	UpdatedAt   time.Time    `bson:"actualizado_en"`
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
