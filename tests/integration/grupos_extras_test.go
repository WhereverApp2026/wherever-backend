package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/chat"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/vaca"
)

// raw hace una petición con If-Match opcional y devuelve estado y cuerpo.
func raw(t *testing.T, s gruposSetup, method, path string, body any, who amigo, ifMatch string) (int, string) {
	t.Helper()
	got := planCall(t, s.app, method, path, body, who.token, ifMatch)
	return got.status, got.body
}

func asView(t *testing.T, body string) grupos.View {
	t.Helper()
	var v grupos.View
	require.NoError(t, json.Unmarshal([]byte(body), &v), body)
	return v
}

// confirmedGroup crea el grupo del plan con n amigos confirmados.
func (s gruposSetup) confirmedGroup(t *testing.T, prefix string, n int) (grupos.View, []amigo) {
	t.Helper()
	fr := s.friends(t, prefix, n)
	v := s.create(t, fr)
	for _, f := range fr {
		grupoCall(t, s.app, http.MethodPatch, gruposPath+"/"+v.ID+"/integrantes/yo", map[string]string{"estado": "confirmado"}, f.token, http.StatusOK)
	}
	return v, fr
}

func planDayAt(p planes.Plan, clock string) string {
	return p.Fecha.Format(planes.DayLayout) + "T" + clock + "-06:00"
}

