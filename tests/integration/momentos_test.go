package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

const momentosPath = "/api/v1/momentos"

func TestMomentos_PublicarFeedYEtiquetas(t *testing.T) {
	s := nuevoPostales(t)
	yo, amiga, otra, extrano := seedAmigo(t, s.env, "momentos"), seedAmigo(t, s.env, "momentos-amiga"), seedAmigo(t, s.env, "momentos-otra"), seedAmigo(t, s.env, "momentos-ext")
	befriend(t, s.env, yo, amiga)
	befriend(t, s.env, yo, otra)

	enModeracion := crearPostal(t, s, yo, cuerpoPostal(fotoDudosa(t, s, yo), postales.VisPublica))
	pcall(t, s.app, http.MethodPost, momentosPath, map[string]any{"postal_id": enModeracion, "visibilidad": "publica"}, yo.token, http.StatusUnprocessableEntity)

	postal := crearPostal(t, s, yo, cuerpoPostal(fotoPublicada(t, s, yo), postales.VisPrivada))
	pcall(t, s.app, http.MethodPost, momentosPath, map[string]any{"postal_id": postal, "visibilidad": "amigos", "etiquetados": []string{extrano.id()}}, yo.token, http.StatusUnprocessableEntity)
	pcall(t, s.app, http.MethodPost, momentosPath, map[string]any{"postal_id": postal, "visibilidad": "amigos"}, amiga.token, http.StatusNotFound)
	m := pcall(t, s.app, http.MethodPost, momentosPath, map[string]any{"postal_id": postal, "visibilidad": "amigos", "texto": "Con las amigas", "etiquetados": []string{amiga.id()}}, yo.token, http.StatusCreated)
	mid := str(t, m["id"])
	assert.Equal(t, postales.MomentoPublicado, m["estado"])
	assert.Equal(t, int64(1), avisos(t, s, amiga, notificaciones.KindMomentTagged))
	pcall(t, s.app, http.MethodPost, momentosPath, map[string]any{"postal_id": postal, "visibilidad": "amigos"}, yo.token, http.StatusConflict)
	assert.Equal(t, postales.VisAmigos, pcall(t, s.app, http.MethodGet, postalesPath+"/"+postal, nil, yo.token, http.StatusOK)["visibilidad"], "la privada toma la del momento")

	feed := list(t, pcall(t, s.app, http.MethodGet, momentosPath+"/feed", nil, amiga.token, http.StatusOK)["items"])
	require.Len(t, feed, 1)
	item := obj(t, feed[0])
	assert.Equal(t, "momentos", obj(t, item["autor"])["nombre_usuario"])
	assert.Contains(t, obj(t, item["postal"])["imagen_url"], "/1080.jpg")
	assert.Nil(t, item["plan_id"])
	assert.Empty(t, list(t, pcall(t, s.app, http.MethodGet, momentosPath+"/feed", nil, extrano.token, http.StatusOK)["items"]))
	pcall(t, s.app, http.MethodGet, momentosPath+"/"+mid, nil, extrano.token, http.StatusNotFound)
	pcall(t, s.app, http.MethodGet, momentosPath+"/"+mid, nil, "", http.StatusNotFound)
	pcall(t, s.app, http.MethodGet, momentosPath+"/"+mid, nil, amiga.token, http.StatusOK)

	// Más abierto que la postal = 422; editar avisa solo a la nueva etiquetada.
	pcall(t, s.app, http.MethodPatch, momentosPath+"/"+mid, map[string]any{"visibilidad": "publica"}, yo.token, http.StatusUnprocessableEntity, ifm(1)...)
	pcall(t, s.app, http.MethodPatch, momentosPath+"/"+mid, map[string]any{"texto": "x"}, yo.token, http.StatusBadRequest)
	ed := pcall(t, s.app, http.MethodPatch, momentosPath+"/"+mid, map[string]any{"etiquetados": []string{amiga.id(), otra.id()}}, yo.token, http.StatusOK, ifm(1)...)
	assert.Equal(t, float64(2), ed["version"])
	assert.Equal(t, int64(1), avisos(t, s, amiga, notificaciones.KindMomentTagged))
	assert.Equal(t, int64(1), avisos(t, s, otra, notificaciones.KindMomentTagged))
	pcall(t, s.app, http.MethodPatch, momentosPath+"/"+mid, map[string]any{"texto": "y"}, amiga.token, http.StatusNotFound, ifm(2)...)

	// Texto dudoso: el momento queda en moderación y solo el autor lo ve hasta que se apruebe.
	s.env.ModeradorPostales.Fijar(moderacion.VeredictoCuarentena)
	ed = pcall(t, s.app, http.MethodPatch, momentosPath+"/"+mid, map[string]any{"texto": "texto dudoso"}, yo.token, http.StatusOK, ifm(2)...)
	assert.Equal(t, postales.MomentoEnModeracion, ed["estado"])
	pcall(t, s.app, http.MethodGet, momentosPath+"/"+mid, nil, amiga.token, http.StatusNotFound)
	decidirCola(t, s.env, mid, "aprobado")
	pcall(t, s.app, http.MethodGet, momentosPath+"/"+mid, nil, amiga.token, http.StatusOK)

	pcall(t, s.app, http.MethodDelete, momentosPath+"/"+mid, nil, yo.token, http.StatusConflict, ifm(1)...)
	pcall(t, s.app, http.MethodDelete, momentosPath+"/"+mid, nil, yo.token, http.StatusNoContent, ifm(3)...)
	pcall(t, s.app, http.MethodGet, postalesPath+"/"+postal, nil, yo.token, http.StatusOK)
}

