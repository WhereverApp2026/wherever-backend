package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/consejos"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

func rutaConsejo(b negocios.Business) string { return "/api/v1/negocios/" + b.ID.Hex() + "/consejo" }

// diaLocal es el día de México con `offset` días.
func diaLocal(offset int) string {
	return time.Now().In(negocios.MexicoCity()).AddDate(0, 0, offset).Format(consejos.FormatoDia)
}

// sembrarDias escribe `n` días en `estadisticas_diarias` (nv-74) hasta ayer.
func sembrarDias(t *testing.T, env *authtest.Env, b negocios.Business, n int, f func(fecha time.Time) bson.D) {
	t.Helper()
	docs := make([]any, 0, n)
	for i := 1; i <= n; i++ {
		fecha := time.Now().In(negocios.MexicoCity()).AddDate(0, 0, -i)
		doc := bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "negocio_id", Value: b.ID}, {Key: "fecha", Value: fecha.Format(consejos.FormatoDia)}}
		docs = append(docs, append(doc, f(fecha)...))
	}
	_, err := env.Mongo.DB().Collection("estadisticas_diarias").InsertMany(context.Background(), docs)
	require.NoError(t, err)
}

// resenaSinRespuesta siembra una reseña publicada con texto y autor; el consejo solo la cuenta.
func resenaSinRespuesta(t *testing.T, env *authtest.Env, b negocios.Business, hace time.Duration, comentario string, respondida bool) {
	t.Helper()
	autor := bson.NewObjectID()
	doc := bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "negocio_id", Value: b.ID}, {Key: "usuario_id", Value: autor}, {Key: "checkin_id", Value: bson.NewObjectID()},
		{Key: "calificacion", Value: 2}, {Key: "comentario", Value: comentario}, {Key: "estado", Value: "publicada"}, {Key: "creado_en", Value: time.Now().UTC().Add(-hace)}}
	if respondida {
		doc = append(doc, bson.E{Key: "respuesta_negocio", Value: bson.D{{Key: "texto", Value: "Gracias"}, {Key: "estado", Value: "publicada"}}})
	}
	_, err := env.Mongo.DB().Collection("resenas").InsertOne(context.Background(), doc)
	require.NoError(t, err)
}

func consejoDB(t *testing.T, env *authtest.Env, b negocios.Business) consejos.Consejo {
	t.Helper()
	var c consejos.Consejo
	require.NoError(t, env.Mongo.DB().Collection(consejos.Collection).FindOne(context.Background(), bson.D{{Key: "negocio_id", Value: b.ID}}).Decode(&c))
	return c
}

// porHora: 2 llegadas por hora de 10 a 20, sin nadie los días `flojo` de 16 a 18.
func porHora(flojo time.Weekday) func(time.Time) bson.D {
	return func(fecha time.Time) bson.D {
		h := make([]int, 24)
		for x := 10; x < 20; x++ {
			h[x] = 2
		}
		if fecha.Weekday() == flojo {
			h[16], h[17] = 0, 0
		}
		return bson.D{{Key: "llegadas", Value: 20}, {Key: "por_hora", Value: h}}
	}
}

