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
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
	"github.com/WhereverApp2026/wherever-backend/internal/resenas"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// TestContract_PersonasPerfil fija las formas de nv-70: perfil de amigo (degradado sin grafo),
// 404, silencio y contactos.
func TestContract_PersonasPerfil(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	me := env.CreateUser(t, "yo-perfil-contrato@wherever.test")
	friend := env.CreateUser(t, "amiga-perfil-contrato@wherever.test")
	_, err := env.Users.Update(context.Background(), friend.ID, bson.D{
		{Key: "telefono_hash", Value: usuarios.HashPhone(env.Cfg.PhoneHMACKey, "+525587654321")}, {Key: "privacidad.buscable_por_telefono", Value: true},
	})
	require.NoError(t, err)
	now := time.Now().UTC()
	f := amistades.Friendship{ID: bson.NewObjectID(), From: me.ID, To: friend.ID, Status: amistades.StatusAccepted, Pair: amistades.PairKey(me.ID, friend.ID), CreatedAt: now, UpdatedAt: now}
	require.NoError(t, amistades.NewRepository(env.Mongo.DB()).Insert(context.Background(), &f))
	token := testutil.ValidTokenFor(t, me.ID.Hex(), "jti-yo-perfil-contrato", middleware.RoleConsumer)

	status, body := call(t, app, http.MethodGet, "/api/v1/personas/"+friend.ID.Hex(), nil, token)
	check(t, "personas/perfil_amigo_200", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/personas/"+bson.NewObjectID().Hex(), nil, token)
	check(t, "personas/perfil_404", status, body)
	status, body = call(t, app, http.MethodPut, "/api/v1/amistades/"+f.ID.Hex()+"/silencio", map[string]bool{"silenciado": true}, token)
	check(t, "personas/silencio_200", status, body)
	status, body = call(t, app, http.MethodPost, "/api/v1/personas/contactos", map[string][]string{"telefonos": {"5587654321"}}, token)
	check(t, "personas/contactos_200", status, body)
}

// TestContract_PersonasPestanas fija las formas de nv-81: las tres pestañas del perfil con un
// elemento cada una y la lista de amigos con sus planes.
func TestContract_PersonasPestanas(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	env.ClearRateLimits(t)
	app := env.App()
	ctx := context.Background()
	db := env.Mongo.DB()
	me := env.CreateUser(t, "yo-pestanas-contrato@wherever.test")
	friend := env.CreateUser(t, "amiga-pestanas-contrato@wherever.test")
	now := time.Now().UTC()
	f := amistades.Friendship{ID: bson.NewObjectID(), From: me.ID, To: friend.ID, Status: amistades.StatusAccepted, Pair: amistades.PairKey(me.ID, friend.ID), CreatedAt: now, UpdatedAt: now}
	require.NoError(t, amistades.NewRepository(db).Insert(ctx, &f))
	lugar := negocios.Business{ID: bson.NewObjectID(), Slug: "pestanas-contrato", QRCheckin: "qr-pestanas-contrato", Name: "Pestañas", NormalizedName: "pestanas",
		Category: "restaurante", State: negocios.StatusApproved, Location: negocios.Location{Type: "Point", Coordinates: []float64{-99.13, 19.43}}, CreatedAt: now}
	_, err := db.Collection(negocios.Collection).InsertOne(ctx, lugar)
	require.NoError(t, err)
	_, err = db.Collection(planes.Collection).InsertOne(ctx, bson.D{
		{Key: "_id", Value: bson.NewObjectID()}, {Key: "dueno_id", Value: friend.ID}, {Key: "nombre", Value: "Plan"}, {Key: "fecha", Value: now},
		{Key: "estado", Value: planes.StatusActive}, {Key: "visibilidad", Value: planes.VisibilityPublic}, {Key: "personas", Value: 2},
		{Key: "movilidad", Value: planes.MobilityWalk}, {Key: "paradas", Value: bson.A{bson.D{{Key: "id", Value: bson.NewObjectID()}, {Key: "negocio_id", Value: lugar.ID}, {Key: "llega", Value: now}, {Key: "minutos", Value: 60}, {Key: "precio", Value: 100}}}},
	})
	require.NoError(t, err)
	_, err = db.Collection(resenas.Collection).InsertOne(ctx, bson.D{
		{Key: "_id", Value: bson.NewObjectID()}, {Key: "usuario_id", Value: friend.ID}, {Key: "negocio_id", Value: lugar.ID}, {Key: "checkin_id", Value: bson.NewObjectID()},
		{Key: "calificacion", Value: 5}, {Key: "comentario", Value: "Rico"}, {Key: "chips", Value: bson.A{"sabor"}}, {Key: "fotos", Value: bson.A{}},
		{Key: "estado", Value: resenas.EstadoPublicada}, {Key: "version", Value: 1}, {Key: "creado_en", Value: now}, {Key: "actualizado_en", Value: now},
		{Key: "respuesta_negocio", Value: bson.D{{Key: "texto", Value: "Gracias"}, {Key: "autor_id", Value: bson.NewObjectID()}, {Key: "fecha", Value: now}, {Key: "estado", Value: resenas.EstadoPublicada}}},
	})
	require.NoError(t, err)
	p := postales.Postal{ID: bson.NewObjectID(), UsuarioID: friend.ID, ImagenID: bson.NewObjectID().Hex(), ImagenEstado: "publicada", NegocioID: &lugar.ID,
		Estilo: postales.EstiloPolaroid, TextoEstado: postales.TextoAprobado, Visibilidad: postales.VisPublica, Estado: postales.EstadoLista, Version: 1, CreadoEn: now, ActualizadoEn: now}
	_, err = db.Collection(postales.Collection).InsertOne(ctx, p)
	require.NoError(t, err)
	_, err = db.Collection(postales.MomentosCollection).InsertOne(ctx, postales.Momento{ID: bson.NewObjectID(), AutorID: friend.ID, PostalID: p.ID, NegocioID: &lugar.ID,
		Visibilidad: postales.VisPublica, Estado: postales.MomentoPublicado, Etiquetados: []bson.ObjectID{}, Version: 1, CreadoEn: now, ActualizadoEn: now})
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, me.ID.Hex(), "jti-yo-pestanas-contrato", middleware.RoleConsumer)

	for _, pestana := range []string{"momentos", "planes", "resenas"} {
		status, body := call(t, app, http.MethodGet, "/api/v1/personas/"+friend.ID.Hex()+"/"+pestana, nil, token)
		check(t, "personas/pestana_"+pestana+"_200", status, body)
	}
	status, body := call(t, app, http.MethodGet, "/api/v1/personas/"+bson.NewObjectID().Hex()+"/planes", nil, token)
	check(t, "personas/pestana_404", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/amistades", nil, token)
	check(t, "personas/amigos_200", status, body)
}
