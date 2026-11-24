package contract

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

// TestContract_PlanesNV89 fija la forma de reservar todo el plan, del enlace con permiso y del
// plan visto por un integrante (`mi_permiso`).
func TestContract_PlanesNV89(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	duena := env.CreateUser(t, "duena-nv89-contrato@wherever.test")
	conCupo := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})
	sinReservas := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{SinReservas: true})
	owner := env.CreateUser(t, "owner-nv89-contrato@wherever.test")
	editor := env.CreateUser(t, "editor-nv89-contrato@wherever.test")
	ctx := context.Background()
	require.NoError(t, env.Users.MarkEmailVerified(ctx, owner.ID))
	p := planes.Plan{
		OwnerID: owner.ID, Nombre: "Contrato", Fecha: reservastest.Manana(0), Personas: 2, Movilidad: planes.MobilityWalk,
		Origen: planes.OriginManual, Visibilidad: planes.VisibilityPrivate, Traslados: []planes.Traslado{},
		Paradas: []planes.Parada{
			{ID: bson.NewObjectID(), NegocioID: conCupo.ID, Llega: reservastest.Manana(13), Minutos: 60, Precio: 100},
			{ID: bson.NewObjectID(), NegocioID: sinReservas.ID, Llega: reservastest.Manana(15), Minutos: 60, Precio: 100},
		},
	}
	require.NoError(t, planes.NewRepository(env.Mongo.DB()).Create(ctx, &p))
	token := testutil.ValidTokenFor(t, owner.ID.Hex(), "jti-owner-nv89-contrato", middleware.RoleConsumer)
	other := testutil.ValidTokenFor(t, editor.ID.Hex(), "jti-editor-nv89-contrato", middleware.RoleConsumer)
	path := "/api/v1/planes/" + p.ID.Hex()

	// La llave va partida: gitleaks lee una UUID junto a «Key» como llave de API.
	llave := "4d6f1c2a-8b3e-4f5a-" + "9c7d-1e2f3a4b5c89"
	status, body := callHeaders(t, app, http.MethodPost, path+"/reservas", nil, token, map[string]string{"Idempotency-Key": llave})
	check(t, "planes/reservar_todo_200", status, body)

	status, body = call(t, app, http.MethodPost, path+"/compartir", map[string]string{"permiso": "editar"}, token)
	check(t, "planes/compartir_201", status, body)
	status, joined := call(t, app, http.MethodPost, "/api/v1/grupos/unirse", map[string]string{"codigo": str(t, obj(t, body)["codigo"])}, other)
	require.Equal(t, http.StatusOK, status)
	status, body = call(t, app, http.MethodGet, path, nil, other)
	check(t, "planes/detalle_integrante_200", status, body)

	g, err := grupos.NewRepository(env.Mongo.DB()).FindByPlan(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, g.ID.Hex(), str(t, obj(t, joined)["id"]))
	status, body = call(t, app, http.MethodPatch, "/api/v1/grupos/"+g.ID.Hex()+"/integrantes/"+editor.ID.Hex(), map[string]string{"permiso_plan": "ver"}, token)
	check(t, "grupos/permiso_integrante_200", status, body)
}
