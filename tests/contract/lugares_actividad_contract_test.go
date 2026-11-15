package contract

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// TestContract_LugaresActividad fija las formas de nv-66: conteo, afluencia, recientes y los 204.
func TestContract_LugaresActividad(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	place := negocios.Business{ID: bson.NewObjectID(), Slug: "actividad-contrato", QRCheckin: "qr-actividad", Name: "Actividad", Category: "restaurante",
		State: negocios.StatusApproved, PriceLevel: 2, Location: negocios.Location{Type: "Point", Coordinates: []float64{-99.13, 19.43}}, CreatedAt: time.Now()}
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), place)
	require.NoError(t, err)
	user := env.CreateUser(t, "actividad-contrato@wherever.test")
	token := testutil.ValidTokenFor(t, user.ID.Hex(), "jti-actividad-contrato", middleware.RoleConsumer)

	status, body := call(t, app, http.MethodGet, "/api/v1/lugares/conteo?categoria=restaurante", nil, "")
	check(t, "lugares/conteo_200", status, body)
	// Un día que no es hoy: `hora_actual` va en null (con hoy sería un número).
	manana := (int(time.Now().In(negocios.MexicoCity()).Weekday()) + 1) % 7
	status, body = call(t, app, http.MethodGet, "/api/v1/lugares/"+place.ID.Hex()+"/afluencia?dia="+strconv.Itoa(manana), nil, "")
	check(t, "lugares/afluencia_200", status, body)
	status, _ = statusWith(t, app, http.MethodPost, "/api/v1/lugares/"+place.ID.Hex()+"/vistas", "", map[string]string{"origen": "mapa"}, token)
	require.Equal(t, http.StatusNoContent, status)
	status, body = call(t, app, http.MethodGet, "/api/v1/lugares/recientes", nil, token)
	check(t, "lugares/recientes_200", status, body)
	status, _ = statusWith(t, app, http.MethodDelete, "/api/v1/lugares/recientes", "", nil, token)
	require.Equal(t, http.StatusNoContent, status)
}
