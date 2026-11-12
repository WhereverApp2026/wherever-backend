package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// personaOpts ajusta a una persona sembrada para la búsqueda (nv-35).
type personaOpts struct {
	phone       string // E.164; vacío = sin teléfono
	phoneSearch bool
	hideName    bool
	status      string
}

func seedPersona(t *testing.T, env *authtest.Env, email, username string, o personaOpts) usuarios.User {
	t.Helper()
	u := env.CreateUser(t, email)
	set := bson.D{{Key: "nombre_usuario", Value: username}, {Key: "privacidad.buscable_por_telefono", Value: o.phoneSearch}}
	if o.phone != "" {
		set = append(set, bson.E{Key: "telefono_hash", Value: usuarios.HashPhone(env.Cfg.PhoneHMACKey, o.phone)})
	}
	if o.hideName {
		set = append(set, bson.E{Key: "privacidad.buscable_por_usuario", Value: false})
	}
	if o.status != "" {
		set = append(set, bson.E{Key: "estado", Value: o.status})
	}
	u, err := env.Users.Update(context.Background(), u.ID, set)
	require.NoError(t, err)
	return u
}

// buscarEnv siembra a quien busca (`yo_busca`) y devuelve la app y su token.
func buscarEnv(t *testing.T) (*authtest.Env, *fiber.App, string) {
	t.Helper()
	env := authEnv(t)
	env.ClearRateLimits(t)
	me := seedPersona(t, env, "yo@busca.mx", "yo_busca", personaOpts{})
	return env, env.App(), testutil.ValidTokenFor(t, me.ID.Hex(), "jti-busca", middleware.RoleConsumer)
}

func buscar(t *testing.T, app *fiber.App, token, q string, want int) string {
	t.Helper()
	resp := sendJSON(t, app, http.MethodGet, "/api/v1/personas/buscar?q="+url.QueryEscape(q), nil, token)
	raw := readAll(t, resp)
	require.Equal(t, want, resp.StatusCode, raw)
	return raw
}

func usernamesOf(t *testing.T, raw string) []string {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &body))
	assertSnakeCase(t, body)
	out := []string{}
	for _, item := range list(t, body["resultados"]) {
		out = append(out, str(t, obj(t, item)["nombre_usuario"]))
	}
	return out
}

func TestBuscarPersonas_PorUsuarioYPrefijo(t *testing.T) {
	env, app, token := buscarEnv(t)
	seedPersona(t, env, "ana@x.mx", "ana_lopez", personaOpts{})
	seedPersona(t, env, "anabel@x.mx", "anabel", personaOpts{})
	seedPersona(t, env, "beto@x.mx", "beto", personaOpts{})

	assert.Equal(t, []string{"ana_lopez", "anabel"}, usernamesOf(t, buscar(t, app, token, "ana", http.StatusOK)))
	assert.Equal(t, []string{"ana_lopez"}, usernamesOf(t, buscar(t, app, token, "@Ana_L", http.StatusOK)), "mayúsculas y @ se normalizan")
	assert.Empty(t, usernamesOf(t, buscar(t, app, token, "yo_busca", http.StatusOK)), "quien busca no se ve a sí mismo")
}

func TestBuscarPersonas_UsuarioInexistenteMismaRespuesta(t *testing.T) {
	_, app, token := buscarEnv(t)
	assert.JSONEq(t, `{"resultados":[]}`, buscar(t, app, token, "nadie_aqui", http.StatusOK))
}

func TestBuscarPersonas_OcultosPorPrivacidadYEstado(t *testing.T) {
	env, app, token := buscarEnv(t)
	seedPersona(t, env, "oculta@x.mx", "carla_oculta", personaOpts{hideName: true})
	seedPersona(t, env, "bloq@x.mx", "carla_bloq", personaOpts{status: usuarios.StatusBlocked})
	seedPersona(t, env, "elim@x.mx", "carla_elim", personaOpts{status: usuarios.StatusPendingDeletion})
	seedPersona(t, env, "visible@x.mx", "carla_visible", personaOpts{})

	assert.Equal(t, []string{"carla_visible"}, usernamesOf(t, buscar(t, app, token, "carla", http.StatusOK)))
}

func TestBuscarPersonas_PorTelefono(t *testing.T) {
	env, app, token := buscarEnv(t)
	seedPersona(t, env, "tel@x.mx", "dani_tel", personaOpts{phone: "+525512345678", phoneSearch: true})
	seedPersona(t, env, "notel@x.mx", "eva_notel", personaOpts{phone: "+525587654321"})

	assert.Equal(t, []string{"dani_tel"}, usernamesOf(t, buscar(t, app, token, "+52 55 1234-5678", http.StatusOK)))
	assert.Equal(t, []string{"dani_tel"}, usernamesOf(t, buscar(t, app, token, "55 1234 5678", http.StatusOK)), "10 dígitos = +52")

	noSearch := buscar(t, app, token, "+525587654321", http.StatusOK)
	missing := buscar(t, app, token, "+525500000000", http.StatusOK)
	assert.Equal(t, missing, noSearch, "no buscable y no registrado responden igual")
}

func TestBuscarPersonas_ConsultaInvalida422(t *testing.T) {
	_, app, token := buscarEnv(t)
	for _, q := range []string{"", "ab", "no valido!", "+12"} {
		buscar(t, app, token, q, http.StatusUnprocessableEntity)
	}
}

func TestBuscarPersonas_SinToken401(t *testing.T) {
	_, app, _ := buscarEnv(t)
	buscar(t, app, "", "ana", http.StatusUnauthorized)
}

func TestBuscarPersonas_LimitePorHora(t *testing.T) {
	_, app, token := buscarEnv(t)
	for i := 0; i < 30; i++ {
		buscar(t, app, token, "ana", http.StatusOK)
	}
	resp := sendJSON(t, app, http.MethodGet, "/api/v1/personas/buscar?q=ana", nil, token)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get(fiber.HeaderRetryAfter))
}
