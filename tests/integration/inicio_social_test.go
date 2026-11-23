package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
	"github.com/WhereverApp2026/wherever-backend/internal/resenas"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/votaciones"
)

const (
	planesAmigosPath    = "/api/v1/planes/amigos"
	historiasPath       = "/api/v1/momentos/historias"
	votosPendientesPath = "/api/v1/votaciones/pendientes"
)

// ini84Plan siembra un plan activo de owner con una parada en el lugar.
func ini84Plan(t *testing.T, env *authtest.Env, owner amigo, nombre, vis string, fecha, creado time.Time, lugar bson.ObjectID) planes.Plan {
	t.Helper()
	// El plan es un día de México y la parada llega a las 11:00 (los lugares abren de 9 a 18).
	local := fecha.In(negocios.MexicoCity())
	fecha = time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location()).UTC()
	p := planes.Plan{ID: bson.NewObjectID(), OwnerID: owner.user.ID, Nombre: nombre, Fecha: fecha, Personas: 2, Movilidad: planes.MobilityWalk,
		Paradas:   []planes.Parada{{ID: bson.NewObjectID(), NegocioID: lugar, Llega: fecha.Add(11 * time.Hour), Minutos: 60, Precio: 210, Nota: "nota privada"}},
		Traslados: []planes.Traslado{}, Estado: planes.StatusActive, Origen: planes.OriginManual, Version: 1, Visibilidad: vis,
		CostoPorPersona: 420, CreadoEn: creado, ActualizadoEn: creado}
	_, err := env.Mongo.DB().Collection(planes.Collection).InsertOne(context.Background(), p)
	require.NoError(t, err)
	return p
}

// ini84Lugar siembra un negocio aprobado y devuelve su id.
func ini84Lugar(t *testing.T, env *authtest.Env, slug string) bson.ObjectID {
	t.Helper()
	lugar := placeDoc(placeSpec{slug: slug})
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), lugar)
	require.NoError(t, err)
	return lugar.ID
}

// ini84Social cambia `privacidad.social` de la persona directo en la base.
func ini84Social(t *testing.T, env *authtest.Env, who amigo, campo string, valor any) {
	t.Helper()
	_, err := env.Mongo.DB().Collection("usuarios").UpdateOne(context.Background(), bson.D{{Key: "_id", Value: who.user.ID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "privacidad.social." + campo, Value: valor}}}})
	require.NoError(t, err)
}

// ini84Grupo crea el grupo del plan con el dueño de organizador y los integrantes en estado.
func ini84Grupo(t *testing.T, env *authtest.Env, p planes.Plan, estado string, members ...amigo) grupos.Group {
	t.Helper()
	now := time.Now().UTC()
	g := grupos.Group{ID: bson.NewObjectID(), PlanID: p.ID, OrganizerID: p.OwnerID, Version: 1, CreatedAt: now, UpdatedAt: now,
		Members: []grupos.Member{{UserID: p.OwnerID, Role: grupos.RoleOrganizer, Status: grupos.StatusConfirmed, Since: now}}}
	for _, m := range members {
		g.Members = append(g.Members, grupos.Member{UserID: m.user.ID, Role: grupos.RoleMember, Status: estado, Since: now})
	}
	g.Occupied = len(g.Members)
	require.NoError(t, grupos.NewRepository(env.Mongo.DB()).Create(context.Background(), &g))
	return g
}

func ini84IDs(t *testing.T, body map[string]any) []string {
	t.Helper()
	items := list(t, body["items"])
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, str(t, obj(t, it)["id"]))
	}
	return out
}