func TestMomentos_PanelDelNegocio(t *testing.T) {
	s := nuevoPostales(t)
	yo := seedAmigo(t, s.env, "cliente-panel")
	body := cuerpoPostal(fotoPublicada(t, s, yo), postales.VisPublica)
	body["negocio_id"] = s.negocio.ID.Hex()
	m := pcall(t, s.app, http.MethodPost, momentosPath, map[string]any{"postal_id": crearPostal(t, s, yo, body), "visibilidad": "publica"}, yo.token, http.StatusCreated)
	mid := str(t, m["id"])
	soloAmigos := cuerpoPostal(fotoPublicada(t, s, yo), postales.VisAmigos)
	soloAmigos["negocio_id"] = s.negocio.ID.Hex()
	pcall(t, s.app, http.MethodPost, momentosPath, map[string]any{"postal_id": crearPostal(t, s, yo, soloAmigos), "visibilidad": "amigos"}, yo.token, http.StatusCreated)

	panel := "/api/v1/negocios/" + s.negocio.ID.Hex() + "/momentos"
	items := list(t, pcall(t, s.app, http.MethodGet, panel, nil, s.dueno, http.StatusOK)["items"])
	require.Len(t, items, 1, "solo públicos")
	autor := obj(t, obj(t, items[0])["autor"])
	assert.Equal(t, "cliente-panel", autor["nombre_usuario"])
	assert.Nil(t, autor["correo"])

	_, ajenoTok := dueno(t, s.env, "otro-comerciante@wherever.test", reservastest.Config{})
	pcall(t, s.app, http.MethodGet, panel, nil, ajenoTok, http.StatusNotFound)
	pcall(t, s.app, http.MethodGet, panel, nil, yo.token, http.StatusForbidden)
	pcall(t, s.app, http.MethodGet, panel, nil, "", http.StatusUnauthorized)

	pcall(t, s.app, http.MethodPatch, panel+"/"+mid, map[string]any{}, s.dueno, http.StatusUnprocessableEntity)
	pcall(t, s.app, http.MethodPatch, panel+"/"+mid, map[string]any{"destacado": true}, ajenoTok, http.StatusNotFound)
	out := pcall(t, s.app, http.MethodPatch, panel+"/"+mid, map[string]any{"agradecido": true, "destacado": true}, s.dueno, http.StatusOK)
	assert.Equal(t, true, out["agradecido_por_negocio"])
	pcall(t, s.app, http.MethodPatch, panel+"/"+mid, map[string]any{"agradecido": true}, s.dueno, http.StatusOK)
	assert.Equal(t, int64(1), avisos(t, s, yo, notificaciones.KindMomentThanked), "avisa solo al encender")
	assert.Equal(t, int64(1), avisos(t, s, yo, notificaciones.KindMomentFeatured))
	destacados := list(t, pcall(t, s.app, http.MethodGet, panel+"?destacados=true", nil, s.dueno, http.StatusOK)["items"])
	require.Len(t, destacados, 1)
	pcall(t, s.app, http.MethodPatch, panel+"/"+bson.NewObjectID().Hex(), map[string]any{"destacado": true}, s.dueno, http.StatusNotFound)
}

func TestMomentos_Limite30PorHora(t *testing.T) {
	s := nuevoPostales(t)
	yo := seedAmigo(t, s.env, "limite-momentos")
	// El límite cuenta cada POST, también los rechazados.
	for range 30 {
		pcall(t, s.app, http.MethodPost, momentosPath, map[string]any{"postal_id": bson.NewObjectID().Hex(), "visibilidad": "amigos"}, yo.token, http.StatusNotFound)
	}
	pcall(t, s.app, http.MethodPost, momentosPath, map[string]any{"postal_id": bson.NewObjectID().Hex(), "visibilidad": "amigos"}, yo.token, http.StatusTooManyRequests)
	otro := seedAmigo(t, s.env, "limite-otro").token
	pcall(t, s.app, http.MethodPost, momentosPath, map[string]any{"postal_id": bson.NewObjectID().Hex(), "visibilidad": "amigos"}, otro, http.StatusNotFound)
}
