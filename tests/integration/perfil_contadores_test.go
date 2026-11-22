package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
	"github.com/WhereverApp2026/wherever-backend/internal/resenas"
)

func perfilPath(id string) string { return "/api/v1/personas/" + id }

// plan81 siembra un plan del dueño con esa visibilidad, estado y fecha.
func plan81(t *testing.T, db *mongo.Database, dueno bson.ObjectID, vis, estado string, fecha time.Time) bson.ObjectID {
	t.Helper()
	id := bson.NewObjectID()
	_, err := db.Collection(planes.Collection).InsertOne(context.Background(), bson.D{
		{Key: "_id", Value: id}, {Key: "dueno_id", Value: dueno}, {Key: "nombre", Value: "Plan " + vis}, {Key: "fecha", Value: fecha},
		{Key: "estado", Value: estado}, {Key: "visibilidad", Value: vis}, {Key: "paradas", Value: bson.A{}},
	})
	require.NoError(t, err)
	return id
}

// resena81 siembra una reseña del autor en el negocio con ese estado.
func resena81(t *testing.T, db *mongo.Database, autor, negocio bson.ObjectID, estado string, creada time.Time) bson.ObjectID {
	t.Helper()
	id := bson.NewObjectID()
	_, err := db.Collection(resenas.Collection).InsertOne(context.Background(), bson.D{
		{Key: "_id", Value: id}, {Key: "usuario_id", Value: autor}, {Key: "negocio_id", Value: negocio}, {Key: "checkin_id", Value: bson.NewObjectID()},
		{Key: "calificacion", Value: 5}, {Key: "comentario", Value: "Rico"}, {Key: "chips", Value: bson.A{}}, {Key: "fotos", Value: bson.A{}},
		{Key: "estado", Value: estado}, {Key: "version", Value: 1}, {Key: "creado_en", Value: creada}, {Key: "actualizado_en", Value: creada},
	})
	require.NoError(t, err)
	return id
}

// momento81 siembra una postal lista y su momento publicado con esa visibilidad.
func momento81(t *testing.T, db *mongo.Database, autor bson.ObjectID, vis string) bson.ObjectID {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	p := postales.Postal{ID: bson.NewObjectID(), UsuarioID: autor, ImagenID: bson.NewObjectID().Hex(), ImagenEstado: "publicada",
		Estilo: postales.EstiloPolaroid, TextoEstado: postales.TextoAprobado, Visibilidad: vis, Estado: postales.EstadoLista, Version: 1, CreadoEn: now, ActualizadoEn: now}
	_, err := db.Collection(postales.Collection).InsertOne(ctx, p)
	require.NoError(t, err)
	m := postales.Momento{ID: bson.NewObjectID(), AutorID: autor, PostalID: p.ID, Visibilidad: vis, Estado: postales.MomentoPublicado,
		Etiquetados: []bson.ObjectID{}, Version: 1, CreadoEn: now, ActualizadoEn: now}
	_, err = db.Collection(postales.MomentosCollection).InsertOne(ctx, m)
	require.NoError(t, err)
	return m.ID
}

func ids81(t *testing.T, page map[string]any) []string {
	t.Helper()
	out := []string{}
	for _, it := range list(t, page["items"]) {
		out = append(out, str(t, obj(t, it)["id"]))
	}
	return out
}

// Los contadores propios cuentan todos los planes activos y no pasan por la caché; los de otra
// persona, solo los públicos, con caché. Planes borrados y reseñas ocultas no cuentan. Quitar
// una amistad invalida la caché de las dos personas.
func TestPerfilContadores_PropiosAjenosEInvalidacion(t *testing.T) {
	env, app := amistadesEnv(t)
	db := env.Mongo.DB()
	ana, beto, cleo := seedAmigo(t, env, "cont81_ana"), seedAmigo(t, env, "cont81_beto"), seedAmigo(t, env, "cont81_cleo")
	befriend(t, env, ana, beto)
	befriend(t, env, cleo, beto)
	now := time.Now().UTC()
	plan81(t, db, beto.user.ID, planes.VisibilityPublic, planes.StatusActive, now)
	plan81(t, db, beto.user.ID, planes.VisibilityPrivate, planes.StatusActive, now)
	plan81(t, db, beto.user.ID, planes.VisibilityPublic, planes.StatusDeleted, now)
	negocio := bson.NewObjectID()
	resena81(t, db, beto.user.ID, negocio, resenas.EstadoPublicada, now)
	resena81(t, db, beto.user.ID, negocio, resenas.EstadoOculta, now)

	yo := pcall(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, beto.token, http.StatusOK)
	assert.Equal(t, map[string]any{"planes": 2.0, "amigos": 2.0, "resenas": 1.0}, obj(t, yo["contadores"]))
	ajeno := pcall(t, app, http.MethodGet, perfilPath(beto.id()), nil, cleo.token, http.StatusOK)
	assert.Equal(t, map[string]any{"planes": 1.0, "amigos": 2.0, "resenas": 1.0}, obj(t, ajeno["contadores"]))
	assert.Contains(t, ajeno, "insignias_destacadas")

	// Lo sembrado por fuera de los services no se ve en el perfil ajeno mientras viva la caché;
	// el propio cuenta directo en MongoDB.
	resena81(t, db, beto.user.ID, negocio, resenas.EstadoPublicada, now)
	ajeno = pcall(t, app, http.MethodGet, perfilPath(beto.id()), nil, cleo.token, http.StatusOK)
	assert.Equal(t, 1.0, obj(t, ajeno["contadores"])["resenas"], "sigue la caché")
	yo = pcall(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, beto.token, http.StatusOK)
	assert.Equal(t, 2.0, obj(t, yo["contadores"])["resenas"], "el propio no usa la caché")
	// Quitar la amistad invalida a las dos personas.
	amigos := pcall(t, app, http.MethodGet, "/api/v1/amistades", nil, ana.token, http.StatusOK)
	rel := str(t, obj(t, list(t, amigos["resultados"])[0])["id"])
	pcall(t, app, http.MethodDelete, "/api/v1/amistades/"+rel, nil, ana.token, http.StatusNoContent)
	ajeno = pcall(t, app, http.MethodGet, perfilPath(beto.id()), nil, cleo.token, http.StatusOK)
	assert.Equal(t, map[string]any{"planes": 1.0, "amigos": 1.0, "resenas": 2.0}, obj(t, ajeno["contadores"]))
	// Sin amistad y con `actividad: amigos` (por omisión), los contadores van en null.
	ajeno = pcall(t, app, http.MethodGet, perfilPath(beto.id()), nil, ana.token, http.StatusOK)
	assert.Equal(t, map[string]any{"planes": nil, "amigos": nil, "resenas": nil}, obj(t, ajeno["contadores"]))
}

