package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

const privacidadPath = "/api/v1/usuarios/yo/privacidad"

// social cambia `privacidad.social` por la ruta real (nv-82).
func social(t *testing.T, app *fiber.App, who amigo, body map[string]any) map[string]any {
	t.Helper()
	out := pcall(t, app, http.MethodPatch, privacidadPath, map[string]any{"social": body}, who.token, http.StatusOK)
	return obj(t, out["social"])
}

// Una cuenta previa lee los valores por omisión; el PATCH es parcial y GET /usuarios/yo lo
// devuelve; un valor fuera del enum es 422.
func TestPrivacidadSocial_ValoresPorOmisionYPatchParcial(t *testing.T) {
	env, app := amistadesEnv(t)
	ana := seedAmigo(t, env, "social_ana")

	yo := pcall(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, ana.token, http.StatusOK)
	assert.Equal(t, map[string]any{"actividad": "amigos", "compartir_resenas": true, "compartir_planes": true,
		"compartir_postales": true, "en_sugerencias": true, "solicitudes": "cualquiera"}, obj(t, obj(t, yo["privacidad"])["social"]))

	got := social(t, app, ana, map[string]any{"actividad": "nadie", "en_sugerencias": false})
	assert.Equal(t, "nadie", got["actividad"])
	assert.Equal(t, false, got["en_sugerencias"])
	assert.Equal(t, true, got["compartir_planes"], "lo que no llega no cambia")
	got = social(t, app, ana, map[string]any{"solicitudes": "codigo"})
	assert.Equal(t, "nadie", got["actividad"], "un PATCH parcial no borra lo anterior")
	assert.Equal(t, "codigo", got["solicitudes"])

	yo = pcall(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, ana.token, http.StatusOK)
	assert.Equal(t, "codigo", obj(t, obj(t, yo["privacidad"])["social"])["solicitudes"])
	for _, bad := range []map[string]any{{"actividad": "publico"}, {"solicitudes": "nadie"}, {"actividad": 3}} {
		resp := sendJSON(t, app, http.MethodPatch, privacidadPath, map[string]any{"social": bad}, ana.token)
		assert.Contains(t, []int{http.StatusBadRequest, http.StatusUnprocessableEntity}, resp.StatusCode, bad)
		_ = resp.Body.Close()
	}
}

// Con `actividad: nadie` el perfil solo trae datos básicos aun entre amigos y las insignias
// salen vacías; con los valores por omisión, la amiga ve lo de hoy.
func TestPrivacidadSocial_PerfilEInsignias(t *testing.T) {
	env, app := amistadesEnv(t)
	ana, beto := seedAmigo(t, env, "social_perfil_ana"), seedAmigo(t, env, "social_perfil_beto")
	befriend(t, env, ana, beto)
	sembrarActividad(t, env.Mongo.DB(), beto.user.ID, actividad{grupos: 1}) // anfitrion_grupo
	insignias := "/api/v1/personas/" + beto.id() + "/insignias"

	full := pcall(t, app, http.MethodGet, "/api/v1/personas/"+beto.id(), nil, ana.token, http.StatusOK)
	assert.Contains(t, full, "nivel")
	assert.Contains(t, full, "grupos_en_comun")
	assert.NotEmpty(t, list(t, pcall(t, app, http.MethodGet, insignias, nil, ana.token, http.StatusOK)["insignias"]))

	social(t, app, beto, map[string]any{"actividad": "nadie"})
	basic := pcall(t, app, http.MethodGet, "/api/v1/personas/"+beto.id(), nil, ana.token, http.StatusOK)
	assert.Equal(t, "amigo", basic["relacion"])
	assert.Equal(t, "social_perfil_beto", basic["nombre_usuario"])
	for _, hidden := range []string{"nivel", "amigos_en_comun", "grupos_en_comun", "silenciado"} {
		assert.NotContains(t, basic, hidden)
	}
	assert.Empty(t, list(t, pcall(t, app, http.MethodGet, insignias, nil, ana.token, http.StatusOK)["insignias"]))
	mine := pcall(t, app, http.MethodGet, "/api/v1/usuarios/yo/insignias", nil, beto.token, http.StatusOK)
	assert.NotEmpty(t, mine["insignias"], "las propias siguen")
}