func TestGruposExtras_RenombrarYDisolver(t *testing.T) {
	s := newGrupos(t)
	v, fr := s.confirmedGroup(t, "ren", 1)
	path := gruposPath + "/" + v.ID
	assert.Equal(t, 1, v.Version)

	status, _ := raw(t, s, http.MethodPatch, path, map[string]string{"nombre": "Viernes"}, s.org, "")
	assert.Equal(t, http.StatusBadRequest, status, "sin If-Match")
	status, _ = raw(t, s, http.MethodPatch, path, map[string]string{"nombre": "Viernes"}, fr[0], "1")
	assert.Equal(t, http.StatusForbidden, status, "integrante")
	status, _ = raw(t, s, http.MethodPatch, path, map[string]string{"nombre": "   "}, s.org, "1")
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	status, body := raw(t, s, http.MethodPatch, path, map[string]string{"nombre": "Viernes"}, s.org, "1")
	require.Equal(t, http.StatusOK, status, body)
	got := asView(t, body)
	assert.Equal(t, "Viernes", got.Name)
	assert.Equal(t, 2, got.Version)
	status, _ = raw(t, s, http.MethodPatch, path, map[string]string{"nombre": "Otro"}, s.org, "1")
	assert.Equal(t, http.StatusConflict, status, "versión vieja")

	// Con saldos de La Vaca sin liquidar no se disuelve.
	debts := s.env.Mongo.DB().Collection(vaca.DebtsCollection)
	debt := vaca.Debt{ID: bson.NewObjectID(), GroupID: mustID(t, v.ID), Debtor: fr[0].user.ID, Creditor: s.org.user.ID, Cents: 5000, Status: vaca.DebtPending, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_, err := debts.InsertOne(context.Background(), debt)
	require.NoError(t, err)
	status, body = raw(t, s, http.MethodDelete, path, nil, s.org, "2")
	assert.Equal(t, http.StatusConflict, status)
	assert.Contains(t, body, "saldos_pendientes")
	_, err = debts.DeleteOne(context.Background(), bson.D{{Key: "_id", Value: debt.ID}})
	require.NoError(t, err)

	status, _ = raw(t, s, http.MethodDelete, path, nil, fr[0], "2")
	assert.Equal(t, http.StatusForbidden, status)
	status, _ = raw(t, s, http.MethodDelete, path, nil, s.org, "")
	assert.Equal(t, http.StatusBadRequest, status)
	status, body = raw(t, s, http.MethodDelete, path, nil, s.org, "2")
	require.Equal(t, http.StatusNoContent, status, body)

	// El plan sigue siendo del dueño; los integrantes ya no lo ven ni al grupo.
	planPath := "/api/v1/planes/" + s.plan.ID.Hex()
	assert.Equal(t, http.StatusOK, planCall(t, s.app, http.MethodGet, planPath, nil, s.org.token, "").status)
	assert.Equal(t, http.StatusNotFound, planCall(t, s.app, http.MethodGet, planPath, nil, fr[0].token, "").status)
	grupoCall(t, s.app, http.MethodGet, path, nil, fr[0].token, http.StatusNotFound)

	// Invitar otra vez al mismo plan crea un grupo nuevo, con otro id.
	again := grupoCall(t, s.app, http.MethodPost, planPath+"/invitaciones", map[string]any{"usuarios": []string{fr[0].id()}}, s.org.token, http.StatusOK)
	assert.NotEqual(t, v.ID, again.ID)
	assert.Empty(t, again.Name)
}

func TestGruposExtras_PuntoDeEncuentro(t *testing.T) {
	s := newGrupos(t)
	v, fr := s.confirmedGroup(t, "punto", 1)
	invited := s.friends(t, "punto_inv", 1)
	grupoCall(t, s.app, http.MethodPost, gruposPath+"/"+v.ID+"/integrantes", map[string]any{"usuarios": idsOf(invited)}, s.org.token, http.StatusOK)
	path := gruposPath + "/" + v.ID + "/punto-encuentro"
	at := planDayAt(s.plan, "11:30:00")

	grupoCall(t, s.app, http.MethodPut, path, map[string]any{"nombre": "Kiosco", "lat": 19.43, "lng": -99.13, "hora": at}, invited[0].token, http.StatusForbidden)
	grupoCall(t, s.app, http.MethodPut, path, map[string]any{"nombre": "Kiosco", "lat": 19.43, "lng": -99.13, "hora": time.Now().Add(-time.Hour).Format(time.RFC3339)}, fr[0].token, http.StatusUnprocessableEntity)
	grupoCall(t, s.app, http.MethodPut, path, map[string]any{"negocio_id": bson.NewObjectID().Hex(), "hora": at}, fr[0].token, http.StatusUnprocessableEntity)

	got := grupoCall(t, s.app, http.MethodPut, path, map[string]any{"nombre": "Kiosco", "lat": 19.43, "lng": -99.13, "hora": at}, fr[0].token, http.StatusOK)
	require.NotNil(t, got.MeetingPoint)
	assert.Equal(t, "Kiosco", got.MeetingPoint.Name)
	assert.Equal(t, fr[0].id(), got.MeetingPoint.ProposedBy)
	assert.Equal(t, 2, got.Version, "fijar el punto sube la versión")

	// Con negocio, el nombre y la ubicación salen del negocio; el último gana.
	got = grupoCall(t, s.app, http.MethodPut, path, map[string]any{"negocio_id": s.shop.Hex(), "hora": at}, s.org.token, http.StatusOK)
	assert.Equal(t, s.shop.Hex(), got.MeetingPoint.BusinessID)
	assert.InDelta(t, centerLat, got.MeetingPoint.Lat, 1e-6)
}

func TestGruposExtras_SugeridoYTraslados(t *testing.T) {
	s := newGrupos(t)
	v, fr := s.confirmedGroup(t, "sug", 2)
	base := gruposPath + "/" + v.ID
	share := func(who amigo, lat, lng float64) {
		got := grupoCall(t, s.app, http.MethodPatch, base+"/integrantes/yo", map[string]any{"ubicacion": map[string]float64{"lat": lat, "lng": lng}}, who.token, http.StatusOK)
		assertNoLocation(t, got)
	}
	share(fr[0], 19.4301234, -99.1331234)
	status, body := raw(t, s, http.MethodGet, base+"/punto-encuentro/sugerido", nil, fr[0], "")
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	assert.Contains(t, body, "faltan_ubicaciones")

	share(s.org, 19.4351234, -99.1341234)
	status, body = raw(t, s, http.MethodGet, base+"/punto-encuentro/sugerido", nil, fr[1], "")
	require.Equal(t, http.StatusOK, status, body)
	var sug grupos.SuggestResponse
	require.NoError(t, json.Unmarshal([]byte(body), &sug))
	assert.Equal(t, 2, sug.Participants)
	require.NotEmpty(t, sug.Places)
	assert.LessOrEqual(t, len(sug.Places), 3)
	for _, coord := range []string{"19.4301", "19.4351", "-99.1331", "-99.1341"} {
		assert.NotContains(t, body, coord, "ni el centro ni las ubicaciones")
	}
	assert.NotContains(t, body, "distancia_m")

	// Vencida = no cuenta.
	groups := s.env.Mongo.DB().Collection(grupos.Collection)
	_, err := groups.UpdateOne(context.Background(), bson.D{{Key: "_id", Value: mustID(t, v.ID)}, {Key: "integrantes.usuario_id", Value: s.org.user.ID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "integrantes.$.ubicacion_vence", Value: time.Now().Add(-time.Minute)}}}})
	require.NoError(t, err)
	status, _ = raw(t, s, http.MethodGet, base+"/punto-encuentro/sugerido", nil, fr[0], "")
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	share(s.org, 19.4351234, -99.1341234)

	// Traslados: sin punto = 404; con punto, exacto solo en la fila propia.
	status, body = raw(t, s, http.MethodGet, base+"/punto-encuentro/traslados", nil, fr[0], "")
	assert.Equal(t, http.StatusNotFound, status)
	assert.Contains(t, body, "sin_punto")
	grupoCall(t, s.app, http.MethodPut, base+"/punto-encuentro", map[string]any{"negocio_id": s.shop.Hex(), "hora": planDayAt(s.plan, "11:30:00")}, fr[0].token, http.StatusOK)
	status, body = raw(t, s, http.MethodGet, base+"/punto-encuentro/traslados", nil, fr[0], "")
	require.Equal(t, http.StatusOK, status, body)
	var tr grupos.TransfersResponse
	require.NoError(t, json.Unmarshal([]byte(body), &tr))
	require.Len(t, tr.Items, 3)
	mine := tr.Items[0]
	assert.True(t, mine.Mine)
	assert.Equal(t, fr[0].id(), mine.ID)
	require.NotNil(t, mine.Minutes)
	require.NotNil(t, mine.Cost)
	for _, row := range tr.Items[1:] {
		assert.Nil(t, row.Minutes)
		assert.Nil(t, row.Cost)
		if row.ID == fr[1].id() {
			assert.True(t, row.NoLocation)
		} else {
			assert.NotEmpty(t, row.Range)
			assert.NotEmpty(t, row.CostRange)
		}
	}

	// Retirar la ubicación.
	got := grupoCall(t, s.app, http.MethodPatch, base+"/integrantes/yo", map[string]any{"compartir_ubicacion": false}, fr[0].token, http.StatusOK)
	for _, m := range got.Members {
		if m.UserID == fr[0].id() {
			assert.False(t, m.SharesLocation)
		}
	}
	grupoCall(t, s.app, http.MethodPatch, base+"/integrantes/yo", map[string]any{}, fr[0].token, http.StatusUnprocessableEntity)
}

