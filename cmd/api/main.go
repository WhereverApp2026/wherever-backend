// Command api arranca la API de Wherever.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata" // el trabajo nocturno usa America/Mexico_City aunque la imagen no traiga zonas

	"github.com/gofiber/fiber/v2"
	goredis "github.com/redis/go-redis/v9"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/asistente"
	"github.com/WhereverApp2026/wherever-backend/internal/auth"
	"github.com/WhereverApp2026/wherever-backend/internal/busquedas"
	"github.com/WhereverApp2026/wherever-backend/internal/ciudades"
	"github.com/WhereverApp2026/wherever-backend/internal/clima"
	"github.com/WhereverApp2026/wherever-backend/internal/configuracion"
	"github.com/WhereverApp2026/wherever-backend/internal/embeddings"
	"github.com/WhereverApp2026/wherever-backend/internal/grafo"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	httpapi "github.com/WhereverApp2026/wherever-backend/internal/http"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/indexes"
	"github.com/WhereverApp2026/wherever-backend/internal/inicio"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/personas"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/cache"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/logger"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mailer"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/redis"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendador"
	"github.com/WhereverApp2026/wherever-backend/internal/semantica"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/sugerencias"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios/account"
	"github.com/WhereverApp2026/wherever-backend/internal/votaciones"
)

// version se fija al compilar: -ldflags "-X main.version=v0.1.0".
var version = "dev"

const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error al arrancar:", err)
		os.Exit(1)
	}
}

