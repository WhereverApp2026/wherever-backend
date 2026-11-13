// Package authtest arma los módulos de identidad reales (auth, usuarios, ciudades y el trabajo
// nocturno de privacidad) contra MongoDB y Redis reales para las suites Integration, Security
// y Contract. Solo se simula el correo (buzón en memoria). Va aparte de testutil para no
// crear un ciclo con los tests internos de auth.
package authtest

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/asistente"
	"github.com/WhereverApp2026/wherever-backend/internal/auth"
	"github.com/WhereverApp2026/wherever-backend/internal/busquedas"
	"github.com/WhereverApp2026/wherever-backend/internal/chat"
	"github.com/WhereverApp2026/wherever-backend/internal/ciudades"
	"github.com/WhereverApp2026/wherever-backend/internal/clima"
	"github.com/WhereverApp2026/wherever-backend/internal/configuracion"
	"github.com/WhereverApp2026/wherever-backend/internal/errores"
	"github.com/WhereverApp2026/wherever-backend/internal/favoritos"
	"github.com/WhereverApp2026/wherever-backend/internal/grafo"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	httpapi "github.com/WhereverApp2026/wherever-backend/internal/http"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/indexes"
	"github.com/WhereverApp2026/wherever-backend/internal/inicio"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/alta"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/expediente"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/personas"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/cache"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/cifrado"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mailer"
	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendador"
	"github.com/WhereverApp2026/wherever-backend/internal/semantica"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/sincronizacion"
	"github.com/WhereverApp2026/wherever-backend/internal/sugerencias"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/climatest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/iatest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/negociostest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios/account"
	"github.com/WhereverApp2026/wherever-backend/internal/votaciones"
)

// Password es la contraseña de los usuarios de prueba; no es secreto de ningún entorno.
const Password = "Prueba_1234"

// Env es el módulo auth listo para probar, con su buzón y sus logs capturados.
type Env struct {
	Cfg             config.Config
	Mongo           *platformmongo.Client
	Redis           *goredis.Client
	Users           *usuarios.Repository
	Service         *auth.Service
	Account         *account.Service
	Cities          *ciudades.Service
	Negocios        *negocios.Service
	Alta            *alta.Service          // alta por pasos y revisión (nv-55)
	Expediente      *expediente.Service    // expediente cifrado (nv-55)
	Almacen         *negociostest.Almacen  // doble del bucket privado (S3)
	Imagenes        *negociostest.Imagenes // doble de las imágenes publicadas hasta que nv-45 llegue a main
	Planes          *planes.Service
	Votaciones      *votaciones.Service
	Chat            *chat.Service
	Moderacion      *moderacion.Service // cola y reglas (nv-43)
	Recomendaciones *recomendaciones.Service
	Clima           *clima.Servicio     // contra OpenMeteo, el doble de Open-Meteo (nv-25)
	OpenMeteo       *climatest.Servidor // la prueba cambia su respuesta, demora o falla
	Exports         *privacidad.Exports
	Grafo           *grafo.Queue // cola de nv-52; las pruebas la aplican con grafo.Worker
	Job             *privacidad.Job
	IA              *iatest.Modelo           // doble del proveedor de IA (nv-31)
	Vectores        *iatest.ServidorVectores // servidor local de embeddings (nv-51)
	Semantica       *semantica.Service
	Busquedas       *busquedas.Service
	Sugerencias     *sugerencias.Service // sin grafo (degradado); las pruebas conectan Neo4j con WithGraph
	Mail            *mailer.Memory
	Log             *slog.Logger
	Logs            *testutil.LogBuffer
	modules         []httpapi.Module
}

// New deja la base y Redis vacíos, crea los índices (como scripts/indexes) y arma el módulo.
func New(t *testing.T, client *platformmongo.Client, rdb *goredis.Client, cfg config.Config) *Env {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, client.DB().Drop(ctx))
	require.NoError(t, rdb.FlushDB(ctx).Err())
	require.NoError(t, platformmongo.EnsureIndexes(ctx, client.DB(), indexes.All()...))
	return Build(t, client, rdb, cfg)
}

// MailFactory arma los dos mailers (peticiones y trabajo nocturno) con el logger del Env,
// para probar el correo real contra Mailpit (nv-06). Sin factory se usa el buzón en memoria.
type MailFactory func(log *slog.Logger) (requests, sync mailer.Mailer)