// assertNoLocation: la vista solo dice si alguien comparte, nunca dónde está.
func assertNoLocation(t *testing.T, v grupos.View) {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "ubicacion_compartida")
	assert.NotContains(t, string(b), "19.43")
}

func TestGruposExtras_Sorpresa(t *testing.T) {
	s := newGrupos(t)
	tomorrow := time.Now().In(time.FixedZone("MX", -6*3600)).AddDate(0, 0, 1)
	s.plan = s.newPlanOn(t, s.org, tomorrow)
	v, fr := s.confirmedGroup(t, "sorp", 2)
	target, other := fr[0], fr[1]
	path := gruposPath + "/" + v.ID + "/sorpresa"
	reveal := planDayAt(s.plan, "20:00:00")

	grupoCall(t, s.app, http.MethodPut, path, map[string]any{"activo": true, "para_usuario_id": target.id(), "revelar_en": reveal}, other.token, http.StatusForbidden)
	grupoCall(t, s.app, http.MethodPut, path, map[string]any{"activo": true, "para_usuario_id": s.org.id(), "revelar_en": reveal}, s.org.token, http.StatusUnprocessableEntity)
	grupoCall(t, s.app, http.MethodPut, path, map[string]any{"activo": true, "para_usuario_id": target.id(), "revelar_en": planDayAt(s.plan, "20:00:00")[:10] + "T23:59:59-08:00"}, s.org.token, http.StatusUnprocessableEntity)

	// Una votación abierta antes de la sorpresa.
	opts := insertPlacesNear(t, s, "sorp")
	vote := map[string]any{"parada_id": s.plan.Paradas[0].ID.Hex(), "opciones": opts, "cierra_en": time.Now().Add(time.Hour).Format(time.RFC3339)}
	created := decode(t, sendJSON(t, s.app, http.MethodPost, gruposPath+"/"+v.ID+"/votaciones", vote, s.org.token))
	voteID, _ := created["id"].(string)
	require.NotEmpty(t, voteID, created)

	got := grupoCall(t, s.app, http.MethodPut, path, map[string]any{"activo": true, "para_usuario_id": target.id(), "revelar_en": reveal}, s.org.token, http.StatusOK)
	require.NotNil(t, got.Surprise)
	assert.Equal(t, target.id(), got.Surprise.For, "los demás ven para quién es")

	planPath := "/api/v1/planes/" + s.plan.ID.Hex()
	hidden := planCall(t, s.app, http.MethodGet, planPath, nil, target.token, "")
	require.Equal(t, http.StatusOK, hidden.status, hidden.body)
	hp := hidden.plan(t)
	assert.Empty(t, hp.Paradas)
	assert.Empty(t, hp.Traslados)
	require.NotNil(t, hp.Sorpresa)
	require.NotNil(t, hp.Sorpresa.Inicio)
	assert.NotContains(t, hidden.body, opts[0])
	assert.Len(t, planCall(t, s.app, http.MethodGet, planPath, nil, other.token, "").plan(t).Paradas, 1)

	pkg := planCall(t, s.app, http.MethodGet, planPath+"/paquete-offline", nil, target.token, "")
	require.Equal(t, http.StatusOK, pkg.status, pkg.body)
	assert.NotContains(t, pkg.body, s.plan.Paradas[0].ID.Hex(), "el paquete tampoco trae paradas")
	assert.Contains(t, pkg.body, `"lugares":[]`)

	mineGroup := grupoCall(t, s.app, http.MethodGet, gruposPath+"/"+v.ID, nil, target.token, http.StatusOK)
	require.NotNil(t, mineGroup.Surprise)
	assert.Empty(t, mineGroup.Surprise.For)

	listed := decode(t, sendJSON(t, s.app, http.MethodGet, gruposPath+"/"+v.ID+"/votaciones", nil, target.token))
	assert.Empty(t, listed["items"], "la persona sorprendida no ve las votaciones")
	assert.Equal(t, http.StatusNotFound, sendJSON(t, s.app, http.MethodPut, "/api/v1/votaciones/"+voteID+"/voto", map[string]string{"negocio_id": opts[0]}, target.token).StatusCode)
	otherList := decode(t, sendJSON(t, s.app, http.MethodGet, gruposPath+"/"+v.ID+"/votaciones", nil, other.token))
	assert.Len(t, otherList["items"], 1)

	// A la hora de revelar ve todo (se decide al leer).
	_, err := s.env.Mongo.DB().Collection(grupos.Collection).UpdateOne(context.Background(), bson.D{{Key: "_id", Value: mustID(t, v.ID)}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "sorpresa.revelar_en", Value: time.Now().Add(-time.Second)}}}})
	require.NoError(t, err)
	shown := planCall(t, s.app, http.MethodGet, planPath, nil, target.token, "").plan(t)
	assert.Len(t, shown.Paradas, 1)
	assert.Nil(t, shown.Sorpresa)

	// Apagarla.
	off := grupoCall(t, s.app, http.MethodPut, path, map[string]any{"activo": false}, s.org.token, http.StatusOK)
	assert.Nil(t, off.Surprise)
}

