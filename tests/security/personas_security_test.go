package security

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

const buscarPath = "/api/v1/personas/buscar?q="

// La búsqueda (nv-35) nunca devuelve correo ni teléfono, y un teléfono no buscable responde
// igual que uno no registrado: mismo estado, mismos bytes.
func TestPersonasSecurity_SinDatosPrivadosNiEnumeracion(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	ctx := context.Background()
	me := env.CreateUser(t, "busca-seg@wherever.test")
	token := testutil.ValidTokenFor(t, me.ID.Hex(), "jti-busca-seg", middleware.RoleConsumer)

	found := env.CreateUser(t, "encontrada-seg@wherever.test")
	_, err := env.Users.Update(ctx, found.ID, bson.D{
		{Key: "nombre_usuario", Value: "seg_encontrada"},
		{Key: "telefono_hash", Value: usuarios.HashPhone(env.Cfg.PhoneHMACKey, "+525511112222")},
		{Key: "privacidad.buscable_por_telefono", Value: true},
	})
	require.NoError(t, err)
	hidden := env.CreateUser(t, "oculta-seg@wherever.test")
	_, err = env.Users.Update(ctx, hidden.ID, bson.D{
		{Key: "nombre_usuario", Value: "seg_oculta"},
		{Key: "telefono_hash", Value: usuarios.HashPhone(env.Cfg.PhoneHMACKey, "+525533334444")},
	})
	require.NoError(t, err)

	for _, q := range []string{"seg_", "%2B525511112222"} {
		r := send(t, app, http.MethodGet, buscarPath+q, nil, token)
		require.Equal(t, http.StatusOK, r.status, r.body)
		for _, secret := range []string{"correo", "telefono", "@wherever.test", "5511112222", "password"} {
			assert.NotContains(t, r.body, secret, q)
		}
	}

	notSearchable := send(t, app, http.MethodGet, buscarPath+"%2B525533334444", nil, token)
	notRegistered := send(t, app, http.MethodGet, buscarPath+"%2B525599990000", nil, token)
	assert.Equal(t, notRegistered.status, notSearchable.status)
	assert.Equal(t, notRegistered.body, notSearchable.body, "no se distingue un teléfono registrado pero oculto")
}

func TestPersonasSecurity_RequiereSesionYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, buscarPath+"ana", nil, "").status)

	pending := env.CreateUser(t, "pendiente-busca@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-busca", middleware.RoleConsumer)
	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodGet, buscarPath+"ana", nil, token).status)
}