// NewWithMail es New con mailers propios.
func NewWithMail(t *testing.T, client *platformmongo.Client, rdb *goredis.Client, cfg config.Config, mail MailFactory) *Env {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, client.DB().Drop(ctx))
	require.NoError(t, rdb.FlushDB(ctx).Err())
	require.NoError(t, platformmongo.EnsureIndexes(ctx, client.DB(), indexes.All()...))
	return build(t, client, rdb, cfg, mail)
}

// Build arma el módulo sin limpiar nada (para una segunda instancia o un Redis caído).
func Build(t testing.TB, client *platformmongo.Client, rdb *goredis.Client, cfg config.Config) *Env {
	t.Helper()
	return build(t, client, rdb, cfg, nil)
}

func build(t testing.TB, client *platformmongo.Client, rdb *goredis.Client, cfg config.Config, factory MailFactory) *Env {
	t.Helper()
	log, logs := testutil.Logger()
	mail := mailer.NewMemory()
	var requests, sync mailer.Mailer = mail, mail
	if factory != nil {
		requests, sync = factory(log)
	}
	users := usuarios.NewRepository(client.DB()).WithCipher(testCipher(t, cfg.DataKeys))
	svc, err := auth.NewService(users, auth.NewRepository(client.DB()), rdb, requests, auth.Config{
		JWT:        middleware.JWTConfig{Key: cfg.JWTKey, Issuer: cfg.JWTIssuer, Audience: cfg.JWTAudience},
		AccessTTL:  cfg.AccessTTL,
		RefreshTTL: cfg.RefreshTTL,
	}, log)
	require.NoError(t, err)
	svc.WithSyncMailer(sync)
	v, err := validation.New()
	require.NoError(t, err)

	authRepo := auth.NewRepository(client.DB())
	cities := ciudades.NewService(ciudades.NewRepository(client.DB()))
	businessRepo := negocios.NewRepository(client.DB())
	cursor, err := pagination.NewSigner(cfg.CursorHMACKey)
	require.NoError(t, err)
	businesses := negocios.NewService(businessRepo, users, log).WithCursor(cursor).WithCache(cache.New(rdb, log))
	modelo := iatest.Nuevo()
	iaUses := ia.NewRegistroMongo(client.DB())
	// El doble va detrás de la guardia real (tope, caché y límites contra Redis y usos_ia).
	guardia, err := ia.NewGuardia(modelo, ia.NewContadorRedis(rdb), iaUses, ia.NewCacheRedis(rdb), guardiaConfig(cfg.IA, rdb), log)
	require.NoError(t, err)
	pesos, err := recomendador.PesosPorDefecto()
	require.NoError(t, err)
	assistant, err := asistente.NewService(guardia, businessRepo, users, asistente.NewDescartes(client.DB()),
		asistente.Config{Pesos: pesos, PlazoIA: PlazoIA}, log)
	require.NoError(t, err)
	vectores := iatest.NuevoServidorVectores(t)
	sem := semanticaDe(t, client, rdb, vectores.URL, pesos, log)
	assistant.WithSemantica(sem)
	recoRepo := recomendaciones.NewRepository(client.DB())
	searchRepo := busquedas.NewRepository(client.DB())
	searches := busquedas.NewService(searchRepo, log)
	friendRepo := amistades.NewRepository(client.DB())
	exports := privacidad.NewExports(client.DB())
	planRepo := planes.NewRepository(client.DB())
	planPrivacy := planes.NewPrivacy(planRepo)
	graph := grafo.NewQueue(client.DB(), log)
	groupRepo := grupos.NewRepository(client.DB())
	favRepo := favoritos.NewRepository(client.DB())
	syncRepo := sincronizacion.NewRepository(client.DB())
	syncActions := sincronizacion.NewActions(client.DB())
	voteRepo := votaciones.NewRepository(client.DB())
	chatRepo := chat.NewRepository(client.DB())
	modRepo := moderacion.NewRepository(client.DB())
	notifRepo := notificaciones.NewRepository(client.DB())
	notifier := notificaciones.NewService(notifRepo, cursor, log).WithPush(notificaciones.PusherFromConfig(cfg.FCM, users, notifRepo, log))
	plans := planes.NewService(planRepo, planes.NewCalculadora(businessRepo, planes.DefaultVelocidades())).WithGraph(graph).WithCursor(cursor).WithMembers(groupRepo).WithSync(syncRepo)
	consentRepo := usuarios.NewConsentRepository(client.DB())
	registry := privacidad.NewRegistry()
	for _, p := range []privacidad.Purger{auth.NewPrivacy(authRepo), exports, iaUses, planPrivacy, recomendaciones.NewPrivacy(recoRepo), busquedas.NewPrivacy(searchRepo), amistades.NewPrivacy(friendRepo), chat.NewPrivacy(chatRepo, groupRepo), grupos.NewPrivacy(groupRepo), votaciones.NewPrivacy(voteRepo), notificaciones.NewPrivacy(notifRepo), sincronizacion.NewPrivacy(syncRepo, syncActions), grafo.NewPrivacy(graph), usuarios.NewConsentPrivacy(consentRepo), moderacion.NewPrivacy(modRepo), favoritos.NewPrivacy(favRepo), account.NewPrivacy(users)} {
		registry.RegisterPurger(p)
	}
	registry.RegisterExporter(account.NewPrivacy(users))
	registry.RegisterExporter(auth.NewPrivacy(authRepo))
	registry.RegisterExporter(planPrivacy)
	registry.RegisterExporter(iaUses)
	registry.RegisterExporter(recomendaciones.NewPrivacy(recoRepo))
	registry.RegisterExporter(busquedas.NewPrivacy(searchRepo))
	registry.RegisterExporter(amistades.NewPrivacy(friendRepo))
	registry.RegisterExporter(grupos.NewPrivacy(groupRepo))
	registry.RegisterExporter(favoritos.NewPrivacy(favRepo))
	registry.RegisterExporter(votaciones.NewPrivacy(voteRepo))
	registry.RegisterExporter(chat.NewPrivacy(chatRepo, groupRepo))
	registry.RegisterExporter(notificaciones.NewPrivacy(notifRepo))
	registry.RegisterExporter(usuarios.NewConsentPrivacy(consentRepo))
	registry.RegisterExporter(moderacion.NewPrivacy(modRepo))
	almacen := negociostest.NuevoAlmacen()
	imagenes := negociostest.NuevasImagenes()
	exp := expediente.NewService(expediente.NewRepository(client.DB()), testCipher(t, cfg.DataKeys), almacen, log)
	expPrivacy := expediente.NewPrivacy(exp)
	registry.RegisterPurger(alta.NewPrivacy(businessRepo, businesses))
	registry.RegisterPurger(expPrivacy)
	registry.RegisterExporter(expPrivacy)
	altaSvc := alta.NewService(businessRepo, exp, users, cursor, alta.Config{AvisoVersion: cfg.VersionApp.Aviso}, log).
		WithImagenes(imagenes).WithFichas(businesses).WithNotifier(notifier)
	acc, err := account.NewService(users, svc, cities, exports, registry,
		account.Config{PhoneHMACKey: cfg.PhoneHMACKey, GraceDays: cfg.ArcoGraceDays}, log)
	require.NoError(t, err)
	acc.WithConsents(consentRepo, account.Versions{Terms: cfg.VersionApp.Terminos, Notice: cfg.VersionApp.Aviso}).WithRetiroDatosNegocio(expPrivacy)
	reco, err := recomendaciones.NewService(recoRepo, users, businessRepo, pesos, log)
	require.NoError(t, err)
	openMeteo := climatest.Nuevo(t)
	weather := clima.NewServicio(clima.NewOpenMeteo(openMeteo.URL), log)
	friends, err := amistades.NewService(friendRepo, users, log)
	require.NoError(t, err)
	friends.WithGraph(graph).WithNotifier(notifier)
	people, err := personas.NewService(users, friendRepo, cfg.PhoneHMACKey, log)
	require.NoError(t, err)
	suggestions, err := sugerencias.NewService(friendRepo, users, businessRepo, log)
	require.NoError(t, err)
	appErrors, err := errores.NewService(errores.NewRepository(client.DB()), cursor)
	require.NoError(t, err)

	mods := moderacion.NewService(modRepo, cursor, log)
	chats := chat.NewService(chatRepo, groupRepo, planRepo, users, cursor, log).WithNotifier(notifier)
	votes := votaciones.NewService(voteRepo, groupRepo, planRepo, plans, businessRepo, log).WithNotifier(notifier).WithChat(chats)
	return &Env{
		Cfg: cfg, Mongo: client, Redis: rdb, Users: users, Service: svc, Account: acc, Cities: cities, Negocios: businesses, Planes: plans, Recomendaciones: reco, Clima: weather, OpenMeteo: openMeteo, Exports: exports, Grafo: graph, IA: modelo, Votaciones: votes, Moderacion: mods, Chat: chats,
		Alta: altaSvc, Expediente: exp, Almacen: almacen, Imagenes: imagenes,
		Vectores: vectores, Semantica: sem, Sugerencias: suggestions, Busquedas: searches,
		Job: privacidad.NewJob(rdb, users, exports, registry, svc, log).WithTareas(expediente.NewRetencion(exp)), Mail: mail, Log: log, Logs: logs,
		modules: []httpapi.Module{
			auth.NewHandler(svc, v).WithActiveGuard(usuarios.RequireActive(users)),
			account.NewHandler(acc, v).WithActiveGuard(usuarios.RequireActive(users)),
			ciudades.NewHandler(cities, v),
			configuracion.NewHandler(configuracion.NewService(cfg.VersionApp)),
			// semantica y busquedas van antes que negocios: si no, /lugares/:id las atiende.
			semantica.NewHandler(sem, v),
			busquedas.NewHandler(searches, v).WithActiveGuard(usuarios.RequireActive(users)),
			negocios.NewHandler(businesses, v),
			alta.NewHandler(altaSvc, v).WithActiveGuard(usuarios.RequireActive(users)),
			moderacion.NewHandler(mods, v),
			asistente.NewHandler(assistant, v).WithActiveGuard(usuarios.RequireActive(users)),
			chat.NewHandler(chats, v).WithActiveGuard(usuarios.RequireActive(users)),       // antes que grupos
			votaciones.NewHandler(votes, v).WithActiveGuard(usuarios.RequireActive(users)), // antes que grupos
			grupos.NewHandler(grupos.NewService(groupRepo, planRepo, friendRepo, users, cursor).WithShare(businessRepo, cfg.Mail.LinkBase).WithNotifier(notifier), v).WithActiveGuard(usuarios.RequireActive(users)), // antes que planes
			favoritos.NewHandler(favoritos.NewService(favRepo, businessRepo, planRepo, cursor), v).WithActiveGuard(usuarios.RequireActive(users)),
			planes.NewHandler(plans, v).WithActiveGuard(usuarios.RequireActive(users)),
			recomendaciones.NewHandler(reco, v).WithActiveGuard(usuarios.RequireActive(users)),
			clima.NewHandler(weather, cities, v),
			inicio.NewHandler(inicio.NewService(reco, weather), v),
			notificaciones.NewHandler(notifier, v).WithActiveGuard(usuarios.RequireActive(users)),
			sincronizacion.NewHandler(sincronizacion.NewService(syncRepo, planRepo, users, cursor), v).
				WithPending(sincronizacion.NewPending(syncActions, plans, acc, users, v), usuarios.RequireActive(users)),
			errores.NewHandler(appErrors, v),
			personas.NewHandler(people).WithActiveGuard(usuarios.RequireActive(users)),
			sugerencias.NewHandler(suggestions).WithActiveGuard(usuarios.RequireActive(users)), // antes que amistades
			amistades.NewHandler(friends, v).WithActiveGuard(usuarios.RequireActive(users)),
		},
	}
}

