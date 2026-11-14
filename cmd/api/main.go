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
	"github.com/WhereverApp2026/wherever-backend/internal/chat"
	"github.com/WhereverApp2026/wherever-backend/internal/ciudades"
	"github.com/WhereverApp2026/wherever-backend/internal/clima"
	"github.com/WhereverApp2026/wherever-backend/internal/configuracion"
	"github.com/WhereverApp2026/wherever-backend/internal/embeddings"
	"github.com/WhereverApp2026/wherever-backend/internal/errores"
	"github.com/WhereverApp2026/wherever-backend/internal/favoritos"
	"github.com/WhereverApp2026/wherever-backend/internal/grafo"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	httpapi "github.com/WhereverApp2026/wherever-backend/internal/http"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
	"github.com/WhereverApp2026/wherever-backend/internal/indexes"
	"github.com/WhereverApp2026/wherever-backend/internal/inicio"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/alta"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/menu"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/panel"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/pagos"
	"github.com/WhereverApp2026/wherever-backend/internal/personas"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/cache"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/cifrado"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/logger"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mailer"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/redis"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendador"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/semantica"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/sincronizacion"
	"github.com/WhereverApp2026/wherever-backend/internal/sugerencias"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios/account"
	"github.com/WhereverApp2026/wherever-backend/internal/vaca"
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
	waitVotes := startVotes(ctx, mongoClient.DB(), graphQueue, cfg.FCM, cfg.DataKeys, log)
	waitBookings := startReservas(ctx, mongoClient.DB(), log)

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
	waitBookings()
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
	users := usuarios.NewRepository(db).WithCipher(fieldCipher(cfg.DataKeys, log))
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
	// Los integrantes del grupo leen el plan (nv-37); cada cambio se numera para la sincronización (nv-46).
	groupRepo := grupos.NewRepository(db)
	favRepo := favoritos.NewRepository(db)
	syncRepo := sincronizacion.NewRepository(db)
	syncActions := sincronizacion.NewActions(db)
	voteRepo := votaciones.NewRepository(db)
	chatRepo := chat.NewRepository(db)
	modRepo := moderacion.NewRepository(db)
	// Cola de revisión y reglas (nv-43): las fotos leen ahí sus umbrales y mandan lo dudoso.
	modSvc := moderacion.NewService(modRepo, cursor, log)
	// Centro de avisos (nv-40): amistades, grupos y votaciones guardan ahí sus avisos.
	notifRepo := notificaciones.NewRepository(db)
	notifier := notificaciones.NewService(notifRepo, cursor, log).WithPush(notificaciones.PusherFromConfig(cfg.FCM, users, notifRepo, log))
	planSvc := planes.NewService(planRepo, planes.NewCalculadora(businessRepo, planes.DefaultVelocidades())).WithGraph(graph.queue).WithCursor(cursor).WithMembers(groupRepo).WithSync(syncRepo)
	recoRepo := recomendaciones.NewRepository(db)
	searchRepo := busquedas.NewRepository(db)
	friendRepo := amistades.NewRepository(db)
	consentRepo := usuarios.NewConsentRepository(db)
	images, imagePrivacy, err := buildImagenes(cfg.Imagenes, db, modSvc, log)
	if err != nil {
		return nil, nil, fmt.Errorf("imagenes: %w", err)
	}
	registry := privacidad.NewRegistry()
	for _, p := range []privacidad.Purger{imagePrivacy, auth.NewPrivacy(authRepo), exports, iaUses, planPrivacy, recomendaciones.NewPrivacy(recoRepo), busquedas.NewPrivacy(searchRepo), amistades.NewPrivacy(friendRepo), chat.NewPrivacy(chatRepo, groupRepo), grupos.NewPrivacy(groupRepo), votaciones.NewPrivacy(voteRepo), notificaciones.NewPrivacy(notifRepo), sincronizacion.NewPrivacy(syncRepo, syncActions), grafo.NewPrivacy(graph.queue), usuarios.NewConsentPrivacy(consentRepo), moderacion.NewPrivacy(modRepo), favoritos.NewPrivacy(favRepo), account.NewPrivacy(users)} {
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
	vacaRepo := vaca.NewRepository(db)
	registry.RegisterExporter(vaca.NewExpensesPrivacy(vacaRepo))
	registry.RegisterExporter(vaca.NewDebtsPrivacy(vacaRepo))
	registry.RegisterExporter(favoritos.NewPrivacy(favRepo))
	registry.RegisterExporter(votaciones.NewPrivacy(voteRepo))
	registry.RegisterExporter(chat.NewPrivacy(chatRepo, groupRepo))
	registry.RegisterExporter(notificaciones.NewPrivacy(notifRepo))
	registry.RegisterExporter(usuarios.NewConsentPrivacy(consentRepo))
	registry.RegisterExporter(moderacion.NewPrivacy(modRepo))
	bookings := buildReservas(cfg, db, businessRepo, users, redisClient, cursor, registry, log)
	business := buildAlta(cfg, db, businessRepo, businessSvc, users, cursor, images, notifier, registry, bookings, log)
	merchant := buildPanel(db, businessRepo, businessSvc, business, images, notifier, registry, bookings, log)
	registry.RegisterExporter(imagePrivacy)

	recoSvc, err := buildRecomendaciones(recoRepo, users, businessRepo, log)
	if err != nil {
		return nil, nil, err
	}

	// Clima de Open-Meteo (nv-25): sin llave, caché en memoria por instancia, sin Redis.
	weather := clima.NewServicio(clima.NewOpenMeteo(clima.OpenMeteoURL), log)

	accountSvc, err := account.NewService(users, authSvc, citySvc, exports, registry,
		account.Config{PhoneHMACKey: cfg.PhoneHMACKey, GraceDays: cfg.ArcoGraceDays}, log)
	if err != nil {
		return nil, nil, fmt.Errorf("usuarios: %w", err)
	}
	accountSvc.WithRetiroDatosNegocio(business.privacy)
	// Con la cuenta en eliminación pendiente solo quedan GET /usuarios/yo y cancelar (nv-05).
	active := usuarios.RequireActive(users)
	social, err := buildExtras(socialDeps{users: users, friends: friendRepo, places: businessRepo, graph: graph, notifier: notifier, phoneKey: cfg.PhoneHMACKey, validator: validator, active: active}, db, cursor, log)
	if err != nil {
		return nil, nil, err
	}
	assistant, sem, err := buildAsistente(cfg.IA, iaDeps{redis: redisClient, usos: iaUses, db: db}, businessRepo, users, asistente.NewDescartes(db), log)
	if err != nil {
		return nil, nil, fmt.Errorf("asistente: %w", err)
	}
	// Chat de grupo (nv-57): REST y push agrupado, sin canal en vivo ni Redis.
	chatSvc := chat.NewService(chatRepo, groupRepo, planRepo, users, cursor, log).WithNotifier(notifier)
	modules := []httpapi.Module{
		auth.NewHandler(authSvc, validator).WithActiveGuard(active),
		// Aceptación de términos y aviso (nv-19) con las versiones que publica /configuracion.
		account.NewHandler(accountSvc.WithConsents(consentRepo, account.Versions{Terms: cfg.VersionApp.Terminos, Notice: cfg.VersionApp.Aviso}), validator).WithActiveGuard(active),
		ciudades.NewHandler(citySvc, validator),
		configuracion.NewHandler(configuracion.NewService(cfg.VersionApp)),
		// semantica y busquedas van antes que negocios: si no, /lugares/:id las atiende.
		semantica.NewHandler(sem, validator),
		busquedas.NewHandler(busquedas.NewService(searchRepo, log), validator).WithActiveGuard(active),
		// El menú va antes que negocios: si no, /lugares/:id atiende /lugares/:id/menu.
		menu.NewHandler(merchant.menu, validator).WithActiveGuard(active),
		negocios.NewHandler(businessSvc, validator),
		alta.NewHandler(business.svc, validator).WithActiveGuard(active),
		panel.NewHandler(merchant.panel, validator).WithActiveGuard(active),
		reservas.NewHandler(bookings.svc, validator).WithActiveGuard(active),
		pagos.NewHandler(bookings.pagos, validator).WithActiveGuard(active),
		// Cola de revisión y reglas (nv-43); las fotos de nv-45 entran y se publican por aquí.
		moderacion.NewHandler(modSvc, validator),
		asistente.NewHandler(assistant, validator).WithActiveGuard(active),
		// chat y votaciones van antes que grupos y grupos antes que planes: atienden rutas de esos prefijos (nv-37, nv-39, nv-57).
		chat.NewHandler(chatSvc, validator).WithActiveGuard(active),
		votaciones.NewHandler(votaciones.NewService(voteRepo, groupRepo, planRepo, planSvc, businessRepo, log).WithNotifier(notifier).WithChat(chatSvc), validator).WithActiveGuard(active),
		grupos.NewHandler(grupos.NewService(groupRepo, planRepo, friendRepo, users, cursor).WithShare(businessRepo, cfg.Mail.LinkBase).WithNotifier(notifier), validator).WithActiveGuard(active),
		vaca.NewHandler(vaca.NewService(vacaRepo, groupRepo, users, vaca.NewRedisLocker(redisClient), cursor), validator).WithActiveGuard(active),
		favoritos.NewHandler(favoritos.NewService(favRepo, businessRepo, planRepo, cursor), validator).WithActiveGuard(active),
		planes.NewHandler(planSvc, validator).WithActiveGuard(active),
		recomendaciones.NewHandler(recoSvc, validator).WithActiveGuard(active),
		clima.NewHandler(weather, citySvc, validator),
		inicio.NewHandler(inicio.NewService(recoSvc, weather), validator),
		notificaciones.NewHandler(notifier, validator).WithActiveGuard(active),
		imagenes.NewHandler(images, validator).WithActiveGuard(active),
		sincronizacion.NewHandler(sincronizacion.NewService(syncRepo, planRepo, users, cursor), validator).
			WithPending(sincronizacion.NewPending(syncActions, planSvc, accountSvc, users, validator), active),
	}
	modules = append(modules, social...)
	return modules, privacidad.NewJob(redisClient, users, exports, registry, authSvc, log).WithTareas(business.retencion, merchant.tarea, pagos.NewTarea(bookings.pagos)), nil
}

