package contract

import (
	"context"
	"net/http"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// TestContract_NotificacionesCategorias fija la pestaña con `categoria` y los conteos
// `por_categoria` de nv-86.
func TestContract_NotificacionesCategorias(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	u := env.CreateUser(t, "categorias-contrato@wherever.test")
	svc := notificaciones.NewService(notificaciones.NewRepository(env.Mongo.DB()), nil, env.Log)
	for range 2 {
		svc.Emit(context.Background(), u.ID, notificaciones.KindLike, map[string]string{"momento_id": bson.NewObjectID().Hex()})
	}
	svc.Emit(context.Background(), u.ID, notificaciones.KindReservaRecordatorio, map[string]string{"reserva_id": bson.NewObjectID().Hex()})
	token := testutil.ValidTokenFor(t, u.ID.Hex(), "jti-categorias-contrato", middleware.RoleConsumer)

	status, body := call(t, app, http.MethodGet, "/api/v1/notificaciones?categoria=social&limite=1", nil, token)
	check(t, "notificaciones/lista_social_200", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/notificaciones/no-leidas", nil, token)
	check(t, "notificaciones/no_leidas_200", status, body)
}