// semanticaDe arma la búsqueda por significado (nv-51) con el adaptador real de embeddings
// contra el servidor local, detrás de los límites reales en Redis.
func semanticaDe(t testing.TB, client *platformmongo.Client, rdb *goredis.Client, url string, pesos recomendador.Pesos, log *slog.Logger) *semantica.Service {
	t.Helper()
	g, err := ia.NewGeminiVectores(ia.GeminiVectoresConfig{APIKey: iatest.LlaveFalsa, ModeloID: "gemini-embedding-2", Timeout: PlazoIA, URLBase: url})
	require.NoError(t, err)
	lim, err := ia.NewVectoresLimitados(g, ia.NewLimitadorRedis(rdb), ia.LimitesVectores{PorMinuto: 80, PorDia: 800})
	require.NoError(t, err)
	repo := semantica.NewRepository(client.DB())
	sem, err := semantica.NewService(lim, semantica.NewProductoPunto(repo), repo, ia.NewCacheRedis(rdb), semantica.Config{Pesos: pesos}, log)
	require.NoError(t, err)
	return sem
}

// PlazoIA acorta el plazo del asistente (7 s en producción) para probar el respaldo sin
// esperar: un doble con Tardar mayor a esto termina en el motor de reglas.
const PlazoIA = 2 * time.Second