// Las pestañas respetan la visibilidad de cada momento y la privacidad de su dueño: con
// `amigos` solo los amigos ven, con `todos` cualquiera ve lo público, con `nadie` nadie.
func TestPerfilPestanas_VisibilidadYPrivacidad(t *testing.T) {
	s := nuevoPostales(t)
	db := s.env.Mongo.DB()
	ana, beto, cleo := seedAmigo(t, s.env, "pest81_ana"), seedAmigo(t, s.env, "pest81_beto"), seedAmigo(t, s.env, "pest81_cleo")
	befriend(t, s.env, ana, beto)
	now := time.Now().UTC()
	mPublico, mAmigos := momento81(t, db, beto.user.ID, postales.VisPublica), momento81(t, db, beto.user.ID, postales.VisAmigos)
	pPublico := plan81(t, db, beto.user.ID, planes.VisibilityPublic, planes.StatusActive, now)
	plan81(t, db, beto.user.ID, planes.VisibilityPrivate, planes.StatusActive, now)
	rVisible := resena81(t, db, beto.user.ID, s.negocio.ID, resenas.EstadoPublicada, now)
	resena81(t, db, beto.user.ID, s.negocio.ID, resenas.EstadoOculta, now)

	ver := func(who amigo, pestana string) []string {
		t.Helper()
		return ids81(t, pcall(t, s.app, http.MethodGet, perfilPath(beto.id())+"/"+pestana, nil, who.token, http.StatusOK))
	}
	// `amigos` (por omisión): la amiga ve todo lo suyo; la extraña, nada.
	assert.ElementsMatch(t, []string{mPublico.Hex(), mAmigos.Hex()}, ver(ana, "momentos"))
	assert.Equal(t, []string{pPublico.Hex()}, ver(ana, "planes"), "un plan privado nunca sale")
	assert.Equal(t, []string{rVisible.Hex()}, ver(ana, "resenas"), "una reseña oculta no sale")
	for _, p := range []string{"momentos", "planes", "resenas"} {
		assert.Empty(t, ver(cleo, p), p)
	}
	resena := obj(t, list(t, pcall(t, s.app, http.MethodGet, perfilPath(beto.id())+"/resenas", nil, ana.token, http.StatusOK)["items"])[0])
	assert.Equal(t, s.negocio.Name, obj(t, resena["negocio"])["nombre"])
	assert.Equal(t, s.negocio.Slug, obj(t, resena["negocio"])["slug"])

	// `todos`: la extraña ve solo lo público; la amiga sigue viendo lo de amigos.
	social(t, s.app, beto, map[string]any{"actividad": "todos"})
	assert.Equal(t, []string{mPublico.Hex()}, ver(cleo, "momentos"))
	assert.Equal(t, []string{pPublico.Hex()}, ver(cleo, "planes"))
	assert.Equal(t, []string{rVisible.Hex()}, ver(cleo, "resenas"))
	assert.Len(t, ver(ana, "momentos"), 2)

	// Cada interruptor cierra su pestaña.
	social(t, s.app, beto, map[string]any{"compartir_resenas": false, "compartir_planes": false})
	assert.Empty(t, ver(cleo, "resenas"))
	assert.Empty(t, ver(ana, "planes"))
	assert.NotEmpty(t, ver(cleo, "momentos"))
	ajeno := pcall(t, s.app, http.MethodGet, perfilPath(beto.id()), nil, cleo.token, http.StatusOK)
	c := obj(t, ajeno["contadores"])
	assert.Nil(t, c["planes"])
	assert.Nil(t, c["resenas"])
	assert.NotNil(t, c["amigos"])

	// `nadie`: pestañas vacías y contadores en null, aun para la amiga.
	social(t, s.app, beto, map[string]any{"actividad": "nadie", "compartir_resenas": true, "compartir_planes": true})
	for _, p := range []string{"momentos", "planes", "resenas"} {
		assert.Empty(t, ver(ana, p), p)
	}
	ajeno = pcall(t, s.app, http.MethodGet, perfilPath(beto.id()), nil, ana.token, http.StatusOK)
	assert.Equal(t, map[string]any{"planes": nil, "amigos": nil, "resenas": nil}, obj(t, ajeno["contadores"]))
	assert.Empty(t, list(t, ajeno["insignias_destacadas"]))
}

