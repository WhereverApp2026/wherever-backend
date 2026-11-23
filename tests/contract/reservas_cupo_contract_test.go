package contract

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

// TestContract_DisponibilidadConAfluencia fija la disponibilidad con `afluencia` y `sugerida`
// por horario (nv-88).
func TestContract_DisponibilidadConAfluencia(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	duena := env.CreateUser(t, "duena-cupo-contrato@wherever.test")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})
	curva := make([][]int, 7)
	for d := range curva {
		curva[d] = make([]int, 24)
		for h := range curva[d] {
			curva[d][h] = 50
		}
	}
	_, err := env.Mongo.DB().Collection(negocios.Collection).UpdateByID(context.Background(), b.ID, bson.D{{Key: "$set", Value: bson.D{{Key: "afluencia", Value: curva}}}})
	require.NoError(t, err)

	status, body := call(t, app, http.MethodGet, "/api/v1/lugares/"+b.ID.Hex()+"/disponibilidad?fecha="+reservastest.FechaManana()+"&personas=2&hora=14:00", nil, "")
	check(t, "reservas/disponibilidad_200", status, body)
}
