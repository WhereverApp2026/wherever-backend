package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/cupones"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/resenas"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// La tarjeta completa de Explorar (nv-85): Sello, afluencia, mejor cupón y amigos que fueron.
func TestContract_ExplorarTarjeta(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	admin := env.CreateAdmin(t, "admin-contrato-exp85@wherever.test")
	rows, err := json.Marshal([]negocios.ImportRow{contractBusiness("contrato-exp85")})
	require.NoError(t, err)
	_, err = env.Negocios.Import(context.Background(), admin.ID.Hex(), bytes.NewReader(rows), "json")
	require.NoError(t, err)

	ctx := context.Background()
	db := env.Mongo.DB()
	var lugar negocios.Business
	require.NoError(t, db.Collection(negocios.Collection).FindOne(ctx, bson.D{{Key: "slug", Value: "contrato-exp85"}}).Decode(&lugar))
	curva := make([][]int, 7)
	for d := range curva {
		curva[d] = make([]int, 24)
		for h := range curva[d] {
			curva[d][h] = 50
		}
	}
	_, err = db.Collection(negocios.Collection).UpdateByID(ctx, lugar.ID, bson.D{{Key: "$set", Value: bson.D{
		{Key: "sello_excelencia", Value: true}, {Key: "cupones_activos", Value: 1}, {Key: "afluencia", Value: curva},
	}}})
	require.NoError(t, err)
	now := time.Now().UTC()
	_, err = db.Collection(cupones.Collection).InsertOne(ctx, cupones.Cupon{ID: bson.NewObjectID(), NegocioID: lugar.ID, Titulo: "Cupón $50",
		Tipo: cupones.TipoMonto, Valor: 5000, VigenciaInicio: now.Add(-time.Hour), VigenciaFin: now.Add(time.Hour), Estado: cupones.EstadoActivo,
		Ubicacion: cupones.Location{Type: "Point", Coordinates: []float64{-99.1332, 19.4326}}, CreadoEn: now, ActualizadoEn: now})
	require.NoError(t, err)

	yo := env.CreateUser(t, "yo-contrato-exp85@wherever.test")
	ana := env.CreateUser(t, "ana-contrato-exp85@wherever.test")
	f := amistades.Friendship{ID: bson.NewObjectID(), From: yo.ID, To: ana.ID, Status: amistades.StatusAccepted,
		Pair: amistades.PairKey(yo.ID, ana.ID), CreatedAt: now, UpdatedAt: now}
	require.NoError(t, amistades.NewRepository(db).Insert(ctx, &f))
	_, err = db.Collection(resenas.CheckinsCollection).InsertOne(ctx, resenas.Checkin{ID: bson.NewObjectID(), UsuarioID: ana.ID, NegocioID: lugar.ID,
		Metodo: resenas.MetodoQR, Valido: true, Dia: now.Format(time.DateOnly), CreadoEn: now.Add(-time.Hour)})
	require.NoError(t, err)

	token := testutil.ValidTokenFor(t, yo.ID.Hex(), "jti-contrato-exp85", middleware.RoleConsumer)
	status, body := call(t, env.App(), http.MethodGet, "/api/v1/lugares?limite=1&de_amigos=true&con_sello=true&con_cupon=true", nil, token)
	check(t, "negocios/lugares_tarjeta_200", status, body)
}
