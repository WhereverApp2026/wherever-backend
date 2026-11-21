package security

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// La descripción (nv-77) solo la cambia su dueña con sesión; un usuario_id en el cuerpo no
// cambia la de otra persona; sin caracteres de control ni de dirección (suplantación); quien
// está bloqueado no la ve.
func TestUsuariosDescripcionSecurity_SoloLaPropiaYSinControles(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	ana := env.CreateUser(t, "ana-desc-seg@wherever.test")
	beto := env.CreateUser(t, "beto-desc-seg@wherever.test")
	anaToken := testutil.ValidTokenFor(t, ana.ID.Hex(), "jti-ana-desc-seg", middleware.RoleConsumer)
	betoToken := testutil.ValidTokenFor(t, beto.ID.Hex(), "jti-beto-desc-seg", middleware.RoleConsumer)

	for name, token := range map[string]string{"sin token": "", "vencido": testutil.ExpiredToken(t), "alg none": testutil.AlgNoneToken(t)} {
		assert.Equal(t, http.StatusUnauthorized, sendIfMatch(t, app, http.MethodPatch, "/api/v1/usuarios/yo", map[string]any{"descripcion": "x"}, token).status, name)
	}

	r := sendIfMatch(t, app, http.MethodPatch, "/api/v1/usuarios/yo", map[string]any{
		"descripcion": "<script>alert(1)</script>\u202etxt.exe\u200b", "usuario_id": ana.ID.Hex(), "id": ana.ID.Hex(),
	}, betoToken)
	require.Equal(t, http.StatusOK, r.status, r.body)
	betoGuardado, err := env.Users.FindByID(context.Background(), beto.ID)
	require.NoError(t, err)
	assert.Equal(t, "<script>alert(1)</script> txt.exe", betoGuardado.Description, "sin controles ni marcas de dirección; el resto es texto")
	anaGuardada, err := env.Users.FindByID(context.Background(), ana.ID)
	require.NoError(t, err)
	assert.Empty(t, anaGuardada.Description, "el cuerpo no elige a quién se edita")
	assert.True(t, strings.HasPrefix(r.headers.Get(fiber.HeaderContentType), fiber.MIMEApplicationJSON), "se sirve como JSON, nunca como HTML")

	visto := send(t, app, http.MethodGet, "/api/v1/personas/"+beto.ID.Hex(), nil, anaToken)
	require.Equal(t, http.StatusOK, visto.status, visto.body)
	assert.Contains(t, visto.body, `"descripcion"`)

	// Con bloqueo, el perfil completo (y su descripción) es 404.
	_, err = env.Mongo.DB().Collection(amistades.Collection).InsertOne(context.Background(), amistades.Friendship{
		ID: bson.NewObjectID(), From: beto.ID, To: ana.ID, Status: amistades.StatusBlocked, Pair: amistades.PairKey(ana.ID, beto.ID)})
	require.NoError(t, err)
	bloqueada := send(t, app, http.MethodGet, "/api/v1/personas/"+beto.ID.Hex(), nil, anaToken)
	assert.Equal(t, http.StatusNotFound, bloqueada.status)
	assert.NotContains(t, bloqueada.body, "txt.exe")
}
