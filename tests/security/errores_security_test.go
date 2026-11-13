package security

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/errores"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

const (
	erroresPath      = "/api/v1/errores"
	adminErroresPath = "/api/v1/admin/errores"
	codigoPath       = "/api/v1/personas/codigo/"
)

func appError(message string) map[string]string {
	return map[string]string{"modulo": "pagos", "mensaje": message, "version": "1.4.0"}
}

// nv-64: las tres rutas piden sesión; la lista solo la ve el administrador.
func TestErroresSecurity_AutenticacionYRol(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodPost, erroresPath, appError("x"), "").status)
	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, adminErroresPath, nil, "").status)
	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, codigoPath+"ZZZZZZZZ", nil, "").status)

	user := env.CreateUser(t, "consumidor-errores@wherever.test")
	token := testutil.ValidTokenFor(t, user.ID.Hex(), "jti-consumidor-errores", middleware.RoleConsumer)
	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodGet, adminErroresPath, nil, token).status)
	merchant := testutil.ValidTokenFor(t, user.ID.Hex(), "jti-comerciante-errores", middleware.RoleMerchant)
	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodGet, adminErroresPath, nil, merchant).status)

	admin := env.CreateAdmin(t, "admin-errores@wherever.test")
	adminToken := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-errores-seg", middleware.RoleAdmin)
	assert.Equal(t, http.StatusOK, send(t, app, http.MethodGet, adminErroresPath, nil, adminToken).status)
}

// Una cuenta en eliminación puede reportar (sin RequireActive) pero no buscar por código.
func TestErroresSecurity_CuentaEnEliminacion(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	pending := env.CreateUser(t, "pendiente-errores@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-errores", middleware.RoleConsumer)
	assert.Equal(t, http.StatusAccepted, send(t, app, http.MethodPost, erroresPath, appError("falló"), token).status)
	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodGet, codigoPath+"ZZZZZZZZ", nil, token).status)
}

// Ningún correo, teléfono, tarjeta, token o credencial llega a `errores_app`, ni el id de quien reporta.
func TestErroresSecurity_SinDatosPersonalesNiSecretos(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	user := env.CreateUser(t, "reporta-seg@wherever.test")
	token := testutil.ValidTokenFor(t, user.ID.Hex(), "jti-reporta-seg", middleware.RoleConsumer)
	msg := "fallo con reporta-seg@wherever.test tarjeta 4111 1111 1111 1111 tel +525511112222 " +
		"mongodb://admin:" + "clave-secreta@db.local/x AIzaSyA1234567890abcdefghijklmnopqrstuv " +
		"Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.c2lnbmF0dXJhLWZhbHNh"
	require.Equal(t, http.StatusAccepted, send(t, app, http.MethodPost, erroresPath, appError(msg), token).status)

	raw, err := env.Mongo.DB().Collection(errores.Collection).FindOne(context.Background(), bson.D{}).Raw()
	require.NoError(t, err)
	doc := raw.String()
	for _, secret := range []string{"reporta-seg@wherever.test", "4111", "5511112222", "clave-secreta", "AIzaSy", "eyJ", user.ID.Hex()} {
		assert.NotContains(t, doc, secret)
	}
}

// El perfil por código no trae correo, teléfono ni el código, y no distingue una cuenta
// bloqueada o inactiva de un código inexistente.
func TestPersonasCodigoSecurity_SinDatosPrivadosNiEnumeracion(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	me := env.CreateUser(t, "qr-yo@wherever.test")
	token := testutil.ValidTokenFor(t, me.ID.Hex(), "jti-qr-yo", middleware.RoleConsumer)
	other := env.CreateUser(t, "qr-otra@wherever.test")
	_, err := env.Users.Update(context.Background(), other.ID, bson.D{
		{Key: "telefono_hash", Value: usuarios.HashPhone(env.Cfg.PhoneHMACKey, "+525511112222")},
	})
	require.NoError(t, err)

	found := send(t, app, http.MethodGet, codigoPath+other.FriendCode, nil, token)
	require.Equal(t, http.StatusOK, found.status, found.body)
	for _, secret := range []string{"correo", "telefono", "@wherever.test", other.FriendCode, "codigo_amigo", "estado"} {
		assert.NotContains(t, found.body, secret)
	}

	missing := send(t, app, http.MethodGet, codigoPath+"ZZZZZZZZ", nil, token)
	for _, status := range []string{usuarios.StatusBlocked, usuarios.StatusPendingDeletion} {
		_, err := env.Users.Update(context.Background(), other.ID, bson.D{{Key: "estado", Value: status}})
		require.NoError(t, err)
		hidden := send(t, app, http.MethodGet, codigoPath+other.FriendCode, nil, token)
		assert.Equal(t, missing.status, hidden.status, status)
		assert.Equal(t, http.StatusNotFound, hidden.status, status)
	}
}
