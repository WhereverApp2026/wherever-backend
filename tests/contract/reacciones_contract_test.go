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
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// TestContract_Reacciones fija las formas de nv-83: me gusta (204), comentarios con una respuesta,
// borrar (204), quién reaccionó y el 404 de un objeto que no se ve.
func TestContract_Reacciones(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	env.ClearRateLimits(t)
	app := env.App()
	ctx := context.Background()
	db := env.Mongo.DB()
	me := env.CreateUser(t, "yo-reacc-contrato@wherever.test")
	friend := env.CreateUser(t, "amiga-reacc-contrato@wherever.test")
	now := time.Now().UTC()
	f := amistades.Friendship{ID: bson.NewObjectID(), From: me.ID, To: friend.ID, Status: amistades.StatusAccepted, Pair: amistades.PairKey(me.ID, friend.ID), CreatedAt: now, UpdatedAt: now}
	require.NoError(t, amistades.NewRepository(db).Insert(ctx, &f))
	p := postales.Postal{ID: bson.NewObjectID(), UsuarioID: friend.ID, ImagenID: bson.NewObjectID().Hex(), ImagenEstado: "publicada",
		Estilo: postales.EstiloPolaroid, TextoEstado: postales.TextoAprobado, Visibilidad: postales.VisAmigos, Estado: postales.EstadoLista, Version: 1, CreadoEn: now, ActualizadoEn: now}
	_, err := db.Collection(postales.Collection).InsertOne(ctx, p)
	require.NoError(t, err)
	m := postales.Momento{ID: bson.NewObjectID(), AutorID: friend.ID, PostalID: p.ID, Visibilidad: postales.VisAmigos, Estado: postales.MomentoPublicado,
		Etiquetados: []bson.ObjectID{}, Version: 1, CreadoEn: now, ActualizadoEn: now}
	_, err = db.Collection(postales.MomentosCollection).InsertOne(ctx, m)
	require.NoError(t, err)
	plan := planes.Plan{ID: bson.NewObjectID(), OwnerID: friend.ID, Nombre: "Sábado de cafés", Fecha: now.AddDate(0, 0, 3), Personas: 2,
		Movilidad: planes.MobilityWalk, Paradas: []planes.Parada{}, Traslados: []planes.Traslado{}, Estado: planes.StatusActive, Origen: planes.OriginManual,
		Version: 1, Visibilidad: planes.VisibilityPublic, CreadoEn: now, ActualizadoEn: now}
	_, err = db.Collection(planes.Collection).InsertOne(ctx, plan)
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, me.ID.Hex(), "jti-yo-reacc-contrato", middleware.RoleConsumer)
	tokenAmiga := testutil.ValidTokenFor(t, friend.ID.Hex(), "jti-amiga-reacc-contrato", middleware.RoleConsumer)

	momento := "/api/v1/momentos/" + m.ID.Hex()
	status, body := call(t, app, http.MethodPut, momento+"/me-gusta", nil, token)
	check(t, "reacciones/me_gusta_204", status, body)
	status, body = call(t, app, http.MethodPut, "/api/v1/planes/"+plan.ID.Hex()+"/me-gusta", nil, token)
	check(t, "reacciones/me_gusta_204", status, body)
	status, body = call(t, app, http.MethodPost, momento+"/comentarios", map[string]string{"texto": "¡Qué bonito!"}, token)
	check(t, "reacciones/comentario_201", status, body)
	padre := str(t, obj(t, body)["id"])
	status, body = call(t, app, http.MethodPost, momento+"/comentarios", map[string]string{"texto": "Gracias", "padre_id": padre}, tokenAmiga)
	check(t, "reacciones/respuesta_201", status, body)
	status, body = call(t, app, http.MethodGet, momento+"/comentarios", nil, token)
	check(t, "reacciones/comentarios_200", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/planes/"+plan.ID.Hex()+"/reacciones", nil, tokenAmiga)
	check(t, "reacciones/reacciones_200", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/planes/"+bson.NewObjectID().Hex()+"/comentarios", nil, token)
	check(t, "reacciones/objeto_404", status, body)
	status, body = call(t, app, http.MethodPost, momento+"/comentarios", map[string]string{"texto": "x", "padre_id": bson.NewObjectID().Hex()}, token)
	check(t, "reacciones/comentario_422", status, body)
	status, body = call(t, app, http.MethodDelete, "/api/v1/comentarios/"+padre, nil, token)
	check(t, "reacciones/borrar_204", status, body)
}
