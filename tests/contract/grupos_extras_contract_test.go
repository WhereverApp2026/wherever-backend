package contract

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// TestContract_GruposExtras fija la forma del grupo con punto de encuentro (nv-69) y de los
// traslados: la fila propia con minutos y costo exactos.
func TestContract_GruposExtras(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	org := env.CreateUser(t, "org-extras-contrato@wherever.test")
	friend := env.CreateUser(t, "amiga-extras-contrato@wherever.test")
	now := time.Now().UTC()
	day := now.AddDate(0, 0, 1)
	p := planes.Plan{
		ID: bson.NewObjectID(), OwnerID: org.ID, Nombre: "Contrato", Fecha: day, Personas: 2, Movilidad: planes.MobilityWalk,
		Paradas: []planes.Parada{{ID: bson.NewObjectID(), NegocioID: bson.NewObjectID(), Llega: day, Minutos: 60}},
		Estado:  planes.StatusActive, Origen: planes.OriginManual, Version: 1, CreadoEn: now, ActualizadoEn: now,
	}
	_, err := env.Mongo.DB().Collection(planes.Collection).InsertOne(context.Background(), p)
	require.NoError(t, err)
	g := grupos.Group{
		ID: bson.NewObjectID(), PlanID: p.ID, OrganizerID: org.ID, Occupied: 2, CreatedAt: now, UpdatedAt: now,
		Members: []grupos.Member{
			{UserID: org.ID, Role: grupos.RoleOrganizer, Status: grupos.StatusConfirmed, Since: now},
			{UserID: friend.ID, Role: grupos.RoleMember, Status: grupos.StatusConfirmed, Since: now},
		},
	}
	require.NoError(t, grupos.NewRepository(env.Mongo.DB()).Create(context.Background(), &g))
	token := testutil.ValidTokenFor(t, org.ID.Hex(), "jti-org-extras-contrato", middleware.RoleConsumer)
	other := testutil.ValidTokenFor(t, friend.ID.Hex(), "jti-amiga-extras-contrato", middleware.RoleConsumer)
	path := "/api/v1/grupos/" + g.ID.Hex()

	loc := time.FixedZone("MX", -6*3600)
	d := day.In(loc)
	at := time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, loc)
	if at.Before(now) {
		at = now.Add(time.Hour)
	}
	call(t, app, http.MethodPatch, path+"/integrantes/yo", map[string]any{"ubicacion": map[string]float64{"lat": 19.43, "lng": -99.13}}, token)
	call(t, app, http.MethodPatch, path+"/integrantes/yo", map[string]any{"ubicacion": map[string]float64{"lat": 19.44, "lng": -99.14}}, other)
	status, body := call(t, app, http.MethodPut, path+"/punto-encuentro", map[string]any{"nombre": "Kiosco", "lat": 19.435, "lng": -99.135, "hora": at.Format(time.RFC3339)}, token)
	check(t, "grupos/punto_encuentro_200", status, body)

	status, body = call(t, app, http.MethodGet, path+"/punto-encuentro/traslados", nil, token)
	check(t, "grupos/traslados_200", status, body)
}