func TestPlanesAmigos_VisibilidadVentanaSilencioYPrivacidad(t *testing.T) {
	env, app := amistadesEnv(t)
	lugar := ini84Lugar(t, env, "ini84-cafe")
	yo, mariana, extrana := seedAmigo(t, env, "i84-yo"), seedAmigo(t, env, "i84-mariana"), seedAmigo(t, env, "i84-extrana")
	silenciada, reservada, oculta := seedAmigo(t, env, "i84-silenciada"), seedAmigo(t, env, "i84-reservada"), seedAmigo(t, env, "i84-oculta")
	for _, f := range []amigo{mariana, silenciada, reservada, oculta} {
		befriend(t, env, yo, f)
	}
	_, err := env.Mongo.DB().Collection(amistades.Collection).UpdateOne(context.Background(),
		bson.D{{Key: "par", Value: amistades.PairKey(yo.user.ID, silenciada.user.ID)}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "silenciado_por", Value: bson.A{yo.user.ID}}}}})
	require.NoError(t, err)
	ini84Social(t, env, reservada, "compartir_planes", false)
	ini84Social(t, env, oculta, "actividad", "nadie")

	now := time.Now().UTC()
	viejo := now.AddDate(0, -1, 0)
	sabado := ini84Plan(t, env, mariana, "Sábado de cafés", planes.VisibilityFriends, now.AddDate(0, 0, 3), viejo, lugar)
	publico := ini84Plan(t, env, mariana, "Ruta pública", planes.VisibilityPublic, now.AddDate(0, 0, 5), viejo, lugar)
	reciente := ini84Plan(t, env, mariana, "Ayer", planes.VisibilityFriends, now.AddDate(0, 0, -3), now.Add(-48*time.Hour), lugar)
	ini84Plan(t, env, mariana, "Privado", planes.VisibilityPrivate, now.AddDate(0, 0, 2), viejo, lugar)
	ini84Plan(t, env, mariana, "Muy lejos", planes.VisibilityFriends, now.AddDate(0, 0, 45), viejo, lugar)
	ini84Plan(t, env, mariana, "Pasado viejo", planes.VisibilityFriends, now.AddDate(0, 0, -20), viejo, lugar)
	ini84Plan(t, env, extrana, "De otra", planes.VisibilityPublic, now.AddDate(0, 0, 2), viejo, lugar)
	ini84Plan(t, env, silenciada, "Silenciado", planes.VisibilityFriends, now.AddDate(0, 0, 2), viejo, lugar)
	ini84Plan(t, env, reservada, "No comparte", planes.VisibilityFriends, now.AddDate(0, 0, 2), viejo, lugar)
	ini84Plan(t, env, oculta, "Sin actividad", planes.VisibilityPublic, now.AddDate(0, 0, 2), viejo, lugar)

	body := pcall(t, app, http.MethodGet, planesAmigosPath, nil, yo.token, http.StatusOK)
	assert.Equal(t, []string{reciente.ID.Hex(), sabado.ID.Hex(), publico.ID.Hex()}, ini84IDs(t, body))

	// Paginación con cursor: 2 + 1.
	first := pcall(t, app, http.MethodGet, planesAmigosPath+"?limite=2", nil, yo.token, http.StatusOK)
	assert.Equal(t, []string{reciente.ID.Hex(), sabado.ID.Hex()}, ini84IDs(t, first))
	next := str(t, first["siguiente_cursor"])
	second := pcall(t, app, http.MethodGet, planesAmigosPath+"?limite=2&cursor="+next, nil, yo.token, http.StatusOK)
	assert.Equal(t, []string{publico.ID.Hex()}, ini84IDs(t, second))
	pcall(t, app, http.MethodGet, planesAmigosPath+"?cursor="+next, nil, mariana.token, http.StatusUnprocessableEntity)

	// Un amigo ve el plan de `amigos`; quien no es amigo, no (404).
	pcall(t, app, http.MethodGet, "/api/v1/planes/"+sabado.ID.Hex(), nil, yo.token, http.StatusOK)
	pcall(t, app, http.MethodGet, "/api/v1/planes/"+sabado.ID.Hex(), nil, extrana.token, http.StatusNotFound)
}

