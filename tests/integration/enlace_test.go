package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
)

const (
	compartidosPath = "/api/v1/planes/compartidos"
	unirsePath      = "/api/v1/grupos/unirse"
)

func (s gruposSetup) share(t *testing.T) grupos.ShareResponse {
	t.Helper()
	resp := sendJSON(t, s.app, http.MethodPost, planPath(s.plan, "/compartir"), nil, s.org.token)
	raw := readAll(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode, raw)
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	var out grupos.ShareResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	return out
}

func TestEnlace_CompartirVerYUnirse(t *testing.T) {
	s := newGrupos(t)
	link := s.share(t)
	assert.Len(t, link.Code, 22)
	assert.True(t, strings.HasSuffix(link.Link, "/plan#codigo="+link.Code), "el código va en el fragmento")
	assert.WithinDuration(t, time.Now().Add(7*24*time.Hour), link.ExpiresAt, time.Minute)

	// Vista pública sin sesión: el plan y sus lugares, sin personas.
	resp := sendJSON(t, s.app, http.MethodPost, compartidosPath, map[string]string{"codigo": link.Code}, "")
	raw := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	var view grupos.PublicPlan
	require.NoError(t, json.Unmarshal([]byte(raw), &view))
	assert.Equal(t, "Salida", view.Name)
	require.Len(t, view.Stops, 1)
	require.NotNil(t, view.Stops[0].Place)
	assert.Equal(t, s.shop.Hex(), view.Stops[0].Place.ID)
	assert.NotContains(t, raw, s.org.id())
	assert.NotContains(t, raw, "organiza")

	// Alguien que no es amigo entra confirmado; repetir no cambia nada.
	guest := seedAmigo(t, s.env, "por_enlace")
	v := grupoCall(t, s.app, http.MethodPost, unirsePath, map[string]string{"codigo": link.Code}, guest.token, http.StatusOK)
	assert.Equal(t, link.GroupID, v.ID)
	assert.Equal(t, grupos.StatusConfirmed, v.MyStatus)
	v = grupoCall(t, s.app, http.MethodPost, unirsePath, map[string]string{"codigo": link.Code}, guest.token, http.StatusOK)
	assert.Equal(t, 2, v.Seats.Occupied)
	// Ya puede leer el plan.
	assert.Equal(t, http.StatusOK, planCall(t, s.app, http.MethodGet, planPath(s.plan, ""), nil, guest.token, "").status)
}

func TestEnlace_InvitadoODeclinadoQuedaConfirmado(t *testing.T) {
	s := newGrupos(t)
	fr := s.friends(t, "enl", 2)
	g := s.create(t, fr)
	grupoCall(t, s.app, http.MethodPatch, gruposPath+"/"+g.ID+"/integrantes/yo", map[string]string{"estado": "declinado"}, fr[1].token, http.StatusOK)
	link := s.share(t)
	assert.Equal(t, g.ID, link.GroupID, "comparte el grupo que ya existe")
	for _, who := range fr {
		v := grupoCall(t, s.app, http.MethodPost, unirsePath, map[string]string{"codigo": link.Code}, who.token, http.StatusOK)
		assert.Equal(t, grupos.StatusConfirmed, v.MyStatus)
	}
	org := grupoCall(t, s.app, http.MethodGet, gruposPath+"/"+g.ID, nil, s.org.token, http.StatusOK)
	assert.Equal(t, 3, org.Seats.Occupied)
	assert.Equal(t, 3, org.Summary.Confirmed)
}

func TestEnlace_RevocadoReemplazadoYVencido(t *testing.T) {
	s := newGrupos(t)
	guest := seedAmigo(t, s.env, "tarde")
	old := s.share(t)
	current := s.share(t)
	// El enlace anterior deja de servir al crear otro.
	assert.Equal(t, http.StatusNotFound, sendJSON(t, s.app, http.MethodPost, compartidosPath, map[string]string{"codigo": old.Code}, "").StatusCode)
	grupoCall(t, s.app, http.MethodPost, unirsePath, map[string]string{"codigo": old.Code}, guest.token, http.StatusNotFound)

	// Vencido: la vista pública da 404 y unirse 410.
	_, err := s.env.Mongo.DB().Collection(grupos.Collection).UpdateOne(context.Background(),
		bson.D{{Key: "_id", Value: mustID(t, current.GroupID)}}, bson.D{{Key: "$set", Value: bson.D{{Key: "codigo_vence", Value: time.Now().Add(-time.Minute)}}}})
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, sendJSON(t, s.app, http.MethodPost, compartidosPath, map[string]string{"codigo": current.Code}, "").StatusCode)
	grupoCall(t, s.app, http.MethodPost, unirsePath, map[string]string{"codigo": current.Code}, guest.token, http.StatusGone)

	// Revocado: 204 y después 404; revocar otra vez = 404.
	fresh := s.share(t)
	resp := sendJSON(t, s.app, http.MethodDelete, planPath(s.plan, "/compartir"), nil, s.org.token)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, http.StatusNotFound, sendJSON(t, s.app, http.MethodPost, compartidosPath, map[string]string{"codigo": fresh.Code}, "").StatusCode)
	assert.Equal(t, http.StatusNotFound, sendJSON(t, s.app, http.MethodDelete, planPath(s.plan, "/compartir"), nil, s.org.token).StatusCode)
}

func TestEnlace_PlanAjenoBloqueoYPlanBorrado(t *testing.T) {
	s := newGrupos(t)
	other := seedAmigo(t, s.env, "no_dueno")
	assert.Equal(t, http.StatusNotFound, sendJSON(t, s.app, http.MethodPost, planPath(s.plan, "/compartir"), nil, other.token).StatusCode)

	link := s.share(t)
	blocked := seedAmigo(t, s.env, "bloqueado")
	now := time.Now().UTC()
	require.NoError(t, amistades.NewRepository(s.env.Mongo.DB()).Insert(context.Background(), &amistades.Friendship{
		ID: bson.NewObjectID(), From: s.org.user.ID, To: blocked.user.ID, Status: amistades.StatusBlocked,
		Pair: amistades.PairKey(s.org.user.ID, blocked.user.ID), CreatedAt: now, UpdatedAt: now,
	}))
	grupoCall(t, s.app, http.MethodPost, unirsePath, map[string]string{"codigo": link.Code}, blocked.token, http.StatusNotFound)

	require.Equal(t, http.StatusNoContent, planCall(t, s.app, http.MethodDelete, planPath(s.plan, ""), nil, s.org.token, `"1"`).status)
	assert.Equal(t, http.StatusNotFound, sendJSON(t, s.app, http.MethodPost, compartidosPath, map[string]string{"codigo": link.Code}, "").StatusCode)
}

func TestEnlace_GrupoLlenoEs409(t *testing.T) {
	s := newGrupos(t)
	s.create(t, s.friends(t, "cupo", 11))
	link := s.share(t)
	late := seedAmigo(t, s.env, "sin_lugar")
	grupoCall(t, s.app, http.MethodPost, unirsePath, map[string]string{"codigo": link.Code}, late.token, http.StatusConflict)
}