// newPlanOn crea un plan de who para `day`.
func (s gruposSetup) newPlanOn(t *testing.T, who amigo, day time.Time) planes.Plan {
	t.Helper()
	d := day.Format(planes.DayLayout)
	body := map[string]any{
		"nombre": "Sorpresa", "fecha": d, "personas": 3, "presupuesto": 2000, "movilidad": "a_pie",
		"paradas": []map[string]any{{"negocio_id": s.shop.Hex(), "llega": d + "T12:00:00-06:00", "minutos": 60, "precio": 100}},
	}
	got := planCall(t, s.app, http.MethodPost, "/api/v1/planes", body, who.token, "")
	require.Equal(t, http.StatusCreated, got.status, got.body)
	return got.plan(t)
}

// insertPlacesNear siembra dos negocios visibles para una votación.
func insertPlacesNear(t *testing.T, s gruposSetup, prefix string) []string {
	t.Helper()
	b := placeDoc(placeSpec{slug: prefix + "-b", dLat: 0.002})
	c := placeDoc(placeSpec{slug: prefix + "-c", dLat: 0.003})
	_, err := s.env.Mongo.DB().Collection("negocios").InsertMany(context.Background(), []any{b, c})
	require.NoError(t, err)
	return []string{b.ID.Hex(), c.ID.Hex()}
}