// Feed, detalle del momento y galería del lugar respetan `actividad` y `compartir_postales`;
// la autora sigue viendo lo suyo.
func TestPrivacidadSocial_MomentosYGaleria(t *testing.T) {
	s := nuevoPostales(t)
	yo, amiga, otra := seedAmigo(t, s.env, "social_mom"), seedAmigo(t, s.env, "social_mom_amiga"), seedAmigo(t, s.env, "social_mom_otra")
	befriend(t, s.env, yo, amiga)
	body := cuerpoPostal(fotoPublicada(t, s, yo), postales.VisPublica)
	body["negocio_id"] = s.negocio.ID.Hex()
	postal := crearPostal(t, s, yo, body)
	mid := str(t, pcall(t, s.app, http.MethodPost, momentosPath, map[string]any{"postal_id": postal, "visibilidad": "publica"}, yo.token, http.StatusCreated)["id"])
	galeria := "/api/v1/lugares/" + s.negocio.ID.Hex() + "/postales"

	visible := func(want bool) {
		t.Helper()
		feed := list(t, pcall(t, s.app, http.MethodGet, momentosPath+"/feed", nil, amiga.token, http.StatusOK)["items"])
		gal := list(t, pcall(t, s.app, http.MethodGet, galeria, nil, otra.token, http.StatusOK)["items"])
		if want {
			assert.Len(t, feed, 1)
			assert.Len(t, gal, 1)
			pcall(t, s.app, http.MethodGet, momentosPath+"/"+mid, nil, "", http.StatusOK)
			pcall(t, s.app, http.MethodGet, postalesPath+"/"+postal, nil, amiga.token, http.StatusOK)
			return
		}
		assert.Empty(t, feed)
		assert.Empty(t, gal)
		for _, tok := range []string{"", amiga.token, otra.token} {
			pcall(t, s.app, http.MethodGet, momentosPath+"/"+mid, nil, tok, http.StatusNotFound)
		}
		pcall(t, s.app, http.MethodGet, postalesPath+"/"+postal, nil, amiga.token, http.StatusNotFound)
	}
	visible(true)

	social(t, s.app, yo, map[string]any{"compartir_postales": false})
	visible(false)
	social(t, s.app, yo, map[string]any{"compartir_postales": true, "actividad": "nadie"})
	visible(false)
	// La autora sigue viendo su momento, su postal y la galería con lo suyo.
	assert.Equal(t, "social_mom", obj(t, pcall(t, s.app, http.MethodGet, momentosPath+"/"+mid, nil, yo.token, http.StatusOK)["autor"])["nombre_usuario"])
	pcall(t, s.app, http.MethodGet, postalesPath+"/"+postal, nil, yo.token, http.StatusOK)
	assert.Len(t, list(t, pcall(t, s.app, http.MethodGet, galeria, nil, yo.token, http.StatusOK)["items"]), 1)

	social(t, s.app, yo, map[string]any{"actividad": "todos"})
	visible(true)
}

// Con `solicitudes: codigo`, desde búsqueda es 403 `solicitud_restringida`; con el código o el
// teléfono de la agenda prospera. Una solicitud recibida se puede aceptar de vuelta.
func TestPrivacidadSocial_SolicitudesPorCodigo(t *testing.T) {
	env, app := amistadesEnv(t)
	cerrada := seedPersona(t, env, "social_cerrada@amigos.mx", "social_cerrada", personaOpts{phone: "+525511112222", phoneSearch: true})
	cerradaTok := amigo{user: cerrada, token: testutil.ValidTokenFor(t, cerrada.ID.Hex(), "jti-social-cerrada", middleware.RoleConsumer)}
	social(t, app, cerradaTok, map[string]any{"solicitudes": "codigo"})
	a, b, c, d := seedAmigo(t, env, "social_sol_a"), seedAmigo(t, env, "social_sol_b"), seedAmigo(t, env, "social_sol_c"), seedAmigo(t, env, "social_sol_d")

	for _, req := range []map[string]any{
		{"usuario_id": cerrada.ID.Hex()},
		{"usuario_id": cerrada.ID.Hex(), "origen": "busqueda"},
		{"usuario_id": cerrada.ID.Hex(), "origen": "codigo"},
		{"usuario_id": cerrada.ID.Hex(), "origen": "codigo", "codigo": "ZZZZ9999"},
		{"usuario_id": cerrada.ID.Hex(), "origen": "contactos", "telefono": "5599990000"},
	} {
		out := pcall(t, app, http.MethodPost, amistadesPath, req, a.token, http.StatusForbidden)
		assert.Equal(t, "solicitud_restringida", out["codigo"], req)
	}
	n, err := env.Mongo.DB().Collection(amistades.Collection).CountDocuments(context.Background(), bson.D{{Key: "par", Value: amistades.PairKey(a.user.ID, cerrada.ID)}})
	require.NoError(t, err)
	assert.Zero(t, n, "no se guarda nada")

	pcall(t, app, http.MethodPost, amistadesPath, map[string]any{"usuario_id": cerrada.ID.Hex(), "origen": "codigo", "codigo": cerrada.FriendCode}, b.token, http.StatusCreated)
	pcall(t, app, http.MethodPost, amistadesPath, map[string]any{"usuario_id": cerrada.ID.Hex(), "origen": "contactos", "telefono": "55 1111 2222"}, c.token, http.StatusCreated)

	// Ella le manda una a d; d la acepta reenviando sin código.
	pcall(t, app, http.MethodPost, amistadesPath, map[string]any{"usuario_id": d.id()}, cerradaTok.token, http.StatusCreated)
	out := pcall(t, app, http.MethodPost, amistadesPath, map[string]any{"usuario_id": cerrada.ID.Hex()}, d.token, http.StatusCreated)
	assert.Equal(t, amistades.StatusAccepted, out["estado"])
}