// buildRecomendaciones valida los pesos del recomendador: con un JSON inválido la API no
// arranca (nv-24).
func buildRecomendaciones(repo *recomendaciones.Repository, users recomendaciones.Users, places recomendaciones.Places, log *slog.Logger) (*recomendaciones.Service, error) {
	pesos, err := recomendador.PesosPorDefecto()
	if err != nil {
		return nil, fmt.Errorf("recomendador: %w", err)
	}
	svc, err := recomendaciones.NewService(repo, users, places, pesos, log)
	if err != nil {
		return nil, fmt.Errorf("recomendaciones: %w", err)
	}
	return svc, nil
}

// buildImagenes arma la subida de fotos (nv-45) con S3 y Rekognition por el rol de la
// instancia. Sin IMAGENES_BUCKET e IMAGENES_URL (solo fuera de prod) no hay service: las rutas
// responden 503 y ARCO igual borra las filas.
func buildImagenes(cfg config.Imagenes, db *mongodriver.Database, mods *moderacion.Service, log *slog.Logger) (*imagenes.Service, *imagenes.Privacy, error) {
	repo := imagenes.NewRepository(db)
	if !cfg.Enabled() {
		log.Warn("sin IMAGENES_BUCKET e IMAGENES_URL: /api/v1/imagenes responde 503")
		return nil, imagenes.NewPrivacy(repo, nil, ""), nil
	}
	ctx := context.Background()
	almacen, err := imagenes.NewS3(ctx, imagenes.S3Config{Bucket: cfg.Bucket, Region: cfg.Region})
	if err != nil {
		return nil, nil, err
	}
	rek, err := moderacion.NewRekognition(ctx, moderacion.RekognitionConfig{Region: cfg.Region})
	if err != nil {
		return nil, nil, err
	}
	// Solo imágenes: el texto (reseñas, postales) lo cablea su spec con la ia.Guardia.
	// Los umbrales salen de moderacion_reglas en cada evaluación (nv-43); lo dudoso va a la cola
	// y la decisión del administrador publica o descarta la foto.
	mod := moderacion.New(rek, nil, mods, log)
	svc, err := imagenes.NewService(repo, almacen, mod, imagenes.Config{URLPublica: cfg.URL}, log)
	if err != nil {
		return nil, nil, err
	}
	svc.WithCola(mods)
	mods.WithDestino(imagenes.TipoCola, svc.Destino())
	return svc, imagenes.NewPrivacy(repo, almacen, cfg.URL), nil
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
	notifier  notificaciones.Notifier
	phoneKey  []byte
	validator *validation.Validator
	active    fiber.Handler
}

