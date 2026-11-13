package grupos

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

func ids(n int) []string {
	out := make([]string, 0, n)
	for range n {
		out = append(out, bson.NewObjectID().Hex())
	}
	return out
}

func TestDTO_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	one := bson.NewObjectID().Hex()
	cases := []struct {
		name string
		dto  any
		ok   bool
	}{
		{"crear sin invitados", CreateRequest{PlanID: one}, true},
		{"crear con 11 invitados", CreateRequest{PlanID: one, Invited: ids(11)}, true},
		{"crear con 12 invitados", CreateRequest{PlanID: one, Invited: ids(12)}, false},
		{"crear sin plan", CreateRequest{}, false},
		{"plan mal formado", CreateRequest{PlanID: "xyz"}, false},
		{"invitar a uno", InviteRequest{Users: []string{one}}, true},
		{"invitar a nadie", InviteRequest{Users: []string{}}, false},
		{"invitar repetido", InviteRequest{Users: []string{one, one}}, false},
		{"invitar a 12", InviteRequest{Users: ids(12)}, false},
		{"id que no es hexadecimal", InviteRequest{Users: []string{strings.Repeat("z", 24)}}, false},
		{"confirmar", RespondRequest{Status: StatusConfirmed}, true},
		{"declinar", RespondRequest{Status: StatusDeclined}, true},
		{"estado invitado no se elige", RespondRequest{Status: StatusInvited}, false},
		{"lista por defecto", NewListQuery(), true},
		{"límite 51", ListQuery{Limit: 51}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := v.Struct(c.dto)
			if c.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func sampleGroup() (Group, bson.ObjectID, bson.ObjectID, bson.ObjectID) {
	org, yes, no := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	now := time.Date(2026, 11, 12, 18, 0, 0, 0, time.UTC)
	return Group{
		ID: bson.NewObjectID(), PlanID: bson.NewObjectID(), OrganizerID: org, Occupied: 2,
		Members: []Member{
			{UserID: org, Role: RoleOrganizer, Status: StatusConfirmed, Since: now},
			{UserID: yes, Role: RoleMember, Status: StatusConfirmed, Since: now},
			{UserID: no, Role: RoleMember, Status: StatusDeclined, Since: now},
		},
	}, org, yes, no
}

func TestNewView_OrganizadorVeATodos(t *testing.T) {
	g, org, yes, _ := sampleGroup()
	people := map[bson.ObjectID]usuarios.User{yes: {ID: yes, Name: "Ana", Username: "ana"}}
	v := newView(g, org, people)
	assert.Equal(t, RoleOrganizer, v.MyRole)
	assert.Len(t, v.Members, 3)
	require.NotNil(t, v.Summary.Declined)
	assert.Equal(t, 1, *v.Summary.Declined)
	assert.Equal(t, 2, v.Summary.Confirmed)
	assert.Equal(t, Seats{Max: 12, Occupied: 2}, v.Seats)
	require.NotNil(t, v.Members[1].Person)
	assert.Equal(t, "ana", v.Members[1].Person.Username)
	assert.Nil(t, v.Members[0].Person, "sin cuenta activa no sale la persona")
}

func TestNewView_IntegranteNoVeAQuienDeclino(t *testing.T) {
	g, _, yes, no := sampleGroup()
	v := newView(g, yes, nil)
	assert.Equal(t, StatusConfirmed, v.MyStatus)
	assert.Len(t, v.Members, 2)
	assert.Nil(t, v.Summary.Declined)
	for _, m := range v.Members {
		assert.NotEqual(t, no.Hex(), m.UserID)
	}
	// Quien declinó sí se ve a sí mismo.
	mine := newView(g, no, nil)
	assert.Equal(t, StatusDeclined, mine.MyStatus)
	assert.Len(t, mine.Members, 3)
}