func (s gruposSetup) makePublic(t *testing.T) {
	t.Helper()
	got := planCall(t, s.app, http.MethodPatch, "/api/v1/planes/"+s.plan.ID.Hex(), map[string]string{"visibilidad": "publico"}, s.org.token, strconv.Itoa(s.plan.Version))
	require.Equal(t, http.StatusOK, got.status, got.body)
}

func TestGruposExtras_MeApunto(t *testing.T) {
	s := newGrupos(t)
	path := "/api/v1/planes/" + s.plan.ID.Hex() + "/me-apunto"
	fr := s.friends(t, "apunto", 3)
	stranger := seedAmigo(t, s.env, "apunto_ajeno")

	grupoCall(t, s.app, http.MethodPost, path, nil, fr[0].token, http.StatusForbidden)
	grupoCall(t, s.app, http.MethodPost, path, nil, stranger.token, http.StatusNotFound)
	grupoCall(t, s.app, http.MethodPost, "/api/v1/planes/"+bson.NewObjectID().Hex()+"/me-apunto", nil, fr[0].token, http.StatusNotFound)
	s.makePublic(t)
	grupoCall(t, s.app, http.MethodPost, path, nil, stranger.token, http.StatusNotFound)

	// Dos apuntes a la vez crean un solo grupo.
	var wg sync.WaitGroup
	views := make([]grupos.View, 2)
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			views[i] = grupoCall(t, s.app, http.MethodPost, path, nil, fr[i].token, http.StatusOK)
		}(i)
	}
	wg.Wait()
	assert.Equal(t, views[0].ID, views[1].ID)
	again := grupoCall(t, s.app, http.MethodPost, path, nil, fr[0].token, http.StatusOK)
	assert.Equal(t, grupos.StatusConfirmed, again.MyStatus)
	assert.Equal(t, 3, again.Seats.Occupied, "repetir no ocupa otro lugar")

	// Con bloqueo = 404.
	friends := s.env.Mongo.DB().Collection(amistades.Collection)
	_, err := friends.UpdateOne(context.Background(), bson.D{{Key: "par", Value: amistades.PairKey(s.org.user.ID, fr[2].user.ID)}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: amistades.StatusBlocked}}}})
	require.NoError(t, err)
	grupoCall(t, s.app, http.MethodPost, path, nil, fr[2].token, http.StatusNotFound)

	// Lleno = 409.
	groups := s.env.Mongo.DB().Collection(grupos.Collection)
	full := s.friends(t, "apunto_lleno", 1)[0]
	_, err = groups.UpdateOne(context.Background(), bson.D{{Key: "_id", Value: mustID(t, views[0].ID)}}, bson.D{{Key: "$set", Value: bson.D{{Key: "ocupados", Value: grupos.MaxMembers}}}})
	require.NoError(t, err)
	grupoCall(t, s.app, http.MethodPost, path, nil, full.token, http.StatusConflict)

	grupoCall(t, s.app, http.MethodDelete, path, nil, fr[1].token, http.StatusNoContent)
	grupoCall(t, s.app, http.MethodGet, gruposPath+"/"+views[0].ID, nil, fr[1].token, http.StatusNotFound)
	grupoCall(t, s.app, http.MethodDelete, path, nil, s.org.token, http.StatusConflict)
}