// guardiaConfig toma el tope y los límites de cfg; los que vienen en cero (testutil.Config no
// los fija) usan los valores por defecto de producción.
func guardiaConfig(c config.IA, rdb *goredis.Client) ia.GuardiaConfig {
	pick := func(v, def int) int {
		if v > 0 {
			return v
		}
		return def
	}
	tope := c.TopeUSDMes
	if tope <= 0 {
		tope = 5
	}
	return ia.GuardiaConfig{
		TopeUSDMes: tope,
		Precios:    ia.Precios{EntradaUSDMTok: c.PrecioEntradaUSDMTok, SalidaUSDMTok: c.PrecioSalidaUSDMTok},
		Limites:    ia.Limites{PorHora: pick(c.LimiteHora, 10), PorMinuto: pick(c.LimiteMin, 12), PorDia: pick(c.LimiteDia, 400)},
		Limitador:  ia.NewLimitadorRedis(rdb),
	}
}

// App construye la app real con el módulo auth montado.
func (e *Env) App() *fiber.App {
	return httpapi.NewApp(e.Cfg, httpapi.Deps{
		Logger:  e.Log,
		Version: "v0.0.0-test",
		Redis:   e.Redis,
		Mongo:   e.Mongo,
		Neo4j:   testutil.DownPinger{},
		Modules: e.modules,
	})
}

