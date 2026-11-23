package contract

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// TestContract_Directos fija las formas de nv-87: abrir (201/200), la lista, los mensajes con un
// plan adjunto, reaccionar, leído (204), borrar (204) y el 404 de una conversación ajena.
func TestContract_Directos(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	env.ClearRateLimits(t)
	app := env.App()
	ctx := context.Background()
	db := env.Mongo.DB()
	me := env.CreateUser(t, "yo-dm-contrato@wherever.test")
	friend := env.CreateUser(t, "amiga-dm-contrato@wherever.test")
	now := time.Now().UTC()
	f := amistades.Friendship{ID: bson.NewObjectID(), From: me.ID, To: friend.ID, Status: amistades.StatusAccepted, Pair: amistades.PairKey(me.ID, friend.ID), CreatedAt: now, UpdatedAt: now}
	require.NoError(t, amistades.NewRepository(db).Insert(ctx, &f))
	plan := planes.Plan{ID: bson.NewObjectID(), OwnerID: me.ID, Nombre: "Tarde de tacos", Fecha: now.AddDate(0, 0, 3), Personas: 2,
		Movilidad: planes.MobilityWalk, Paradas: []planes.Parada{}, Traslados: []planes.Traslado{}, Estado: planes.StatusActive, Origen: planes.OriginManual,
		Version: 1, Visibilidad: planes.VisibilityPublic, CreadoEn: now, ActualizadoEn: now}
	_, err := db.Collection(planes.Collection).InsertOne(ctx, plan)
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, me.ID.Hex(), "jti-yo-dm-contrato", middleware.RoleConsumer)
	tokenAmiga := testutil.ValidTokenFor(t, friend.ID.Hex(), "jti-amiga-dm-contrato", middleware.RoleConsumer)

	status, body := call(t, app, http.MethodPost, "/api/v1/conversaciones", map[string]string{"usuario_id": friend.ID.Hex()}, token)
	check(t, "directos/abrir_201", status, body)
	conv := "/api/v1/conversaciones/" + str(t, obj(t, body)["id"])
	status, body = call(t, app, http.MethodPost, "/api/v1/conversaciones", map[string]string{"usuario_id": me.ID.Hex()}, tokenAmiga)
	check(t, "directos/abrir_200", status, body)
	status, body = call(t, app, http.MethodPost, conv+"/mensajes", map[string]any{"texto": "¿Te unes?", "id_cliente": "3f2b8c1e-5d4a-4b6f-9a7e-1c2d3e4f5a6b",
		"adjunto": map[string]string{"tipo": "plan", "id": plan.ID.Hex()}}, token)
	check(t, "directos/mensaje_201", status, body)
	msg := str(t, obj(t, body)["id"])
	status, body = call(t, app, http.MethodGet, "/api/v1/conversaciones", nil, tokenAmiga)
	check(t, "directos/lista_200", status, body)
	status, body = call(t, app, http.MethodGet, conv+"/mensajes", nil, tokenAmiga)
	check(t, "directos/mensajes_200", status, body)
	status, body = call(t, app, http.MethodPut, conv+"/mensajes/"+msg+"/reacciones", map[string]string{"emoji": "🎉"}, tokenAmiga)
	check(t, "directos/reaccion_200", status, body)
	status, body = call(t, app, http.MethodPost, conv+"/leido", nil, tokenAmiga)
	check(t, "directos/leido_204", status, body)
	status, body = call(t, app, http.MethodPost, conv+"/mensajes", map[string]any{"texto": "x", "id_cliente": "6a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d",
		"adjunto": map[string]string{"tipo": "momento", "id": bson.NewObjectID().Hex()}}, token)
	check(t, "directos/adjunto_422", status, body)
	status, body = call(t, app, http.MethodDelete, conv+"/mensajes/"+msg, nil, token)
	check(t, "directos/borrar_204", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/conversaciones/"+bson.NewObjectID().Hex()+"/mensajes", nil, token)
	check(t, "directos/conversacion_404", status, body)
}