func run() error {
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	log := logger.New(os.Stdout, cfg.Env)

	mongoClient, err := mongo.New(cfg.MongoURI)
	if err != nil {
		return fmt.Errorf("mongodb: %w", err)
	}
	redisClient, err := redis.New(cfg.RedisAddr)
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	neo4jDriver, err := neo4j.New(cfg.Neo4jURI)
	if err != nil {
		return fmt.Errorf("neo4j: %w", err)
	}

	mail := buildMail(cfg.Mail, log)
	graphQueue := grafo.NewQueue(mongoClient.DB(), log)
	modules, job, err := buildModules(cfg, log, mongoClient, redisClient, mail, graphStores{queue: graphQueue, driver: neo4jDriver})
	if err != nil {
		return err
	}
	stopJob, err := job.Start(cfg.JobHour)
	if err != nil {
		return fmt.Errorf("trabajo nocturno: %w", err)
	}
	defer func() { _ = stopJob() }()

	app := httpapi.NewApp(cfg, httpapi.Deps{
		Logger:  log,
		Version: version,
		Redis:   redisClient,
		Mongo:   mongoClient,
		Neo4j:   neo4jDriver,
		// La API no crea índices (constitución §5); /ready da 503 si falta alguno.
		MongoIndexes: mongo.NewIndexVerifier(mongoClient.DB(), indexes.All()...),
		Modules:      modules,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	waitGraph := startGraph(ctx, mongoClient.DB(), graphQueue, neo4jDriver, log)
	waitVotes := startVotes(ctx, mongoClient.DB(), graphQueue, log)

	serveErr := make(chan error, 1)
	go func() { serveErr <- app.Listen(":" + strconv.Itoa(cfg.Port)) }()
	log.Info("API escuchando", "puerto", cfg.Port, "entorno", cfg.Env, "version", version)

	select {
	case err = <-serveErr:
	case <-ctx.Done():
		log.Info("apagando")
		err = app.ShutdownWithTimeout(shutdownTimeout)
	}

	stop()
	waitGraph()
	waitVotes()
	closeCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	// Antes de cerrar las bases se vacía la cola de correo (con el mismo plazo).
	return errors.Join(err, mail.shutdown(closeCtx), mongoClient.Close(closeCtx), redisClient.Close(), neo4jDriver.Close(closeCtx))
}

// buildModules arma los módulos de negocio y el trabajo nocturno con sus dependencias.
func buildModules(cfg config.Config, log *slog.Logger, mongoClient *mongo.Client, redisClient *goredis.Client, mail mailers, graph graphStores) ([]httpapi.Module, *privacidad.Job, error) {
	validator, err := validation.New()
	if err != nil {
		return nil, nil, err
	}
	db := mongoClient.DB()
	users := usuarios.NewRepository(db)
	authRepo := auth.NewRepository(db)
	authSvc, err := auth.NewService(users, authRepo, redisClient, mail.requests, auth.Config{
		JWT:        middleware.JWTConfig{Key: cfg.JWTKey, Issuer: cfg.JWTIssuer, Audience: cfg.JWTAudience},
		AccessTTL:  cfg.AccessTTL,
		RefreshTTL: cfg.RefreshTTL,
	}, log)
	if err != nil {
		return nil, nil, fmt.Errorf("auth: %w", err)
	}
	authSvc.WithSyncMailer(mail.sync)
	citySvc := ciudades.NewService(ciudades.NewRepository(db))
	businessRepo := negocios.NewRepository(db)
	businessSvc, cursor, err := buildBusinesses(cfg, businessRepo, users, redisClient, db, log)
	if err != nil {
		return nil, nil, err
	}
	exports := privacidad.NewExports(db)
	iaUses := ia.NewRegistroMongo(db)

	// ARCO: el documento del usuario se borra al final (si otro módulo falla, se reintenta).
	planRepo := planes.NewRepository(db)
	planPrivacy := planes.NewPrivacy(planRepo)
	// Los integrantes del grupo leen el plan (nv-37).
	groupRepo := grupos.NewRepository(db)
	voteRepo := votaciones.NewRepository(db)
	planSvc := planes.NewService(planRepo, planes.NewCalculadora(businessRepo, planes.DefaultVelocidades())).WithGraph(graph.queue).WithCursor(cursor).WithMembers(groupRepo)
	recoRepo := recomendaciones.NewRepository(db)
	searchRepo := busquedas.NewRepository(db)
	friendRepo := amistades.NewRepository(db)
	consentRepo := usuarios.NewConsentRepository(db)
	registry := privacidad.NewRegistry()
	for _, p := range []privacidad.Purger{auth.NewPrivacy(authRepo), exports, iaUses, planPrivacy, recomendaciones.NewPrivacy(recoRepo), busquedas.NewPrivacy(searchRepo), amistades.NewPrivacy(friendRepo), grupos.NewPrivacy(groupRepo), votaciones.NewPrivacy(voteRepo), grafo.NewPrivacy(graph.queue), usuarios.NewConsentPrivacy(consentRepo), account.NewPrivacy(users)} {
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
	registry.RegisterExporter(votaciones.NewPrivacy(voteRepo))
	registry.RegisterExporter(usuarios.NewConsentPrivacy(consentRepo))

	// Los pesos del recomendador se validan aquí: con un JSON inválido la API no arranca (nv-24).
	pesos, err := recomendador.PesosPorDefecto()
	if err != nil {
		return nil, nil, fmt.Errorf("recomendador: %w", err)
	}
	recoSvc, err := recomendaciones.NewService(recoRepo, users, businessRepo, pesos, log)
	if err != nil {
		return nil, nil, fmt.Errorf("recomendaciones: %w", err)
	}

	// Clima de Open-Meteo (nv-25): sin llave, caché en memoria por instancia, sin Redis.
	weather := clima.NewServicio(clima.NewOpenMeteo(clima.OpenMeteoURL), log)

	accountSvc, err := account.NewService(users, authSvc, citySvc, exports, registry,
		account.Config{PhoneHMACKey: cfg.PhoneHMACKey, GraceDays: cfg.ArcoGraceDays}, log)
	if err != nil {
		return nil, nil, fmt.Errorf("usuarios: %w", err)
	}
	// Con la cuenta en eliminación pendiente solo quedan GET /usuarios/yo y cancelar (nv-05).
	active := usuarios.RequireActive(users)
	social, err := buildSocial(socialDeps{users: users, friends: friendRepo, places: businessRepo, graph: graph, phoneKey: cfg.PhoneHMACKey, validator: validator, active: active}, log)
	if err != nil {
		return nil, nil, err
	}
	assistant, sem, err := buildAsistente(cfg.IA, iaDeps{redis: redisClient, usos: iaUses, db: db}, businessRepo, users, asistente.NewDescartes(db), log)
	if err != nil {
		return nil, nil, fmt.Errorf("asistente: %w", err)
	}
	modules := []httpapi.Module{
		auth.NewHandler(authSvc, validator).WithActiveGuard(active),
		// Aceptación de términos y aviso (nv-19) con las versiones que publica /configuracion.
		account.NewHandler(accountSvc.WithConsents(consentRepo, account.Versions{Terms: cfg.VersionApp.Terminos, Notice: cfg.VersionApp.Aviso}), validator).WithActiveGuard(active),
		ciudades.NewHandler(citySvc, validator),
		configuracion.NewHandler(configuracion.NewService(cfg.VersionApp)),
		// semantica y busquedas van antes que negocios: si no, /lugares/:id las atiende.
		semantica.NewHandler(sem, validator),
		busquedas.NewHandler(busquedas.NewService(searchRepo, log), validator).WithActiveGuard(active),
		negocios.NewHandler(businessSvc, validator),
		asistente.NewHandler(assistant, validator).WithActiveGuard(active),
		// votaciones va antes que grupos y grupos antes que planes: atienden rutas de esos prefijos (nv-37, nv-39).
		votaciones.NewHandler(votaciones.NewService(voteRepo, groupRepo, planRepo, planSvc, businessRepo, log), validator).WithActiveGuard(active),
		grupos.NewHandler(grupos.NewService(groupRepo, planRepo, friendRepo, users, cursor).WithShare(businessRepo, cfg.Mail.LinkBase), validator).WithActiveGuard(active),
		planes.NewHandler(planSvc, validator).WithActiveGuard(active),
		recomendaciones.NewHandler(recoSvc, validator).WithActiveGuard(active),
		clima.NewHandler(weather, citySvc, validator),
		inicio.NewHandler(inicio.NewService(recoSvc, weather), validator),
	}
	modules = append(modules, social...)
	return modules, privacidad.NewJob(redisClient, users, exports, registry, authSvc, log), nil
}

// buildBusinesses arma el service de negocios con el cursor firmado (nv-17), que también usa
// planes, y los vectores al importar (nv-50). Sin IA_EMBEDDINGS_ID o sin llave no hay
// vectores y todo sigue con el motor de reglas.
func buildBusinesses(cfg config.Config, repo *negocios.Repository, users *usuarios.Repository, rdb *goredis.Client, db *mongodriver.Database, log *slog.Logger) (*negocios.Service, *pagination.Signer, error) {
	cursor, err := pagination.NewSigner(cfg.CursorHMACKey)
	if err != nil {
		return nil, nil, fmt.Errorf("cursor: %w", err)
	}
	svc := negocios.NewService(repo, users, log).WithCursor(cursor).WithCache(cache.New(rdb, log))
	rec, err := embeddings.Armar(cfg.IA, rdb, db, "")
	if err != nil {
		return nil, nil, fmt.Errorf("embeddings: %w", err)
	}
	if rec == nil {
		log.Warn("sin GEMINI_API_KEY o IA_EMBEDDINGS_ID: los negocios no tienen vectores de significado")
		return svc, cursor, nil
	}
	svc.WithEmbeddings(rec)
	return svc, cursor, nil
}

// buildSemantica arma la búsqueda por significado (nv-51) con el mismo vectorizador limitado
// del recálculo. Sin IA de embeddings, GET /lugares/semantica responde el motor de reglas.
func buildSemantica(cfg config.IA, rdb *goredis.Client, db *mongodriver.Database, pesos recomendador.Pesos, log *slog.Logger) (*semantica.Service, error) {
	vect, err := embeddings.Vectorizador(cfg, rdb, "")
	if err != nil {
		return nil, err
	}
	repo := semantica.NewRepository(db)
	return semantica.NewService(vect, semantica.NewProductoPunto(repo), repo, ia.NewCacheRedis(rdb), semantica.Config{Pesos: pesos}, log)
}

// iaDeps son las bases que usa ia.Guardia: Redis (gasto, caché y límites) y usos_ia.
type iaDeps struct {
	redis *goredis.Client
	usos  *ia.RegistroMongo
	db    *mongodriver.Database
}

// buildAsistente arma el asistente de planes (nv-31) con Gemini detrás de ia.Guardia (tope,
// caché y límites de nv-27/nv-28). Sin GEMINI_API_KEY o IA_MODELO_ID la IA queda apagada y
// POST /planes/generar responde con el motor de reglas (nv-33).
func buildAsistente(cfg config.IA, deps iaDeps, fuente recomendador.Fuente, users asistente.Usuarios, descartes asistente.Descartes, log *slog.Logger) (*asistente.Service, *semantica.Service, error) {
	pesos, err := recomendador.PesosPorDefecto()
	if err != nil {
		return nil, nil, err
	}
	var modelo ia.Modelo
	if cfg.Enabled() {
		g, err := ia.NewGemini(ia.GeminiConfig{APIKey: cfg.GeminiAPIKey, ModeloID: cfg.ModeloID, Timeout: cfg.Timeout})
		if err != nil {
			return nil, nil, err
		}
		guardia, err := ia.NewGuardia(g, ia.NewContadorRedis(deps.redis), deps.usos, ia.NewCacheRedis(deps.redis), ia.GuardiaConfig{
			TopeUSDMes: cfg.TopeUSDMes,
			Precios:    ia.Precios{EntradaUSDMTok: cfg.PrecioEntradaUSDMTok, SalidaUSDMTok: cfg.PrecioSalidaUSDMTok},
			Limites:    ia.Limites{PorHora: cfg.LimiteHora, PorMinuto: cfg.LimiteMin, PorDia: cfg.LimiteDia},
			Limitador:  ia.NewLimitadorRedis(deps.redis),
		}, log)
		if err != nil {
			return nil, nil, err
		}
		modelo = guardia
	} else {
		log.Warn("sin GEMINI_API_KEY o IA_MODELO_ID: el asistente de planes responde solo con el motor de reglas")
	}
	sem, err := buildSemantica(cfg, deps.redis, deps.db, pesos, log)
	if err != nil {
		return nil, nil, fmt.Errorf("semantica: %w", err)
	}
	assistant, err := asistente.NewService(modelo, fuente, users, descartes, asistente.Config{Pesos: pesos, Velocidades: planes.DefaultVelocidades()}, log)
	if err != nil {
		return nil, nil, err
	}
	return assistant.WithSemantica(sem), sem, nil
}

// mailers son los dos caminos del correo (nv-06): las peticiones encolan y nunca esperan al
// servidor; el trabajo nocturno envía directo porque necesita saber si el aviso salió.
type mailers struct {
	requests mailer.Mailer
	sync     mailer.Mailer
	shutdown func(context.Context) error
}

// buildMail arma el correo según la configuración. Sin SMTP_HOST (solo dev y staging; prod no
// arranca) se usa el Log, que registra tipo y destinatario enmascarado.
func buildMail(cfg config.Mail, log *slog.Logger) mailers {
	if !cfg.Enabled() {
		log.Warn("sin SMTP_HOST: los correos no se envían, solo se registran (tipo y destinatario enmascarado)")
		logMailer := mailer.NewLog(log)
		return mailers{requests: logMailer, sync: logMailer, shutdown: func(context.Context) error { return nil }}
	}
	smtpMailer := mailer.NewSMTP(mailer.SMTPConfig{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Password: cfg.SMTPPassword, TLSMode: cfg.SMTPTLS, From: cfg.From,
	}, mailer.NewLinks(cfg.LinkBase), log)
	direct := mailer.NewRetrying(smtpMailer, mailer.DefaultAttempts, mailer.DefaultRetryWait, cfg.Timeout)
	queue := mailer.NewAsync(direct, cfg.Workers, cfg.QueueSize, log)
	return mailers{requests: queue, sync: direct, shutdown: queue.Shutdown}
}

// graphStores son la cola que llena Neo4j (nv-52) y el driver que lo lee (nv-53).
type graphStores struct {
	queue  *grafo.Queue
	driver *neo4j.Driver
}

// socialDeps es lo que necesitan los módulos sociales.
type socialDeps struct {
	users     *usuarios.Repository
	friends   *amistades.Repository
	places    *negocios.Repository
	graph     graphStores
	phoneKey  []byte
	validator *validation.Validator
	active    fiber.Handler
}

// buildSocial arma la búsqueda de personas (nv-35), las sugerencias y la señal social del
// grafo (nv-53) y amistades (nv-36). Sugerencias va antes que amistades: su grupo
// /amistades ya pide JWT y la ruta no debe pasar dos veces por él.
func buildSocial(d socialDeps, log *slog.Logger) ([]httpapi.Module, error) {
	friends, err := amistades.NewService(d.friends, d.users, log)
	if err != nil {
		return nil, fmt.Errorf("amistades: %w", err)
	}
	friends.WithGraph(d.graph.queue)
	people, err := personas.NewService(d.users, d.friends, d.phoneKey, log)
	if err != nil {
		return nil, err
	}
	suggestions, err := sugerencias.NewService(d.friends, d.users, d.places, log)
	if err != nil {
		return nil, err
	}
	suggestions.WithGraph(d.graph.driver)
	return []httpapi.Module{
		personas.NewHandler(people).WithActiveGuard(d.active),
		sugerencias.NewHandler(suggestions).WithActiveGuard(d.active),
		amistades.NewHandler(friends, d.validator).WithActiveGuard(d.active),
	}, nil
}

// startGraph corre el trabajador que refleja amistades y planes en Neo4j (nv-52) hasta que
// ctx termina; la función devuelta espera a que suelte las bases.
func startGraph(ctx context.Context, db *mongodriver.Database, queue *grafo.Queue, driver *neo4j.Driver, log *slog.Logger) func() {
	worker := grafo.NewWorker(queue, amistades.NewRepository(db), planes.NewRepository(db), driver, log)
	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()
	return func() { <-done }
}

// startVotes cierra cada minuto las votaciones vencidas y aplica su ganadora al plan (nv-39)
// hasta que ctx termina. Solo usa MongoDB; la función devuelta espera a que termine.
func startVotes(ctx context.Context, db *mongodriver.Database, queue *grafo.Queue, log *slog.Logger) func() {
	planRepo := planes.NewRepository(db)
	places := negocios.NewRepository(db)
	plans := planes.NewService(planRepo, planes.NewCalculadora(places, planes.DefaultVelocidades())).WithGraph(queue)
	svc := votaciones.NewService(votaciones.NewRepository(db), grupos.NewRepository(db), planRepo, plans, places, log)
	closer := votaciones.NewCloser(svc, log)
	done := make(chan struct{})
	go func() {
		closer.Run(ctx)
		close(done)
	}()
	return func() { <-done }
}