// nv-69: sugerido y traslados cuestan una lectura autenticada más la ventana
// `grupos_ubicacion`, igual que cualquier otra ventana por usuario.
func TestPresupuestoRedis_UbicacionDelGrupoNV69(t *testing.T) {
	s := newGrupos(t)
	v, fr := s.confirmedGroup(t, "pres69", 1)
	base := gruposPath + "/" + v.ID
	read := commandsOf(t, func() {
		assert.Equal(t, http.StatusOK, sendJSON(t, s.app, http.MethodGet, base, nil, fr[0].token).StatusCode)
	})
	window := commandsOf(t, func() {
		assert.Equal(t, http.StatusOK, sendJSON(t, s.app, http.MethodGet, "/api/v1/personas/buscar?q=nadie", nil, fr[0].token).StatusCode)
	}) - commandsOf(t, func() {
		assert.Equal(t, http.StatusOK, sendJSON(t, s.app, http.MethodGet, "/api/v1/usuarios/yo", nil, fr[0].token).StatusCode)
	})
	require.Positive(t, window)
	assert.Equal(t, read+window, commandsOf(t, func() {
		assert.Equal(t, http.StatusUnprocessableEntity, sendJSON(t, s.app, http.MethodGet, base+"/punto-encuentro/sugerido", nil, fr[0].token).StatusCode)
	}), "sugerido: lectura autenticada + ventana")
	assert.Equal(t, read+window, commandsOf(t, func() {
		assert.Equal(t, http.StatusNotFound, sendJSON(t, s.app, http.MethodGet, base+"/punto-encuentro/traslados", nil, fr[0].token).StatusCode)
	}), "traslados: lectura autenticada + ventana")
}

// La ubicación propia entra en el ARCO de grupos; la tarea nocturna borra las vencidas y el
// chat de los grupos disueltos.
func TestGruposExtras_ArcoYTareasNocturnas(t *testing.T) {
	s := newGrupos(t)
	v, fr := s.confirmedGroup(t, "noche", 1)
	base := gruposPath + "/" + v.ID
	grupoCall(t, s.app, http.MethodPatch, base+"/integrantes/yo", map[string]any{"ubicacion": map[string]float64{"lat": 19.4301234, "lng": -99.1331234}}, fr[0].token, http.StatusOK)
	grupoCall(t, s.app, http.MethodPatch, base+"/integrantes/yo", map[string]any{"ubicacion": map[string]float64{"lat": 19.4551234, "lng": -99.1551234}}, s.org.token, http.StatusOK)

	repo := grupos.NewRepository(s.env.Mongo.DB())
	exported, err := grupos.NewPrivacy(repo).Export(context.Background(), fr[0].user.ID)
	require.NoError(t, err)
	out, err := json.Marshal(exported)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"ubicacion_compartida":{"lat":19.43,"lng":-99.133}`, "redondeada a 3 decimales")
	assert.NotContains(t, string(out), "19.455", "la del organizador no")

	groupID := mustID(t, v.ID)
	_, err = s.env.Mongo.DB().Collection(grupos.Collection).UpdateOne(context.Background(), bson.D{{Key: "_id", Value: groupID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "integrantes.$[].ubicacion_vence", Value: time.Now().Add(-time.Minute)}}}})
	require.NoError(t, err)
	n, err := grupos.NewLocationsTask(repo).Run(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1)
	g, err := repo.FindByID(context.Background(), groupID)
	require.NoError(t, err)
	for _, m := range g.Members {
		assert.Nil(t, m.SharedLocation)
		assert.Nil(t, m.LocationExpires)
	}

	messages := s.env.Mongo.DB().Collection(chat.Collection)
	_, err = messages.InsertOne(context.Background(), chat.Message{ID: bson.NewObjectID(), GroupID: groupID})
	require.NoError(t, err)
	status, body := raw(t, s, http.MethodDelete, base, nil, s.org, "1")
	require.Equal(t, http.StatusNoContent, status, body)
	_, err = chat.NewDissolvedTask(chat.NewRepository(s.env.Mongo.DB()), repo).Run(context.Background())
	require.NoError(t, err)
	left, err := messages.CountDocuments(context.Background(), bson.D{{Key: "grupo_id", Value: groupID}})
	require.NoError(t, err)
	assert.Zero(t, left, "el chat del grupo disuelto se borra")
}