// CreateUser inserta un usuario activo con rol consumidor y la contraseña de prueba.
func (e *Env) CreateUser(t *testing.T, email string) usuarios.User {
	t.Helper()
	hasher, err := auth.NewHasher()
	require.NoError(t, err)
	hash, err := hasher.Hash(Password)
	require.NoError(t, err)
	birth := time.Date(1995, 5, 17, 0, 0, 0, 0, time.UTC)
	u := usuarios.User{
		Name: "Prueba", NormalizedName: "prueba", Email: email, PasswordHash: hash, BirthDate: &birth,
		Roles: []string{middleware.RoleConsumer}, Status: usuarios.StatusActive,
	}
	require.NoError(t, e.Users.Create(context.Background(), &u))
	return u
}

// CreateAdmin inserta un usuario activo con rol administrador y la contraseña de prueba.
func (e *Env) CreateAdmin(t *testing.T, email string) usuarios.User {
	t.Helper()
	hasher, err := auth.NewHasher()
	require.NoError(t, err)
	hash, err := hasher.Hash(Password)
	require.NoError(t, err)
	u := usuarios.User{
		Name: "Admin", NormalizedName: "admin", Email: email, PasswordHash: hash,
		Roles: []string{middleware.RoleAdmin}, Status: usuarios.StatusActive,
	}
	require.NoError(t, e.Users.Create(context.Background(), &u))
	return u
}

// ClearRateLimits borra los contadores `rate:*` para que las pruebas no choquen con el
// límite de login (5 por IP cada 5 min); los fallos de login no se tocan.
func (e *Env) ClearRateLimits(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	keys, err := e.Redis.Keys(ctx, "rate:*").Result()
	require.NoError(t, err)
	if len(keys) > 0 {
		require.NoError(t, e.Redis.Del(ctx, keys...).Err())
	}
}

// ExpireLinks vence todos los tokens de enlace del usuario (sin esperar al TTL de MongoDB).
func (e *Env) ExpireLinks(t *testing.T, userID bson.ObjectID) {
	t.Helper()
	_, err := e.Mongo.DB().Collection(auth.TokensCollection).UpdateMany(context.Background(),
		bson.D{{Key: "usuario_id", Value: userID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "expira_en", Value: time.Now().UTC().Add(-time.Minute)}}}})
	require.NoError(t, err)
}

// AgeLinks mueve la creación de los enlaces del usuario al pasado para saltar el reenvío de 60 s.
func (e *Env) AgeLinks(t *testing.T, userID bson.ObjectID) {
	t.Helper()
	_, err := e.Mongo.DB().Collection(auth.TokensCollection).UpdateMany(context.Background(),
		bson.D{{Key: "usuario_id", Value: userID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "creado_en", Value: time.Now().UTC().Add(-2 * time.Minute)}}}})
	require.NoError(t, err)
}

// testCipher arma el llavero de la configuración de prueba (nil si la prueba lo quitó).
func testCipher(t testing.TB, keys string) *cifrado.Llavero {
	t.Helper()
	if keys == "" {
		return nil
	}
	l, err := cifrado.CargarLlavero(keys)
	require.NoError(t, err)
	return l
}