// Cada regla con sus datos sembrados, sin IA: responde la plantilla y guarda el consejo listo.
func TestConsejos_ReglasSinIA(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	env.Consejos.WithIA(nil, 0)
	app := env.App()
	ctx := context.Background()
	db := env.Mongo.DB()

	casos := []struct {
		nombre, regla, accion string
		cfg                   reservastest.Config
		sembrar               func(t *testing.T, b negocios.Business)
	}{
		{"franja floja", consejos.ReglaFranjaFloja, consejos.AccionCrearCupon, reservastest.Config{}, func(t *testing.T, b negocios.Business) {
			sembrarDias(t, env, b, 28, porHora(time.Tuesday))
		}},
		{"aspecto en baja", consejos.ReglaAspectoEnBaja, consejos.AccionVerResenas, reservastest.Config{}, func(t *testing.T, b negocios.Business) {
			corte := time.Now().In(negocios.MexicoCity()).AddDate(0, 0, -consejos.DiasVentana)
			sembrarDias(t, env, b, 56, func(fecha time.Time) bson.D {
				atencion := 4.0
				if fecha.Before(corte) {
					atencion = 5
				}
				return bson.D{{Key: "vistas_ficha", Value: 1}, {Key: "resenas_dia", Value: 1},
					{Key: "aspectos_suma", Value: bson.D{{Key: "sabor", Value: 4.0}, {Key: "atencion", Value: atencion}, {Key: "ambiente", Value: 4.0}, {Key: "espera", Value: 4.0}, {Key: "precio_calidad", Value: 4.0}}}}
			})
		}},
		{"reseñas sin respuesta", consejos.ReglaResenasSinResp, consejos.AccionResponderResenas, reservastest.Config{}, func(t *testing.T, b negocios.Business) {
			resenaSinRespuesta(t, env, b, 3*24*time.Hour, "Tardaron mucho", false)
			resenaSinRespuesta(t, env, b, 2*24*time.Hour, "Muy rico", true)
			resenaSinRespuesta(t, env, b, 20*24*time.Hour, "Hace mucho", false)
		}},
		{"cupón sin canjes", consejos.ReglaCuponSinCanjes, consejos.AccionEditarCupon, reservastest.Config{}, func(t *testing.T, b negocios.Business) {
			viejo := time.Now().UTC().AddDate(0, 0, -20)
			sinCanje, conCanje := bson.NewObjectID(), bson.NewObjectID()
			_, err := db.Collection("cupones").InsertMany(ctx, []any{
				bson.D{{Key: "_id", Value: sinCanje}, {Key: "negocio_id", Value: b.ID}, {Key: "estado", Value: "activo"}, {Key: "creado_en", Value: viejo}},
				bson.D{{Key: "_id", Value: conCanje}, {Key: "negocio_id", Value: b.ID}, {Key: "estado", Value: "activo"}, {Key: "creado_en", Value: viejo}},
				bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "negocio_id", Value: b.ID}, {Key: "estado", Value: "activo"}, {Key: "creado_en", Value: time.Now().UTC()}},
			})
			require.NoError(t, err)
			_, err = db.Collection("cupones_usuario").InsertOne(ctx, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "negocio_id", Value: b.ID},
				{Key: "cupon_id", Value: conCanje}, {Key: "usuario_id", Value: bson.NewObjectID()}, {Key: "canjeado_en", Value: time.Now().UTC().Add(-time.Hour)}})
			require.NoError(t, err)
		}},
		{"reservas apagadas", consejos.ReglaReservasApagadas, consejos.AccionConfigurarReservas, reservastest.Config{SinReservas: true}, func(t *testing.T, b negocios.Business) {
			sembrarDias(t, env, b, 28, func(time.Time) bson.D { return bson.D{{Key: "vistas_ficha", Value: 3}} })
		}},
		{"sin datos", consejos.ReglaPerfil, consejos.AccionCompletarPerfil, reservastest.Config{}, func(*testing.T, negocios.Business) {}},
	}
	for i, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			email := "duena-consejo-" + strconv.Itoa(i) + "@wherever.test"
			b, token := dueno(t, env, email, c.cfg)
			c.sembrar(t, b)

			resp := sendJSON(t, app, http.MethodGet, rutaConsejo(b), nil, token)
			requireStatus(t, resp, http.StatusOK)
			assert.Equal(t, "no-store", resp.Header.Get(fiber.HeaderCacheControl))
			out := decode(t, resp)
			assertSnakeCase(t, out)
			assert.Equal(t, c.accion, out["accion"])
			assert.Equal(t, consejos.FuenteReglas, out["fuente"])
			assert.Equal(t, diaLocal(0), out["dia"])
			assert.NotEmpty(t, out["texto"])
			doc := consejoDB(t, env, b)
			assert.Equal(t, c.regla, doc.Regla)
			assert.Equal(t, consejos.EstadoListo, doc.Estado)
			assert.WithinDuration(t, time.Now().Add(consejos.VidaConsejo), doc.ExpiraEn, time.Minute)
		})
	}
	assert.Contains(t, consejoDB(t, env, negocioDe(t, env, "duena-consejo-2@wherever.test")).Texto, "Tienes 1 reseña sin responder", "solo cuenta la publicada, reciente y sin respuesta")
	assert.Contains(t, consejoDB(t, env, negocioDe(t, env, "duena-consejo-3@wherever.test")).Texto, "Tienes un cupón activo", "el canjeado y el nuevo no cuentan")
}