func TestPlanesAmigos_ApuntadosMeGustaYHacerMiVersion(t *testing.T) {
	env, app := amistadesEnv(t)
	lugar := ini84Lugar(t, env, "ini84-fonda")
	yo, mariana, jorge, ajena := seedAmigo(t, env, "i84a-yo"), seedAmigo(t, env, "i84a-mariana"), seedAmigo(t, env, "i84a-jorge"), seedAmigo(t, env, "i84a-ajena")
	befriend(t, env, yo, mariana)
	befriend(t, env, yo, jorge)
	befriend(t, env, mariana, jorge)
	befriend(t, env, mariana, ajena)
	p := ini84Plan(t, env, mariana, "Sábado de cafés", planes.VisibilityFriends, time.Now().UTC().AddDate(0, 0, 3), time.Now().UTC(), lugar)
	ini84Grupo(t, env, p, grupos.StatusConfirmed, jorge, ajena)

	// «Me apunto» y me gusta en un plan de `amigos`.
	pcall(t, app, http.MethodPost, "/api/v1/planes/"+p.ID.Hex()+"/me-apunto", nil, yo.token, http.StatusOK)
	pcall(t, app, http.MethodPut, "/api/v1/planes/"+p.ID.Hex()+"/me-gusta", nil, yo.token, http.StatusNoContent)

	body := pcall(t, app, http.MethodGet, planesAmigosPath, nil, yo.token, http.StatusOK)
	items := list(t, body["items"])
	require.Len(t, items, 1)
	it := obj(t, items[0])
	assert.Equal(t, "Sábado de cafés", it["nombre"])
	assert.InDelta(t, 420, num(t, it["costo_por_persona"]), 0)
	assert.InDelta(t, 1, num(t, it["me_gusta_total"]), 0)
	assert.Equal(t, mariana.id(), str(t, obj(t, it["autor"])["id"]))
	ap := obj(t, it["apuntados"])
	assert.InDelta(t, 3, num(t, ap["total"]), 0)
	amigosVan := list(t, ap["amigos"])
	require.Len(t, amigosVan, 1, "solo Jorge: la ajena no es mi amiga y yo voy en `yo`")
	assert.Equal(t, jorge.id(), str(t, obj(t, amigosVan[0])["id"]))
	assert.Equal(t, map[string]any{"apuntado": true, "me_gusta": true}, it["yo"])
	parada := obj(t, list(t, it["paradas"])[0])
	assert.Equal(t, lugar.Hex(), str(t, obj(t, parada["lugar"])["id"]))
	for _, k := range []string{"integrantes", "grupo", "zona", "nota", "visibilidad"} {
		assert.NotContains(t, it, k)
	}

	// «Hacer mi versión»: la copia es privada y mía.
	copia := pcall(t, app, http.MethodPost, "/api/v1/planes/"+p.ID.Hex()+"/clonar", map[string]any{"fecha": time.Now().UTC().AddDate(0, 0, 7).Format(planes.DayLayout)}, yo.token, http.StatusCreated)
	assert.Equal(t, planes.VisibilityPrivate, copia["visibilidad"])
}

func TestHistorias_AgrupadasUltimas24hYVerificado(t *testing.T) {
	env, app := amistadesEnv(t)
	ctx := context.Background()
	db := env.Mongo.DB()
	lugar := ini84Lugar(t, env, "ini84-meche")
	yo, jorge, ana, extrana := seedAmigo(t, env, "i84h-yo"), seedAmigo(t, env, "i84h-jorge"), seedAmigo(t, env, "i84h-ana"), seedAmigo(t, env, "i84h-extrana")
	befriend(t, env, yo, jorge)
	befriend(t, env, yo, ana)
	now := time.Now().UTC()
	momento := func(who amigo, vis string, hace time.Duration, negocio *bson.ObjectID) bson.ObjectID {
		at := now.Add(-hace)
		p := postales.Postal{ID: bson.NewObjectID(), UsuarioID: who.user.ID, ImagenID: bson.NewObjectID().Hex(), ImagenEstado: "publicada", ImagenURL: "https://img.test/" + who.id(),
			Estilo: postales.EstiloPolaroid, TextoEstado: postales.TextoAprobado, Visibilidad: vis, Estado: postales.EstadoLista, Version: 1, CreadoEn: at, ActualizadoEn: at}
		_, err := db.Collection(postales.Collection).InsertOne(ctx, p)
		require.NoError(t, err)
		m := postales.Momento{ID: bson.NewObjectID(), AutorID: who.user.ID, PostalID: p.ID, NegocioID: negocio, Visibilidad: vis, Estado: postales.MomentoPublicado,
			Etiquetados: []bson.ObjectID{}, Version: 1, CreadoEn: at, ActualizadoEn: at}
		_, err = db.Collection(postales.MomentosCollection).InsertOne(ctx, m)
		require.NoError(t, err)
		return m.ID
	}
	j1 := momento(jorge, postales.VisAmigos, 3*time.Hour, &lugar)
	j2 := momento(jorge, postales.VisPublica, 2*time.Hour, nil)
	momento(jorge, postales.VisAmigos, 30*time.Hour, nil)
	momento(jorge, postales.VisPrivada, time.Hour, nil)
	a1 := momento(ana, postales.VisAmigos, 30*time.Minute, &lugar)
	momento(extrana, postales.VisPublica, time.Hour, nil)
	dia := ini84Dia(now.Add(-3 * time.Hour))
	_, err := db.Collection(resenas.CheckinsCollection).InsertOne(ctx, resenas.Checkin{ID: bson.NewObjectID(), UsuarioID: jorge.user.ID, NegocioID: lugar,
		Metodo: "qr", Valido: true, Dia: dia, CreadoEn: now.Add(-3 * time.Hour)})
	require.NoError(t, err)

	body := pcall(t, app, http.MethodGet, historiasPath, nil, yo.token, http.StatusOK)
	personas := list(t, body["personas"])
	require.Len(t, personas, 2)
	primera, segunda := obj(t, personas[0]), obj(t, personas[1])
	assert.Equal(t, ana.id(), str(t, obj(t, primera["autor"])["id"]))
	deAna := obj(t, list(t, primera["momentos"])[0])
	assert.Equal(t, a1.Hex(), str(t, deAna["id"]))
	assert.Equal(t, false, deAna["verificado"])
	ms := list(t, segunda["momentos"])
	require.Len(t, ms, 2)
	assert.Equal(t, j1.Hex(), str(t, obj(t, ms[0])["id"]))
	assert.Equal(t, true, obj(t, ms[0])["verificado"])
	assert.Equal(t, j2.Hex(), str(t, obj(t, ms[1])["id"]))

	// Quien no comparte postales desaparece de las historias.
	ini84Social(t, env, ana, "compartir_postales", false)
	body = pcall(t, app, http.MethodGet, historiasPath, nil, yo.token, http.StatusOK)
	require.Len(t, list(t, body["personas"]), 1)
}

