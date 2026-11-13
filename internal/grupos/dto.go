package grupos

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/personas"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// CreateRequest es el cuerpo de POST /grupos. `invitados` es opcional.
type CreateRequest struct {
	PlanID  string   `json:"plan_id" validate:"required,len=24,hexadecimal"`
	Invited []string `json:"invitados" validate:"omitempty,max=11,unique,dive,len=24,hexadecimal"`
}

// InviteRequest es el cuerpo de POST /grupos/{id}/integrantes y /planes/{id}/invitaciones.
type InviteRequest struct {
	Users []string `json:"usuarios" validate:"required,min=1,max=11,unique,dive,len=24,hexadecimal"`
}

// RespondRequest es el cuerpo de PATCH /grupos/{id}/integrantes/yo.
type RespondRequest struct {
	Status string `json:"estado" validate:"required,oneof=confirmado declinado"`
}

// ListQuery es GET /grupos.
type ListQuery struct {
	Cursor string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limit  int    `query:"limite" validate:"min=1,max=50"`
}

// NewListQuery trae el límite por defecto (20).
func NewListQuery() ListQuery { return ListQuery{Limit: pagination.DefaultLimit} }

// MemberView es un integrante con lo público de su cuenta. Una cuenta que ya no está activa
// sale sin persona (no se revela nada de ella).
type MemberView struct {
	Person *personas.Person `json:"persona,omitempty"`
	UserID string           `json:"usuario_id"`
	Role   string           `json:"rol"`
	Status string           `json:"estado"`
	Since  time.Time        `json:"desde"`
}

// Seats es el cupo del grupo.
type Seats struct {
	Max      int `json:"max"`
	Occupied int `json:"ocupados"`
}

// Summary cuenta a los integrantes por estado; los declinados solo los ve el organizador.
type Summary struct {
	Confirmed int  `json:"confirmados"`
	Invited   int  `json:"invitados"`
	Declined  *int `json:"declinados,omitempty"`
}

// View es un grupo visto por quien llama.
type View struct {
	ID          string       `json:"id"`
	PlanID      string       `json:"plan_id"`
	OrganizerID string       `json:"organizador_id"`
	MyRole      string       `json:"mi_rol"`
	MyStatus    string       `json:"mi_estado"`
	Seats       Seats        `json:"cupo"`
	Summary     Summary      `json:"resumen"`
	Members     []MemberView `json:"integrantes"`
	CreatedAt   time.Time    `json:"creado_en"`
	UpdatedAt   time.Time    `json:"actualizado_en"`
}

// newView arma la vista de `me`. El organizador ve a todos; los demás no ven a quien
// declinó (requisito 4).
func newView(g Group, me bson.ObjectID, people map[bson.ObjectID]usuarios.User) View {
	mine, _ := g.Member(me)
	organizer := mine.Role == RoleOrganizer
	v := View{
		ID: g.ID.Hex(), PlanID: g.PlanID.Hex(), OrganizerID: g.OrganizerID.Hex(),
		MyRole: mine.Role, MyStatus: mine.Status,
		Seats:   Seats{Max: MaxMembers, Occupied: g.Occupied},
		Members: []MemberView{}, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt,
	}
	declined := 0
	for _, m := range g.Members {
		switch m.Status {
		case StatusConfirmed:
			v.Summary.Confirmed++
		case StatusInvited:
			v.Summary.Invited++
		case StatusDeclined:
			declined++
			if !organizer && m.UserID != me {
				continue
			}
		}
		v.Members = append(v.Members, memberView(m, people))
	}
	if organizer {
		v.Summary.Declined = &declined
	}
	return v
}

func memberView(m Member, people map[bson.ObjectID]usuarios.User) MemberView {
	out := MemberView{UserID: m.UserID.Hex(), Role: m.Role, Status: m.Status, Since: m.Since}
	if u, ok := people[m.UserID]; ok {
		out.Person = &personas.Person{ID: u.ID.Hex(), Name: u.Name, Username: u.Username, PhotoURL: u.PhotoURL}
	}
	return out
}

func invalid(field, message string) error {
	return apperr.Validation(message, apperr.FieldError{Field: field, Message: message})
}