// Las pestañas paginan con cursor firmado del más nuevo al más viejo; un cursor de otra
// persona es 422.
func TestPerfilPestanas_Paginacion(t *testing.T) {
	env, app := amistadesEnv(t)
	db := env.Mongo.DB()
	ana, beto, cleo := seedAmigo(t, env, "pag81_ana"), seedAmigo(t, env, "pag81_beto"), seedAmigo(t, env, "pag81_cleo")
	befriend(t, env, ana, beto)
	befriend(t, env, ana, cleo)
	base := time.Now().UTC()
	var esperados []string
	for i := range 3 {
		esperados = append([]string{plan81(t, db, beto.user.ID, planes.VisibilityPublic, planes.StatusActive, base.Add(time.Duration(i)*time.Hour)).Hex()}, esperados...)
	}
	plan81(t, db, cleo.user.ID, planes.VisibilityPublic, planes.StatusActive, base)
	plan81(t, db, cleo.user.ID, planes.VisibilityPublic, planes.StatusActive, base)
	path := perfilPath(beto.id()) + "/planes?limite=2"
	uno := pcall(t, app, http.MethodGet, path, nil, ana.token, http.StatusOK)
	require.NotEmpty(t, uno["siguiente_cursor"])
	cursor := str(t, uno["siguiente_cursor"])
	dos := pcall(t, app, http.MethodGet, path+"&cursor="+cursor, nil, ana.token, http.StatusOK)
	assert.Equal(t, esperados, append(ids81(t, uno), ids81(t, dos)...))
	assert.Empty(t, dos["siguiente_cursor"])
	pcall(t, app, http.MethodGet, perfilPath(cleo.id())+"/planes?limite=1&cursor="+cursor, nil, ana.token, http.StatusUnprocessableEntity)
}

// La lista de amigos trae los planes públicos de cada amigo y si planea hoy; quien no comparte
// sus planes los lleva en null.
func TestPerfilContadores_ListaDeAmigos(t *testing.T) {
	env, app := amistadesEnv(t)
	db := env.Mongo.DB()
	ana, beto, cleo, dani := seedAmigo(t, env, "lista81_ana"), seedAmigo(t, env, "lista81_beto"), seedAmigo(t, env, "lista81_cleo"), seedAmigo(t, env, "lista81_dani")
	for _, a := range []amigo{beto, cleo, dani} {
		befriend(t, env, ana, a)
	}
	local := time.Now().In(negocios.MexicoCity())
	hoy := time.Date(local.Year(), local.Month(), local.Day(), 12, 0, 0, 0, local.Location())
	plan81(t, db, beto.user.ID, planes.VisibilityPrivate, planes.StatusActive, hoy) // planea hoy, aunque sea privado
	plan81(t, db, beto.user.ID, planes.VisibilityPublic, planes.StatusActive, hoy.AddDate(0, 0, 7))
	plan81(t, db, dani.user.ID, planes.VisibilityPublic, planes.StatusActive, hoy.AddDate(0, 0, 3))
	social(t, app, cleo, map[string]any{"compartir_planes": false})
	plan81(t, db, cleo.user.ID, planes.VisibilityPublic, planes.StatusActive, hoy)

	out := pcall(t, app, http.MethodGet, "/api/v1/amistades", nil, ana.token, http.StatusOK)
	porNombre := map[string]map[string]any{}
	for _, it := range list(t, out["resultados"]) {
		item := obj(t, it)
		porNombre[str(t, obj(t, item["persona"])["nombre_usuario"])] = item
	}
	require.Len(t, porNombre, 3)
	assert.Equal(t, 1.0, porNombre["lista81_beto"]["planes"])
	assert.Equal(t, true, porNombre["lista81_beto"]["planeando_ahora"])
	assert.Equal(t, 1.0, porNombre["lista81_dani"]["planes"])
	assert.Equal(t, false, porNombre["lista81_dani"]["planeando_ahora"])
	assert.Nil(t, porNombre["lista81_cleo"]["planes"])
	assert.Nil(t, porNombre["lista81_cleo"]["planeando_ahora"])
}
