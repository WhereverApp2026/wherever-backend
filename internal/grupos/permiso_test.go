package grupos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func TestMember_PlanPermission(t *testing.T) {
	assert.Equal(t, planes.PermissionEdit, Member{Role: RoleOrganizer}.PlanPermission(), "el organizador siempre edita")
	assert.Equal(t, planes.PermissionView, Member{Role: RoleMember}.PlanPermission(), "sin campo = ver")
	assert.Equal(t, planes.PermissionEdit, Member{Role: RoleMember, Permission: planes.PermissionEdit}.PlanPermission())
	assert.Equal(t, planes.PermissionView, Member{Role: RoleMember, Permission: "admin"}.PlanPermission(), "lo desconocido no edita")
}

func TestShareRequest_PermisoPorOmisionVer(t *testing.T) {
	assert.Equal(t, planes.PermissionView, ShareRequest{}.permission())
	assert.Equal(t, planes.PermissionEdit, ShareRequest{Permission: "editar"}.permission())
}

func TestDTO_Permisos(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	assert.NoError(t, v.Struct(ShareRequest{}))
	assert.NoError(t, v.Struct(ShareRequest{Permission: "editar"}))
	assert.Error(t, v.Struct(ShareRequest{Permission: "admin"}))
	assert.NoError(t, v.Struct(PermissionRequest{Permission: "ver"}))
	assert.Error(t, v.Struct(PermissionRequest{}), "permiso_plan es obligatorio")
	assert.Error(t, v.Struct(PermissionRequest{Permission: "dueno"}))
}

func TestMemberView_LlevaElPermiso(t *testing.T) {
	g := Group{ID: bson.NewObjectID(), Members: []Member{
		{UserID: bson.NewObjectID(), Role: RoleOrganizer, Status: StatusConfirmed},
		{UserID: bson.NewObjectID(), Role: RoleMember, Status: StatusConfirmed, Permission: planes.PermissionEdit},
		{UserID: bson.NewObjectID(), Role: RoleMember, Status: StatusInvited},
	}}
	v := newView(g, g.Members[0].UserID, nil, g.CreatedAt)
	require.Len(t, v.Members, 3)
	assert.Equal(t, planes.PermissionEdit, v.Members[0].Permission)
	assert.Equal(t, planes.PermissionEdit, v.Members[1].Permission)
	assert.Equal(t, planes.PermissionView, v.Members[2].Permission)
}