// negocioDe trae el negocio del dueño con ese correo.
func negocioDe(t *testing.T, env *authtest.Env, email string) negocios.Business {
	t.Helper()
	u, err := env.Users.FindByEmail(context.Background(), email)
	require.NoError(t, err)
	var b negocios.Business
	require.NoError(t, env.Mongo.DB().Collection(negocios.Collection).FindOne(context.Background(), bson.D{{Key: "propietario_id", Value: u.ID}}).Decode(&b))
	return b
}

const textoIA = "Tienes una reseña nueva sin responder. Contestarla hoy genera confianza."

// Con IA: la redacta una vez al día; al modelo solo le llega el hecho agregado.
func TestConsejos_IAUnaVezPorDiaSinDatosDeClientes(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-consejo-ia@wherever.test", reservastest.Config{})
	resenaSinRespuesta(t, env, b, time.Hour, "Me atendió Ana López, escríbeme a ana.lopez@correo.mx", false)
	env.IA.Responder(`{"texto":"` + textoIA + `","accion":"responder_resenas"}`)

	for range 2 {
		resp := sendJSON(t, app, http.MethodGet, rutaConsejo(b), nil, token)
		requireStatus(t, resp, http.StatusOK)
		out := decode(t, resp)
		assert.Equal(t, textoIA, out["texto"])
		assert.Equal(t, consejos.AccionResponderResenas, out["accion"])
		assert.Equal(t, consejos.FuenteIA, out["fuente"])
	}
	peticiones := env.IA.Peticiones()
	require.Len(t, peticiones, 1, "la segunda petición del día no llama al modelo")
	p := peticiones[0]
	assert.Empty(t, p.TextoUsuario)
	assert.Contains(t, p.Contexto, "responder_resenas")
	hex := regexp.MustCompile(`[0-9a-fA-F]{24}`)
	for _, prohibido := range []string{b.Name, b.ID.Hex(), "duena-consejo-ia", "Ana", "ana.lopez", "correo.mx", "atendió"} {
		assert.NotContains(t, p.Contexto+p.Instrucciones, prohibido)
	}
	assert.False(t, hex.MatchString(p.Contexto), "sin ids")
}

// Dos peticiones a la vez: una genera (una llamada al modelo) y la otra responde la plantilla.
func TestConsejos_DosALaVezUnaLlamada(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-consejo-par@wherever.test", reservastest.Config{})
	resenaSinRespuesta(t, env, b, time.Hour, "Lento", false)
	env.IA.Tardar(700 * time.Millisecond).Responder(`{"texto":"` + textoIA + `","accion":"responder_resenas"}`)

	const n = 4
	var wg sync.WaitGroup
	codigos := make([]int, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, rutaConsejo(b), nil)
			req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
			resp, err := app.Test(req, 20000)
			if err == nil {
				codigos[i] = resp.StatusCode
				_ = resp.Body.Close()
			}
		}()
	}
	wg.Wait()
	for _, c := range codigos {
		assert.Equal(t, http.StatusOK, c)
	}
	assert.Len(t, env.IA.Peticiones(), 1)
	out := decode(t, sendJSON(t, app, http.MethodGet, rutaConsejo(b), nil, token))
	assert.Equal(t, consejos.FuenteIA, out["fuente"])
	assert.Equal(t, textoIA, out["texto"])
}