// buildExtras arma los módulos sociales y los errores de la app (nv-64: agrupados por huella,
// sin datos personales). Va aparte porque buildModules está al límite de gocyclo.
func buildExtras(d socialDeps, db *mongodriver.Database, cursor *pagination.Signer, log *slog.Logger) ([]httpapi.Module, error) {
	social, err := buildSocial(d, log)
	if err != nil {
		return nil, err
	}
	appErrors, err := errores.NewService(errores.NewRepository(db), cursor)
	if err != nil {
		return nil, fmt.Errorf("errores: %w", err)
	}
	return append([]httpapi.Module{errores.NewHandler(appErrors, d.validator)}, social...), nil
}

// buildSocial arma la búsqueda de personas (nv-35), las sugerencias y la señal social del
// grafo (nv-53) y amistades (nv-36). Sugerencias va antes que amistades: su grupo
// /amistades ya pide JWT y la ruta no debe pasar dos veces por él.
func buildSocial(d socialDeps, log *slog.Logger) ([]httpapi.Module, error) {
	friends, err := amistades.NewService(d.friends, d.users, log)
	if err != nil {
		return nil, fmt.Errorf("amistades: %w", err)
	}
	friends.WithGraph(d.graph.queue).WithNotifier(d.notifier)
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
func startVotes(ctx context.Context, db *mongodriver.Database, queue *grafo.Queue, fcm config.FCM, dataKeys string, log *slog.Logger) func() {
	planRepo := planes.NewRepository(db)
	places := negocios.NewRepository(db)
	plans := planes.NewService(planRepo, planes.NewCalculadora(places, planes.DefaultVelocidades())).WithGraph(queue).WithSync(sincronizacion.NewRepository(db))
	notifRepo := notificaciones.NewRepository(db)
	notifier := notificaciones.NewService(notifRepo, nil, log).WithPush(notificaciones.PusherFromConfig(fcm, usuarios.NewRepository(db).WithCipher(fieldCipher(dataKeys, log)), notifRepo, log))
	groupRepo := grupos.NewRepository(db)
	// El resultado también queda en el chat del grupo (nv-57); publicar no usa cursor ni cuentas.
	posts := chat.NewService(chat.NewRepository(db), groupRepo, planRepo, nil, nil, log)
	svc := votaciones.NewService(votaciones.NewRepository(db), groupRepo, planRepo, plans, places, log).WithNotifier(notifier).WithChat(posts)
	closer := votaciones.NewCloser(svc, log)
	done := make(chan struct{})
	go func() {
		closer.Run(ctx)
		close(done)
	}()
	return func() { <-done }
}

// fieldCipher arma el llavero de DATA_KEYS (nv-48). config.Load ya lo validó; sin llaves
// (solo fuera de prod) avisa y los datos sensibles se guardan en claro.
func fieldCipher(keys string, log *slog.Logger) *cifrado.Llavero {
	if keys == "" {
		log.Warn("sin DATA_KEYS: alergias y fecha de nacimiento se guardan en claro", "evento", "cifrado_apagado")
		return nil
	}
	l, err := cifrado.CargarLlavero(keys)
	if err != nil {
		log.Warn("DATA_KEYS inválido: alergias y fecha de nacimiento se guardan en claro", "evento", "cifrado_apagado")
		return nil
	}
	return l
}
