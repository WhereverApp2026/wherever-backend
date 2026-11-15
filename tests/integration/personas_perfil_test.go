package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/personas"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

func perfil(t *testing.T, app *fiber.App, who amigo, id string) (int, string) {
	t.Helper()
	resp := sendJSON(t, app, http.MethodGet, "/api/v1/personas/"+id, nil, who.token)
	return resp.StatusCode, readAll(t, resp)
}

func TestPersonasPerfil_BasicoCompletoY404(t *testing.T) {
	env, app := grafoEnv(t)
	env.Personas.WithGraph(neo4jDriver)
	me := seedAmigo(t, env, "perfil_yo")
	friend := seedAmigo(t, env, "perfil_amiga")
	stranger := seedAmigo(t, env, "perfil_ajeno")
	common := seedAmigo(t, env, "perfil_comun")
	relacion(t, env, me.user, friend.user, amistades.StatusAccepted)
	relacion(t, env, me.user, common.user, amistades.StatusAccepted)
	relacion(t, env, friend.user, common.user, amistades.StatusAccepted)
	proyectar(t, env)
	now := time.Now().UTC()
	g := grupos.Group{ID: bson.NewObjectID(), PlanID: bson.NewObjectID(), OrganizerID: me.user.ID, Occupied: 2, CreatedAt: now, UpdatedAt: now,
		Members: []grupos.Member{
			{UserID: me.user.ID, Role: grupos.RoleOrganizer, Status: grupos.StatusConfirmed, Since: now},
			{UserID: friend.user.ID, Role: grupos.RoleMember, Status: grupos.StatusInvited, Since: now},
		}}
	require.NoError(t, grupos.NewRepository(env.Mongo.DB()).Create(context.Background(), &g))

	status, body := perfil(t, app, me, stranger.id())
	require.Equal(t, http.StatusOK, status, body)
	var basic map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &basic))
	assert.Equal(t, personas.RelationNone, basic["relacion"])
	for _, hidden := range []string{"nivel", "amigos_en_comun", "grupos_en_comun", "silenciado"} {
		assert.NotContains(t, basic, hidden)
	}
	assertSnakeCase(t, basic)

	status, body = perfil(t, app, me, friend.id())
	require.Equal(t, http.StatusOK, status, body)
	var full personas.ProfileResponse
	require.NoError(t, json.Unmarshal([]byte(body), &full))
	assert.Equal(t, personas.RelationFriend, full.Relation)
	require.NotNil(t, full.FriendExtras)
	require.NotNil(t, full.CommonFriends)
	assert.Equal(t, int64(1), *full.CommonFriends)
	assert.Equal(t, int64(1), full.CommonGroups)
	assert.False(t, full.Degraded)

	// Propio, inexistente, inactivo y bloqueado en los dos sentidos = el mismo 404.
	inactive := seedPersona(t, env, "perfil_baja@amigos.mx", "perfil_baja", personaOpts{status: usuarios.StatusPendingDeletion})
	blocker := seedAmigo(t, env, "perfil_bloquea")
	blocked := seedAmigo(t, env, "perfil_bloqueado")
	relacion(t, env, blocker.user, me.user, amistades.StatusBlocked)
	relacion(t, env, me.user, blocked.user, amistades.StatusBlocked)
	shape := func(body string) map[string]any {
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(body), &m), body)
		delete(m, "traza_id") // cambia en cada petición
		return m
	}
	_, missing := perfil(t, app, me, bson.NewObjectID().Hex())
	for _, id := range []string{me.id(), inactive.ID.Hex(), blocker.id(), blocked.id()} {
		status, body := perfil(t, app, me, id)
		assert.Equal(t, http.StatusNotFound, status, id)
		assert.Equal(t, shape(missing), shape(body), "el mismo 404")
	}
	status, _ = perfil(t, app, me, "no-es-id")
	assert.Equal(t, http.StatusUnprocessableEntity, status)
}

