package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
)

// planFuturo crea un plan de who para dentro de una semana con una parada en el lugar abierto.
func planFuturo(t *testing.T, s postalesEnv, who amigo) string {
	t.Helper()
	day := time.Now().AddDate(0, 0, 7).Format(planes.DayLayout)
	body := map[string]any{
		"nombre": "Salida al centro", "fecha": day, "personas": 2, "presupuesto": 2000, "movilidad": "a_pie",
		"paradas": []map[string]any{{"negocio_id": s.lugar.Hex(), "llega": day + "T12:00:00-06:00", "minutos": 60, "precio": 150}},
	}
	got := planCall(t, s.app, http.MethodPost, "/api/v1/planes", body, who.token, "")
	require.Equal(t, http.StatusCreated, got.status, got.body)
	return got.plan(t).ID.Hex()
}

// planPasado crea el plan y lo mueve a hace tres días (pestaña `pasados` de nv-30).
func planPasado(t *testing.T, s postalesEnv, who amigo) string {
	t.Helper()
	id := planFuturo(t, s, who)
	setDoc(t, s.env, planes.Collection, oidDe(t, id), bson.D{{Key: "fecha", Value: time.Now().AddDate(0, 0, -3).UTC()}})
	return id
}

// grupoCon deja a who confirmada en el grupo del plan del organizador.
func grupoCon(t *testing.T, s postalesEnv, plan string, org, who amigo, estado string) {
	t.Helper()
	now := time.Now().UTC()
	g := &grupos.Group{ID: bson.NewObjectID(), PlanID: oidDe(t, plan), OrganizerID: org.user.ID, Occupied: 2, CreatedAt: now, UpdatedAt: now,
		Members: []grupos.Member{
			{UserID: org.user.ID, Role: grupos.RoleOrganizer, Status: grupos.StatusConfirmed, Since: now},
			{UserID: who.user.ID, Role: grupos.RoleMember, Status: estado, Since: now},
		}}
	require.NoError(t, grupos.NewRepository(s.env.Mongo.DB()).Create(context.Background(), g))
}

func TestRecuerdos_PlanPasadoPropioYDeGrupo(t *testing.T) {
	s := nuevoPostales(t)
	yo, amiga, invitada, extrano := seedAmigo(t, s.env, "recuerda"), seedAmigo(t, s.env, "recuerda-amiga"), seedAmigo(t, s.env, "recuerda-inv"), seedAmigo(t, s.env, "recuerda-ext")

	futuro := planFuturo(t, s, yo)
	pcall(t, s.app, http.MethodPost, "/api/v1/planes/"+futuro+"/recuerdo", nil, yo.token, http.StatusUnprocessableEntity)

	plan := planPasado(t, s, yo)
	body := cuerpoPostal(fotoPublicada(t, s, yo), postales.VisPrivada)
	body["plan_id"] = plan
	postal := crearPostal(t, s, yo, body)
	tarjeta := fotoPublicada(t, s, yo)
	ruta := "/api/v1/planes/" + plan + "/recuerdo"
	pcall(t, s.app, http.MethodGet, ruta, nil, yo.token, http.StatusNotFound)
	rec := pcall(t, s.app, http.MethodPost, ruta, map[string]any{"tarjeta_imagen_id": tarjeta}, yo.token, http.StatusCreated)
	assert.Equal(t, "Salida al centro", rec["titulo"])
	assert.Equal(t, []any{postal}, rec["postales_ids"])
	assert.Equal(t, float64(0), rec["puntos_ganados"])
	assert.Contains(t, rec["tarjeta_url"], "/i/"+tarjeta+"/1080.jpg")
	paradas := list(t, rec["paradas"])
	require.Len(t, paradas, 1)
	assert.Equal(t, "Lugar postales-plan", obj(t, paradas[0])["nombre"])
	gasto, ok := rec["gasto_total"].(float64)
	require.True(t, ok)
	assert.Zero(t, int64(gasto)%100, "centavos")

	otra := pcall(t, s.app, http.MethodPost, ruta, nil, yo.token, http.StatusOK)
	assert.Equal(t, rec["id"], otra["id"], "regenerar reemplaza la copia")
	assert.Nil(t, otra["tarjeta_url"])
	pcall(t, s.app, http.MethodPost, ruta, map[string]any{"tarjeta_imagen_id": fotoDudosa(t, s, yo)}, yo.token, http.StatusUnprocessableEntity)
	pcall(t, s.app, http.MethodGet, ruta, nil, yo.token, http.StatusOK)

	// Integrante confirmado sí; invitado sin confirmar y ajeno = 404.
	grupoCon(t, s, plan, yo, amiga, grupos.StatusConfirmed)
	_, err := s.env.Mongo.DB().Collection(grupos.Collection).UpdateOne(context.Background(), bson.D{{Key: "plan_id", Value: oidDe(t, plan)}},
		bson.D{{Key: "$push", Value: bson.D{{Key: "integrantes", Value: grupos.Member{UserID: invitada.user.ID, Role: grupos.RoleMember, Status: grupos.StatusInvited, Since: time.Now().UTC()}}}}})
	require.NoError(t, err)
	suyo := pcall(t, s.app, http.MethodPost, ruta, nil, amiga.token, http.StatusCreated)
	assert.NotEqual(t, rec["id"], suyo["id"])
	assert.Empty(t, suyo["postales_ids"], "solo sus postales")
	pcall(t, s.app, http.MethodPost, ruta, nil, invitada.token, http.StatusNotFound)
	pcall(t, s.app, http.MethodPost, ruta, nil, extrano.token, http.StatusNotFound)
	pcall(t, s.app, http.MethodGet, ruta, nil, extrano.token, http.StatusNotFound)

	items := list(t, pcall(t, s.app, http.MethodGet, "/api/v1/recuerdos", nil, yo.token, http.StatusOK)["items"])
	require.Len(t, items, 1)
	pcall(t, s.app, http.MethodGet, "/api/v1/recuerdos?mes=2001-01", nil, yo.token, http.StatusOK)

	id := str(t, rec["id"])
	pcall(t, s.app, http.MethodDelete, "/api/v1/recuerdos/"+id, nil, extrano.token, http.StatusNotFound)
	pcall(t, s.app, http.MethodDelete, "/api/v1/recuerdos/"+id, nil, yo.token, http.StatusNoContent)
	pcall(t, s.app, http.MethodDelete, "/api/v1/recuerdos/"+id, nil, yo.token, http.StatusNotFound)
	pcall(t, s.app, http.MethodGet, postalesPath+"/"+postal, nil, yo.token, http.StatusOK)
}