// Con el cupo diario de la app agotado (IA_CONSEJO_DIA) responde la plantilla sin llamar.
func TestConsejos_CupoDiarioAgotado(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-consejo-cupo@wherever.test", reservastest.Config{})
	docs := make([]any, 0, consejos.CupoPorOmision)
	for range consejos.CupoPorOmision {
		docs = append(docs, consejos.Consejo{ID: bson.NewObjectID(), NegocioID: bson.NewObjectID(), Dia: diaLocal(0), Texto: "x", Accion: consejos.AccionNinguna,
			Fuente: consejos.FuenteIA, Regla: consejos.ReglaNinguna, Estado: consejos.EstadoListo, CreadoEn: time.Now().UTC(), ExpiraEn: time.Now().UTC().Add(time.Hour)})
	}
	_, err := env.Mongo.DB().Collection(consejos.Collection).InsertMany(context.Background(), docs)
	require.NoError(t, err)

	out := decode(t, sendJSON(t, app, http.MethodGet, rutaConsejo(b), nil, token))
	assert.Equal(t, consejos.FuenteReglas, out["fuente"])
	assert.Empty(t, env.IA.Peticiones())
}

// Un modelo que cambia la acción o devuelve algo que no es JSON = plantilla.
func TestConsejos_SalidaDelModeloRechazada(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()

	b, token := dueno(t, env, "duena-consejo-accion@wherever.test", reservastest.Config{})
	env.IA.Responder(`{"texto":"Lanza un cupón de 2x1 esta semana para atraer gente.","accion":"crear_cupon"}`)
	out := decode(t, sendJSON(t, app, http.MethodGet, rutaConsejo(b), nil, token))
	assert.Equal(t, consejos.FuenteReglas, out["fuente"])
	assert.Equal(t, consejos.AccionCompletarPerfil, out["accion"], "la acción es la del hecho")

	env.IA.Reset()
	env.IA.Responder(`no es json`).Responder(`tampoco`)
	b2, token2 := dueno(t, env, "duena-consejo-json@wherever.test", reservastest.Config{})
	resenaSinRespuesta(t, env, b2, time.Hour, "Frío", false)
	out = decode(t, sendJSON(t, app, http.MethodGet, rutaConsejo(b2), nil, token2))
	assert.Equal(t, consejos.FuenteReglas, out["fuente"])
	assert.Equal(t, consejos.AccionResponderResenas, out["accion"])
	assert.Len(t, env.IA.Peticiones(), 2, "GenerarValidado reintenta una vez")
}

// Un `generando` de hace más de un minuto se retoma; uno reciente es de otra petición.
func TestConsejos_GenerandoVencidoSeRetoma(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	env.Consejos.WithIA(nil, 0)
	app := env.App()
	col := env.Mongo.DB().Collection(consejos.Collection)
	sembrar := func(b negocios.Business, hace time.Duration) {
		_, err := col.InsertOne(context.Background(), consejos.Consejo{ID: bson.NewObjectID(), NegocioID: b.ID, Dia: diaLocal(0), Estado: consejos.EstadoGenerando,
			CreadoEn: time.Now().UTC().Add(-hace), ExpiraEn: time.Now().UTC().Add(time.Hour)})
		require.NoError(t, err)
	}

	viejo, tokenViejo := dueno(t, env, "duena-consejo-viejo@wherever.test", reservastest.Config{})
	sembrar(viejo, 2*time.Minute)
	requireStatus(t, sendJSON(t, app, http.MethodGet, rutaConsejo(viejo), nil, tokenViejo), http.StatusOK)
	assert.Equal(t, consejos.EstadoListo, consejoDB(t, env, viejo).Estado)

	reciente, tokenReciente := dueno(t, env, "duena-consejo-reciente@wherever.test", reservastest.Config{})
	sembrar(reciente, 10*time.Second)
	out := decode(t, sendJSON(t, app, http.MethodGet, rutaConsejo(reciente), nil, tokenReciente))
	assert.Equal(t, consejos.FuenteReglas, out["fuente"])
	assert.Equal(t, consejos.EstadoGenerando, consejoDB(t, env, reciente).Estado, "la plantilla no se guarda")
}

// Sin guardia (GEMINI_API_KEY vacía) el service responde con reglas.
func TestConsejos_SinGuardia(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	svc := consejos.NewService(consejos.NewRepository(env.Mongo.DB()), negocios.NewRepository(env.Mongo.DB()), env.Log)
	b, _ := dueno(t, env, "duena-consejo-sin@wherever.test", reservastest.Config{})

	out, err := svc.Consejo(context.Background(), claimsDe(b.OwnerID), b.ID.Hex())
	require.NoError(t, err)
	assert.Equal(t, consejos.FuenteReglas, out.Fuente)
	assert.Equal(t, consejos.AccionCompletarPerfil, out.Accion)
	assert.Empty(t, env.IA.Peticiones())
}