func TestPersonasPerfil_Neo4jCaidoEsDegradado(t *testing.T) {
	env, app := amistadesEnv(t)
	dead, err := neo4j.New("neo4j://127.0.0.1:1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = dead.Close(context.Background()) })
	env.Personas.WithGraph(dead)
	me := seedAmigo(t, env, "perfil_caido")
	friend := seedAmigo(t, env, "perfil_caido_amiga")
	relacion(t, env, me.user, friend.user, amistades.StatusAccepted)
	status, body := perfil(t, app, me, friend.id())
	require.Equal(t, http.StatusOK, status, body)
	assert.Contains(t, body, `"amigos_en_comun":null`)
	assert.Contains(t, body, `"degradado":true`)
}

func TestPersonasPerfil_Limite(t *testing.T) {
	env, app := amistadesEnv(t)
	env.ClearRateLimits(t)
	me := seedAmigo(t, env, "perfil_limite")
	other := seedAmigo(t, env, "perfil_limite_otra")
	for i := 0; i < 120; i++ {
		status, _ := perfil(t, app, me, other.id())
		require.Equal(t, http.StatusOK, status, i)
	}
	status, _ := perfil(t, app, me, other.id())
	assert.Equal(t, http.StatusTooManyRequests, status)
}

func TestAmistadesSilencio_MarcaPropiaEIdempotente(t *testing.T) {
	env, app := amistadesEnv(t)
	me := seedAmigo(t, env, "silencio_yo")
	friend := seedAmigo(t, env, "silencio_amiga")
	pending := seedAmigo(t, env, "silencio_pendiente")
	outsider := seedAmigo(t, env, "silencio_ajeno")
	relacion(t, env, me.user, friend.user, amistades.StatusAccepted)
	relacion(t, env, me.user, pending.user, amistades.StatusPending)
	repo := amistades.NewRepository(env.Mongo.DB())
	f, _, err := repo.FindByPair(context.Background(), me.user.ID, friend.user.ID)
	require.NoError(t, err)
	p, _, err := repo.FindByPair(context.Background(), me.user.ID, pending.user.ID)
	require.NoError(t, err)
	path := "/api/v1/amistades/" + f.ID.Hex() + "/silencio"

	for range 2 {
		got := notifCall(t, app, http.MethodPut, path, map[string]bool{"silenciado": true}, me.token, http.StatusOK)
		assert.Equal(t, true, got["silenciado"])
	}
	f, _, err = repo.FindByPair(context.Background(), me.user.ID, friend.user.ID)
	require.NoError(t, err)
	assert.Equal(t, []bson.ObjectID{me.user.ID}, f.MutedBy, "solo mi marca, una vez")

	mine := notifCall(t, app, http.MethodGet, "/api/v1/amistades", nil, me.token, http.StatusOK)
	assert.Equal(t, true, obj(t, list(t, mine["resultados"])[0])["silenciado"])
	theirs := notifCall(t, app, http.MethodGet, "/api/v1/amistades", nil, friend.token, http.StatusOK)
	assert.Equal(t, false, obj(t, list(t, theirs["resultados"])[0])["silenciado"], "el otro no se entera")
	status, body := perfil(t, app, friend, me.id())
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, `"silenciado":false`)

	notifCall(t, app, http.MethodPut, "/api/v1/amistades/"+p.ID.Hex()+"/silencio", map[string]bool{"silenciado": true}, me.token, http.StatusNotFound)
	notifCall(t, app, http.MethodPut, path, map[string]bool{"silenciado": true}, outsider.token, http.StatusNotFound)
	notifCall(t, app, http.MethodPut, path, map[string]any{"silenciado": true, "extra": 1}, me.token, http.StatusBadRequest)
	notifCall(t, app, http.MethodPut, path, map[string]any{"silenciado": "si"}, me.token, http.StatusBadRequest)
	notifCall(t, app, http.MethodPut, path, map[string]any{}, me.token, http.StatusUnprocessableEntity)
	notifCall(t, app, http.MethodPut, path, map[string]bool{"silenciado": false}, me.token, http.StatusOK)

	// Borrar la amistad borra la marca con el documento.
	notifCall(t, app, http.MethodPut, path, map[string]bool{"silenciado": true}, me.token, http.StatusOK)
	notifCall(t, app, http.MethodDelete, "/api/v1/amistades/"+f.ID.Hex(), nil, me.token, http.StatusNoContent)
	_, ok, err := repo.FindByPair(context.Background(), me.user.ID, friend.user.ID)
	require.NoError(t, err)
	assert.False(t, ok)
}

func contactos(t *testing.T, app *fiber.App, who amigo, phones []string) (int, string) {
	t.Helper()
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/personas/contactos", map[string]any{"telefonos": phones}, who.token)
	return resp.StatusCode, readAll(t, resp)
}