// `en_sugerencias: false` saca a la persona de GET /amistades/sugerencias.
func TestPrivacidadSocial_FueraDeSugerencias(t *testing.T) {
	env, app := grafoEnv(t)
	ana, b := seedAmigo(t, env, "ana_social_sug"), seedAmigo(t, env, "b_social_sug")
	x, y := seedAmigo(t, env, "x_social_sug"), seedAmigo(t, env, "y_social_sug")
	relacion(t, env, ana.user, b.user, amistades.StatusAccepted)
	relacion(t, env, b.user, x.user, amistades.StatusAccepted)
	relacion(t, env, b.user, y.user, amistades.StatusAccepted)
	proyectar(t, env)
	names, _, _ := sugeridos(t, app, ana)
	assert.ElementsMatch(t, []string{"x_social_sug", "y_social_sug"}, names)

	social(t, app, x, map[string]any{"en_sugerencias": false})
	names, _, _ = sugeridos(t, app, ana)
	assert.Equal(t, []string{"y_social_sug"}, names)
}

// `compartir_planes: false` o `actividad: nadie` quita sus planes públicos de populares.
func TestPrivacidadSocial_PlanesPopulares(t *testing.T) {
	s := newExtras(t)
	pub := s.plan(t, s.owner, nextWeek(), s.a)
	made := planCall(t, s.app, http.MethodPatch, planPath(pub, ""), map[string]string{"visibilidad": "publico"}, s.owner.token, `"1"`)
	require.Equal(t, http.StatusOK, made.status, made.body)
	url := fmt.Sprintf("/api/v1/planes/populares?lat=%f&lng=%f", centerLat, centerLng)
	populares := func() string {
		t.Helper()
		resp := sendJSON(t, s.app, http.MethodGet, url, nil, "")
		raw := readAll(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode, raw)
		return raw
	}
	assert.Contains(t, populares(), pub.ID.Hex())
	social(t, s.app, s.owner, map[string]any{"compartir_planes": false})
	assert.NotContains(t, populares(), pub.ID.Hex())
	social(t, s.app, s.owner, map[string]any{"compartir_planes": true, "actividad": "nadie"})
	assert.NotContains(t, populares(), pub.ID.Hex())
	social(t, s.app, s.owner, map[string]any{"actividad": "amigos"})
	assert.Contains(t, populares(), pub.ID.Hex())
}

// Las cuentas sin `privacidad.social` se leen con los valores por omisión desde Mongo.
func TestPrivacidadSocial_CuentaPreviaSinCampo(t *testing.T) {
	env, _ := amistadesEnv(t)
	u := env.CreateUser(t, "social_previa@amigos.mx")
	_, err := env.Mongo.DB().Collection(usuarios.Collection).UpdateOne(context.Background(), bson.D{{Key: "_id", Value: u.ID}},
		bson.D{{Key: "$unset", Value: bson.D{{Key: "privacidad.social", Value: ""}}}})
	require.NoError(t, err)
	got, err := env.Users.FindByID(context.Background(), u.ID)
	require.NoError(t, err)
	assert.Equal(t, usuarios.ActivityFriends, got.Privacy.Social.ActivityLevel())
	assert.True(t, got.Privacy.Social.SharesPostcards())
	assert.Equal(t, usuarios.RequestsAnyone, got.Privacy.Social.RequestsFrom())
}