func ini84Dia(t time.Time) string { return resenas.Dia(t, negocios.MexicoCity()) }

func TestVotacionesPendientes_FaltaTuVoto(t *testing.T) {
	env, app := amistadesEnv(t)
	ctx := context.Background()
	lugar := ini84Lugar(t, env, "ini84-voto")
	otro := ini84Lugar(t, env, "ini84-voto-b")
	org, yo := seedAmigo(t, env, "i84v-org"), seedAmigo(t, env, "i84v-yo")
	befriend(t, env, org, yo)
	now := time.Now().UTC()
	p := ini84Plan(t, env, org, "Viernes", planes.VisibilityPrivate, now.AddDate(0, 0, 2), now, lugar)
	g := ini84Grupo(t, env, p, grupos.StatusInvited, yo)
	repo := votaciones.NewRepository(env.Mongo.DB())
	vote := func(cierra time.Time, estado string, votos map[string]bson.ObjectID) votaciones.Vote {
		v := votaciones.Vote{ID: bson.NewObjectID(), GroupID: g.ID, PlanID: p.ID, StopID: bson.NewObjectID(), Options: []bson.ObjectID{lugar, otro},
			Ballots: votos, ClosesAt: cierra, Status: estado, CreatedBy: org.user.ID, CreatedAt: now, UpdatedAt: now}
		require.NoError(t, repo.Create(ctx, &v))
		return v
	}
	pendiente := vote(now.Add(time.Hour), votaciones.StatusOpen, map[string]bson.ObjectID{})
	vote(now.Add(2*time.Hour), votaciones.StatusOpen, map[string]bson.ObjectID{yo.id(): lugar})
	vote(now.Add(-time.Minute), votaciones.StatusOpen, map[string]bson.ObjectID{})
	vote(now.Add(time.Hour), votaciones.StatusClosed, map[string]bson.ObjectID{})

	body := pcall(t, app, http.MethodGet, votosPendientesPath, nil, yo.token, http.StatusOK)
	items := list(t, body["items"])
	require.Len(t, items, 1)
	it := obj(t, items[0])
	assert.Equal(t, pendiente.ID.Hex(), it["id"])
	assert.Equal(t, map[string]any{"id": g.ID.Hex(), "nombre": "Viernes"}, it["grupo"])

	// Quien declinó ya no tiene votos pendientes; quien no está en el grupo tampoco.
	_, err := env.Mongo.DB().Collection(grupos.Collection).UpdateOne(ctx, bson.D{{Key: "_id", Value: g.ID}, {Key: "integrantes.usuario_id", Value: yo.user.ID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "integrantes.$.estado", Value: grupos.StatusDeclined}}}})
	require.NoError(t, err)
	body = pcall(t, app, http.MethodGet, votosPendientesPath, nil, yo.token, http.StatusOK)
	assert.Empty(t, body["items"])
	extrana := seedAmigo(t, env, "i84v-extrana")
	body = pcall(t, app, http.MethodGet, votosPendientesPath, nil, extrana.token, http.StatusOK)
	assert.Empty(t, body["items"])
}