func TestPersonasContactos_EncuentraSinGuardarNada(t *testing.T) {
	env, app := amistadesEnv(t)
	env.ClearRateLimits(t)
	me := seedAmigo(t, env, "contactos_yo")
	_, err := env.Users.Update(context.Background(), me.user.ID, bson.D{{Key: "telefono_hash", Value: usuarios.HashPhone(env.Cfg.PhoneHMACKey, "+525500000001")}, {Key: "privacidad.buscable_por_telefono", Value: true}})
	require.NoError(t, err)
	ana := seedPersona(t, env, "ana@contactos.mx", "contactos_ana", personaOpts{phone: "+525511111111", phoneSearch: true})
	seedPersona(t, env, "oculta@contactos.mx", "contactos_oculta", personaOpts{phone: "+525522222222"})
	seedPersona(t, env, "baja@contactos.mx", "contactos_baja", personaOpts{phone: "+525533333333", phoneSearch: true, status: usuarios.StatusPendingDeletion})
	blocked := seedPersona(t, env, "bloq@contactos.mx", "contactos_bloq", personaOpts{phone: "+525544444444", phoneSearch: true})
	relacion(t, env, blocked, me.user, amistades.StatusBlocked)
	friend := seedPersona(t, env, "amiga@contactos.mx", "contactos_amiga", personaOpts{phone: "+12025550143", phoneSearch: true})
	relacion(t, env, me.user, friend, amistades.StatusAccepted)

	before := countAll(t, env)
	status, body := contactos(t, app, me, []string{"basura", "55 1111 1111", "5500000001", "5522222222", "5533333333", "5544444444", "+1 202 555 0143", "+525511111111"})
	require.Equal(t, http.StatusOK, status, body)
	var out personas.ContactsResponse
	require.NoError(t, json.Unmarshal([]byte(body), &out))
	require.Len(t, out.Results, 2, body)
	assert.Equal(t, 1, out.Results[0].Index, "el primero donde apareció")
	assert.Equal(t, ana.ID.Hex(), out.Results[0].Person.ID)
	assert.Equal(t, personas.RelationNone, out.Results[0].Relation)
	assert.Equal(t, 6, out.Results[1].Index)
	assert.Equal(t, personas.RelationFriend, out.Results[1].Relation)
	for _, leak := range []string{"5511111111", "telefono", usuarios.HashPhone(env.Cfg.PhoneHMACKey, "+525511111111")} {
		assert.NotContains(t, body, leak)
		assert.NotContains(t, env.Logs.String(), leak)
	}
	assert.Equal(t, before, countAll(t, env), "la petición no crea documentos")

	status, _ = contactos(t, app, me, []string{"hola", "123"})
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	status, _ = contactos(t, app, me, make([]string, 501))
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/personas/contactos", map[string]any{"telefonos": []string{"5511111111"}, "hashes": []string{}}, me.token)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "campo de más")

	// 10 por día por usuario.
	env.ClearRateLimits(t)
	for i := range 10 {
		status, _ := contactos(t, app, me, []string{"5511111111"})
		require.Equal(t, http.StatusOK, status, i)
	}
	status, _ = contactos(t, app, me, []string{"5511111111"})
	assert.Equal(t, http.StatusTooManyRequests, status)
}

// countAll cuenta los documentos de todas las colecciones de la base.
func countAll(t *testing.T, env *authtest.Env) int64 {
	t.Helper()
	db := env.Mongo.DB()
	names, err := db.ListCollectionNames(context.Background(), bson.D{})
	require.NoError(t, err)
	var total int64
	for _, name := range names {
		n, err := db.Collection(name).CountDocuments(context.Background(), bson.D{})
		require.NoError(t, err)
		total += n
	}
	return total
}

// nv-70: el perfil cuesta lo mismo que `personas_buscar` (lectura + ventana) y los contactos,
// por ser POST, una escritura autenticada + ventana.
func TestPresupuestoRedis_VentanasPorUsuarioNV70(t *testing.T) {
	env, app := amistadesEnv(t)
	env.ClearRateLimits(t)
	me := seedAmigo(t, env, "presupuesto70")
	other := seedAmigo(t, env, "presupuesto70_otra")
	read := commandsOf(t, func() {
		assert.Equal(t, http.StatusOK, sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, me.token).StatusCode)
	})
	search := commandsOf(t, func() {
		assert.Equal(t, http.StatusOK, sendJSON(t, app, http.MethodGet, "/api/v1/personas/buscar?q=nadie", nil, me.token).StatusCode)
	})
	assert.Equal(t, search, commandsOf(t, func() {
		status, _ := perfil(t, app, me, other.id())
		assert.Equal(t, http.StatusOK, status)
	}), "perfil = buscar")
	write := commandsOf(t, func() {
		resp := sendJSON(t, app, http.MethodPatch, "/api/v1/usuarios/yo/privacidad", map[string]any{"datos_sensibles": false}, me.token)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
	window := search - read
	require.Positive(t, window)
	assert.Equal(t, write+window, commandsOf(t, func() {
		status, _ := contactos(t, app, me, []string{"5599999999"})
		assert.Equal(t, http.StatusOK, status)
	}), "contactos: escritura autenticada + ventana")
}
