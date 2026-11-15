package security

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Reportar pide sesión y cuenta activa; las rutas admin, rol de administrador; el dueño no
// reporta su negocio; un detalle rechazado = 422.
func TestReportesNegocioSecurity_RolesYModeracion(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	place := negocios.Business{ID: bson.NewObjectID(), Slug: "rep-sec", QRCheckin: "qr-rep-sec", Name: "Reporte seguridad", Category: "restaurante",
		State: negocios.StatusApproved, Location: negocios.Location{Type: "Point", Coordinates: []float64{-99.13, 19.43}}, CreatedAt: time.Now()}
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), place)
	require.NoError(t, err)
	reportPath := "/api/v1/lugares/" + place.ID.Hex() + "/reportes"
	body := map[string]string{"motivo": "horario"}

	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodPost, reportPath, body, "").status)
	for _, r := range [][2]string{{http.MethodGet, "/api/v1/admin/reportes-negocio"}, {http.MethodPatch, "/api/v1/admin/reportes-negocio/" + bson.NewObjectID().Hex()}} {
		assert.Equal(t, http.StatusUnauthorized, sendIfMatch(t, app, r[0], r[1], map[string]string{"estado": "corregido"}, "").status, r[1])
		for _, roles := range [][]string{{middleware.RoleConsumer}, {middleware.RoleConsumer, middleware.RoleMerchant}} {
			u := env.CreateUser(t, "rep-sec-"+strings.Join(roles, "-")+"-"+r[0]+"@wherever.test")
			token := testutil.ValidTokenFor(t, u.ID.Hex(), "jti-rep-sec-"+u.ID.Hex(), roles...)
			assert.Equal(t, http.StatusForbidden, sendIfMatch(t, app, r[0], r[1], map[string]string{"estado": "corregido"}, token).status, r[1])
		}
	}

	pending := env.CreateUser(t, "pendiente-reportes@wherever.test")
	_, err = env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodPost, reportPath, body, testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-reportes", middleware.RoleConsumer)).status)

	duena := env.CreateUser(t, "rep-sec-duena@wherever.test")
	b := env.AltaAprobada(t, duena, "Reporte propio seguridad")
	ownerToken := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-rep-sec-duena", middleware.RoleConsumer, middleware.RoleMerchant)
	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodPost, "/api/v1/lugares/"+b.ID+"/reportes", body, ownerToken).status)

	env.ModeradorReportes.Fijar(moderacion.VeredictoRechazado)
	t.Cleanup(func() { env.ModeradorReportes.Fijar(moderacion.VeredictoAprobado) })
	user := env.CreateUser(t, "rep-sec-texto@wherever.test")
	token := testutil.ValidTokenFor(t, user.ID.Hex(), "jti-rep-sec-texto", middleware.RoleConsumer)
	assert.Equal(t, http.StatusUnprocessableEntity, send(t, app, http.MethodPost, reportPath, map[string]string{"motivo": "otro", "detalle": "texto ofensivo"}, token).status)
}