// La baja del negocio borra sus consejos (negocios.Observador).
func TestConsejos_BajaBorraLosConsejos(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	env.Consejos.WithIA(nil, 0)
	app := env.App()
	b, token := dueno(t, env, "duena-consejo-baja@wherever.test", reservastest.Config{})
	requireStatus(t, sendJSON(t, app, http.MethodGet, rutaConsejo(b), nil, token), http.StatusOK)

	repo := consejos.NewRepository(env.Mongo.DB())
	require.NoError(t, negocios.Observadores{consejos.NewObservador(repo)}.Avisar(context.Background(), negocios.AvisoDadoDeBaja, b.ID))
	n, err := env.Mongo.DB().Collection(consejos.Collection).CountDocuments(context.Background(), bson.D{{Key: "negocio_id", Value: b.ID}})
	require.NoError(t, err)
	assert.Zero(t, n)
}

// claimsDe arma la sesión del comerciante para llamar al service directo.
func claimsDe(id bson.ObjectID) *middleware.Claims {
	return &middleware.Claims{Roles: []string{middleware.RoleMerchant}, RegisteredClaims: jwt.RegisteredClaims{Subject: id.Hex()}}
}

// TestPresupuestoRedis_ConsejoNV75 fija el costo en Upstash: el consejo ya generado es una
// lectura con sesión; generarlo suma la guardia de la IA (caché 1, límite del usuario 3, gasto 1,
// límites globales 6 y guardar en caché 1 = 12). La descripción es una escritura más la guardia.
// Si sube, rehaz la estimación de specs/nv-75.
func TestPresupuestoRedis_ConsejoNV75(t *testing.T) {
	const guardia = 12
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	access := registerHTTP(t, app, "presupuesto75@example.com")
	calentar, tokenCalentar := dueno(t, env, "presupuesto75-a@wherever.test", reservastest.Config{})
	b, token := dueno(t, env, "presupuesto75-b@wherever.test", reservastest.Config{})
	resenaSinRespuesta(t, env, b, time.Hour, "Lento", false)
	env.IA.Responder(`{"texto":"Completa tu perfil con fotos y una buena descripción para atraer visitas.","accion":"completar_perfil"}`).
		Responder(`{"texto":"` + textoIA + `","accion":"responder_resenas"}`).
		Responder(`{"descripcion":"` + descripcionIA + `"}`)
	// La primera llamada del mes crea el contador de gasto (un SETNX más): se hace antes de medir.
	requireStatus(t, sendJSON(t, app, http.MethodGet, rutaConsejo(calentar), nil, tokenCalentar), http.StatusOK)

	read := commandsOf(t, func() {
		requireStatus(t, sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access), http.StatusOK)
	})
	write := commandsOf(t, func() {
		requireStatus(t, sendJSON(t, app, http.MethodPatch, "/api/v1/usuarios/yo/privacidad", map[string]any{"datos_sensibles": false}, access), http.StatusOK)
	})
	generar := commandsOf(t, func() {
		out := decode(t, sendJSON(t, app, http.MethodGet, rutaConsejo(b), nil, token))
		assert.Equal(t, consejos.FuenteIA, out["fuente"])
	})
	leer := commandsOf(t, func() {
		requireStatus(t, sendJSON(t, app, http.MethodGet, rutaConsejo(b), nil, token), http.StatusOK)
	})
	describir := commandsOf(t, func() {
		out := decode(t, sendJSON(t, app, http.MethodPost, rutaDescripcion(b), palabras("tacos", "salsas", "cerveza"), token))
		assert.Equal(t, "ia", out["fuente"])
	})
	t.Logf("consejo generado %d, consejo leído %d, descripción %d", generar, leer, describir)
	assert.Equal(t, read, leer, "consejo ya generado: lectura con sesión, sin Redis propio")
	assert.Equal(t, read+guardia, generar, "una vez por negocio y día")
	assert.Equal(t, write+guardia, describir)
}
