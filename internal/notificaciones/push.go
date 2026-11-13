package notificaciones

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/push"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Sender manda un mensaje a un token (lo cumple push.FCM; las pruebas usan un servidor local).
type Sender interface {
	Send(ctx context.Context, deviceToken string, m push.Message) error
}

// Users lee las preferencias del destinatario.
type Users interface {
	FindByID(ctx context.Context, id bson.ObjectID) (usuarios.User, error)
}

// Ajustes del envío en segundo plano (como el correo de nv-06).
const (
	pushWorkers   = 4
	pushQueueSize = 500
	pushAttempts  = 3
	pushBackoff   = time.Second
	pushTimeout   = 15 * time.Second
)

// pushTitle es el título de todos los avisos y pushText el texto de cada tipo: genéricos, sin
// nombres ni datos de nadie (requisito 5).
const pushTitle = "Wherever"

var pushText = map[string]string{
	KindFriendRequest:    "Tienes una nueva solicitud de amistad",
	KindFriendAccepted:   "Aceptaron tu solicitud de amistad",
	KindPlanInvite:       "Te invitaron a un plan",
	KindVoteOpened:       "Hay una votación abierta en tu plan",
	KindVoteClosed:       "Se cerró una votación de tu plan",
	KindBusinessReviewed: "Ya revisamos tu negocio",
}

type pushJob struct {
	to   bson.ObjectID
	kind string
	data map[string]string
}

// Pusher manda push por FCM en segundo plano. Enqueue nunca bloquea: con la cola llena el
// aviso se descarta (queda en el centro) y se cuenta. Un fallo nunca llega a la acción.
type Pusher struct {
	sender  Sender
	users   Users
	repo    *Repository
	log     *slog.Logger
	jobs    chan pushJob
	wg      sync.WaitGroup
	dropped atomic.Int64
	backoff time.Duration
	now     func() time.Time
	once    sync.Once
}

// NewPusher arranca los trabajadores.
func NewPusher(sender Sender, users Users, repo *Repository, log *slog.Logger) *Pusher {
	p := &Pusher{sender: sender, users: users, repo: repo, log: log, jobs: make(chan pushJob, pushQueueSize), backoff: pushBackoff, now: time.Now}
	for range pushWorkers {
		p.wg.Add(1)
		go p.work()
	}
	return p
}

// Enqueue pone el aviso en la cola sin esperar.
func (p *Pusher) Enqueue(to bson.ObjectID, kind string, data map[string]string) {
	select {
	case p.jobs <- pushJob{to: to, kind: kind, data: data}:
	default:
		p.dropped.Add(1)
		p.log.Warn("push: cola llena, el aviso queda solo en el centro", "evento", "push_descartado", "tipo", kind)
	}
}

// Dropped cuenta los avisos descartados por cola llena.
func (p *Pusher) Dropped() int64 { return p.dropped.Load() }

// Shutdown deja de recibir y espera a que se vacíe la cola, hasta que ctx venza.
func (p *Pusher) Shutdown(ctx context.Context) error {
	p.once.Do(func() { close(p.jobs) })
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Pusher) work() {
	defer p.wg.Done()
	for job := range p.jobs {
		ctx, cancel := context.WithTimeout(context.Background(), pushTimeout)
		p.deliver(ctx, job)
		cancel()
	}
}

// deliver respeta las preferencias y el horario silencioso, y manda a cada teléfono.
func (p *Pusher) deliver(ctx context.Context, job pushJob) {
	u, err := p.users.FindByID(ctx, job.to)
	if err != nil || u.Status != usuarios.StatusActive || !Wanted(u.Preferences, job.kind, p.now()) {
		return
	}
	devices, err := p.repo.Devices(ctx, job.to)
	if err != nil {
		p.log.Warn("push: no se leyeron los teléfonos", "evento", "push_fallido", "tipo", job.kind)
		return
	}
	msg := push.Message{Title: pushTitle, Body: pushText[job.kind], Data: withKind(job.data, job.kind)}
	for _, d := range devices {
		p.sendOne(ctx, d, job.kind, msg)
	}
}

func withKind(data map[string]string, kind string) map[string]string {
	out := make(map[string]string, len(data)+1)
	for k, v := range data {
		out[k] = v
	}
	out["tipo"] = kind
	return out
}

// sendOne reintenta los fallos temporales y borra el token que FCM ya no reconoce. El log
// lleva tipo y resultado; nunca el token ni el cuerpo.
func (p *Pusher) sendOne(ctx context.Context, d Device, kind string, msg push.Message) {
	var err error
	for attempt := range pushAttempts {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(p.backoff * time.Duration(attempt)):
			}
		}
		if err = p.sender.Send(ctx, d.Token, msg); !errors.Is(err, push.ErrRetryable) {
			break
		}
	}
	switch {
	case err == nil:
		p.log.Info("push enviado", "evento", "push_enviado", "tipo", kind)
	case errors.Is(err, push.ErrUnregistered):
		_ = p.repo.RemoveDeviceByHash(ctx, d.TokenHash)
		p.log.Info("push: token dado de baja", "evento", "push_token_baja", "tipo", kind)
	default:
		p.log.Warn("push: no se envió", "evento", "push_fallido", "tipo", kind, "resultado", resultOf(err))
	}
}

func resultOf(err error) string {
	if errors.Is(err, push.ErrRetryable) {
		return "temporal"
	}
	return "rechazado"
}

// Kinds que siguen el interruptor «grupos» de las preferencias; los de amistad no tienen
// interruptor propio en nv-05 y siempre se envían (fuera del horario silencioso).
var groupKinds = map[string]bool{KindPlanInvite: true, KindVoteOpened: true, KindVoteClosed: true}

// Wanted dice si el aviso debe sonar: el tipo está encendido y no es horario silencioso (en
// la hora de México, la zona por defecto de la constitución §5).
func Wanted(prefs usuarios.Preferences, kind string, now time.Time) bool {
	if groupKinds[kind] && !prefs.Notifications.Groups {
		return false
	}
	return !inQuietHours(prefs.QuietHours, now.In(negocios.MexicoCity()))
}

// inQuietHours: `desde` incluido, `hasta` excluido; si `desde` > `hasta` cruza la medianoche.
func inQuietHours(q *usuarios.QuietHours, local time.Time) bool {
	if q == nil {
		return false
	}
	from, okFrom := minuteOf(q.From)
	to, okTo := minuteOf(q.To)
	if !okFrom || !okTo || from == to {
		return false
	}
	m := local.Hour()*60 + local.Minute()
	if from < to {
		return m >= from && m < to
	}
	return m >= from || m < to
}

func minuteOf(hhmm string) (int, bool) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

// pushTimeoutHTTP es el plazo de cada llamada a Google.
const pushTimeoutHTTP = 10 * time.Second

// PusherFromConfig arma el push si hay credenciales de Firebase; si no, devuelve nil y los
// avisos quedan solo en el centro. config.Load ya validó las credenciales.
func PusherFromConfig(cfg config.FCM, users Users, repo *Repository, log *slog.Logger) *Pusher {
	if !cfg.Enabled() {
		log.Info("sin FCM_PROJECT_ID ni credenciales: los avisos quedan solo en el centro", "evento", "push_apagado")
		return nil
	}
	creds, key, err := push.ParseCredentials(cfg.CredentialsB64)
	if err != nil {
		log.Warn("push apagado: credenciales inválidas", "evento", "push_apagado")
		return nil
	}
	return NewPusher(push.NewFCM(cfg.ProjectID, cfg.BaseURL, creds, key, pushTimeoutHTTP), users, repo, log)
}
