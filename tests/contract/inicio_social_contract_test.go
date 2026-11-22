package contract

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/votaciones"
)

// TestContract_InicioSocial fija las formas de nv-84: planes de amigos, historias y votaciones
// pendientes.
func TestContract_InicioSocial(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	env.ClearRateLimits(t)
	app := env.App()
	ctx := context.Background()
	db := env.Mongo.DB()
	me := env.CreateUser(t, "yo-ini84-contrato@wherever.test")
	friend := env.CreateUser(t, "amiga-ini84-contrato@wherever.test")
	now := time.Now().UTC()
	f := amistades.Friendship{ID: bson.NewObjectID(), From: me.ID, To: friend.ID, Status: amistades.StatusAccepted, Pair: amistades.PairKey(me.ID, friend.ID), CreatedAt: now, UpdatedAt: now}
	require.NoError(t, amistades.NewRepository(db).Insert(ctx, &f))
	p := postales.Postal{ID: bson.NewObjectID(), UsuarioID: friend.ID, ImagenID: bson.NewObjectID().Hex(), ImagenEstado: "publicada", ImagenURL: "https://img.test/historia.webp",
		Estilo: postales.EstiloPolaroid, TextoEstado: postales.TextoAprobado, Visibilidad: postales.VisAmigos, Estado: postales.EstadoLista, Version: 1, CreadoEn: now, ActualizadoEn: now}
	_, err := db.Collection(postales.Collection).InsertOne(ctx, p)
	require.NoError(t, err)
	m := postales.Momento{ID: bson.NewObjectID(), AutorID: friend.ID, PostalID: p.ID, Visibilidad: postales.VisAmigos, Estado: postales.MomentoPublicado,
		Etiquetados: []bson.ObjectID{}, Version: 1, CreadoEn: now, ActualizadoEn: now}
	_, err = db.Collection(postales.MomentosCollection).InsertOne(ctx, m)
	require.NoError(t, err)
	plan := planes.Plan{ID: bson.NewObjectID(), OwnerID: friend.ID, Nombre: "Sábado de cafés", Fecha: now.AddDate(0, 0, 3), Personas: 2,
		Movilidad: planes.MobilityWalk, Paradas: []planes.Parada{{ID: bson.NewObjectID(), NegocioID: bson.NewObjectID(), Llega: now.AddDate(0, 0, 3), Minutos: 60, Precio: 210}},
		Traslados: []planes.Traslado{}, Estado: planes.StatusActive, Origen: planes.OriginManual, CostoPorPersona: 420,
		Version: 1, Visibilidad: planes.VisibilityFriends, CreadoEn: now, ActualizadoEn: now}
	_, err = db.Collection(planes.Collection).InsertOne(ctx, plan)
	require.NoError(t, err)
	g := grupos.Group{ID: bson.NewObjectID(), PlanID: plan.ID, OrganizerID: friend.ID, Version: 1, Occupied: 2, CreatedAt: now, UpdatedAt: now,
		Members: []grupos.Member{{UserID: friend.ID, Role: grupos.RoleOrganizer, Status: grupos.StatusConfirmed, Since: now},
			{UserID: me.ID, Role: grupos.RoleMember, Status: grupos.StatusInvited, Since: now}}}
	require.NoError(t, grupos.NewRepository(db).Create(ctx, &g))
	v := votaciones.Vote{ID: bson.NewObjectID(), GroupID: g.ID, PlanID: plan.ID, StopID: plan.Paradas[0].ID, Options: []bson.ObjectID{bson.NewObjectID(), bson.NewObjectID()},
		Ballots: map[string]bson.ObjectID{}, ClosesAt: now.Add(time.Hour), Status: votaciones.StatusOpen, CreatedBy: friend.ID, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, votaciones.NewRepository(db).Create(ctx, &v))
	token := testutil.ValidTokenFor(t, me.ID.Hex(), "jti-yo-ini84-contrato", middleware.RoleConsumer)

	status, body := call(t, app, http.MethodGet, "/api/v1/planes/amigos", nil, token)
	check(t, "inicio/planes_amigos_200", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/planes/amigos?cursor=malo", nil, token)
	check(t, "inicio/planes_amigos_422", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/momentos/historias", nil, token)
	check(t, "inicio/historias_200", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/votaciones/pendientes", nil, token)
	check(t, "inicio/votaciones_pendientes_200", status, body)
}
