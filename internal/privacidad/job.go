package privacidad

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
	goredis "github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Llave del candado (constitución §6: `lock:{recurso}`, con TTL). Con 2 o más instancias solo
// la que lo toma ejecuta el trabajo; sin Redis no corre ninguna (fail-closed).
const (
	LockKey     = "lock:borrado_nocturno"
	lockTTL     = 30 * time.Minute
	batchSize   = 200
	runTimezone = "America/Mexico_City"
)

// releaseScript borra el candado solo si sigue siendo nuestro.
var releaseScript = goredis.NewScript(`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`)

// DueAccounts encuentra las cuentas cuyo plazo de borrado ya llegó.
type DueAccounts interface {
	DueForDeletion(ctx context.Context, now time.Time, limit int64) ([]bson.ObjectID, error)
}

// LinkSender manda el enlace de descarga de una exportación lista.
type LinkSender interface {
	SendExportLink(ctx context.Context, userID bson.ObjectID) error
}

// Result resume una corrida, sin datos personales (solo conteos).
type Result struct {
	Ran           bool
	Deleted       int
	DeleteFailed  int
	Exported      int
	ExportsFailed int
	Notified      int
	NoticeFailed  int
	// Retained son los registros vencidos que borraron las tareas de retención (nv-55).
	Retained int
}

// Tarea es una tarea de retención que corre con el candado del trabajo nocturno (nv-55: el
// expediente del negocio). Devuelve cuántos registros borró.
type Tarea interface {
	Name() string
	Run(ctx context.Context) (int, error)
}

// Job es el trabajo nocturno: borra las cuentas vencidas y arma las exportaciones pendientes.
type Job struct {
	rdb      *goredis.Client
	accounts DueAccounts
	exports  *Exports
	registry *Registry
	links    LinkSender
	log      *slog.Logger
	tareas   []Tarea
}

// NewJob recibe todo por constructor.
func NewJob(rdb *goredis.Client, accounts DueAccounts, exports *Exports, registry *Registry, links LinkSender, log *slog.Logger) *Job {
	return &Job{rdb: rdb, accounts: accounts, exports: exports, registry: registry, links: links, log: log}
}

// WithTareas agrega tareas de retención; corren después de los borrados de cuentas.
func (j *Job) WithTareas(t ...Tarea) *Job {
	j.tareas = append(j.tareas, t...)
	return j
}

// Start programa el trabajo todos los días a esa hora de America/Mexico_City con gocron. La hora
// debe caer con la instancia encendida: apagada no corre y no se recupera. Devuelve la función
// que lo detiene.
func (j *Job) Start(hour int) (stop func() error, err error) {
	loc, err := time.LoadLocation(runTimezone)
	if err != nil {
		return nil, err
	}
	s, err := gocron.NewScheduler(gocron.WithLocation(loc))
	if err != nil {
		return nil, err
	}
	_, err = s.NewJob(
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(uint(hour), 0, 0))),
		gocron.NewTask(func() { _, _ = j.RunOnce(context.Background()) }),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return nil, err
	}
	s.Start()
	return s.Shutdown, nil
}

// RunOnce corre el trabajo si toma el candado; Ran=false si otra instancia lo tiene o si Redis
// no responde.
func (j *Job) RunOnce(ctx context.Context) (Result, error) {
	token, ok, err := j.acquire(ctx)
	if err != nil {
		j.log.WarnContext(ctx, "trabajo nocturno sin candado: Redis no responde", "error", err.Error())
		return Result{}, err
	}
	if !ok {
		j.log.InfoContext(ctx, "trabajo nocturno: otra instancia tiene el candado")
		return Result{}, nil
	}
	defer j.release(token)

	res := Result{Ran: true}
	now := time.Now().UTC()
	j.deleteDue(ctx, now, &res)
	j.runTareas(ctx, &res)
	j.buildExports(ctx, now, &res)
	j.sendNotices(ctx, now, &res)
	j.log.InfoContext(ctx, "trabajo nocturno terminado",
		"cuentas_borradas", res.Deleted, "borrados_fallidos", res.DeleteFailed,
		"exportaciones", res.Exported, "exportaciones_fallidas", res.ExportsFailed,
		"avisos", res.Notified, "avisos_fallidos", res.NoticeFailed, "retenidos_borrados", res.Retained)
	return res, nil
}

func (j *Job) runTareas(ctx context.Context, res *Result) {
	for _, t := range j.tareas {
		n, err := t.Run(ctx)
		res.Retained += n
		if err != nil {
			// Lo que falló sigue vencido y se reintenta la noche siguiente.
			j.log.ErrorContext(ctx, "trabajo nocturno: retención incompleta", "tarea", t.Name(), "error", err.Error())
		}
	}
}

func (j *Job) deleteDue(ctx context.Context, now time.Time, res *Result) {
	ids, err := j.accounts.DueForDeletion(ctx, now, batchSize)
	if err != nil {
		j.log.ErrorContext(ctx, "trabajo nocturno: no se pudieron listar las cuentas vencidas", "error", err.Error())
		return
	}
	for _, id := range ids {
		if err := j.registry.PurgeAccount(ctx, id); err != nil {
			// La cuenta sigue pendiente y se reintenta la noche siguiente.
			res.DeleteFailed++
			j.log.ErrorContext(ctx, "trabajo nocturno: borrado incompleto", "error", err.Error())
			continue
		}
		res.Deleted++
	}
}

func (j *Job) buildExports(ctx context.Context, now time.Time, res *Result) {
	pending, err := j.exports.Pending(ctx, batchSize)
	if err != nil {
		j.log.ErrorContext(ctx, "trabajo nocturno: no se pudieron listar las exportaciones", "error", err.Error())
		return
	}
	for _, exp := range pending {
		if err := j.buildExport(ctx, exp, now); err != nil {
			res.ExportsFailed++
			j.log.ErrorContext(ctx, "trabajo nocturno: exportación fallida", "error", err.Error())
			continue
		}
		res.Exported++
	}
}

func (j *Job) buildExport(ctx context.Context, exp Export, now time.Time) error {
	data, err := j.registry.Export(ctx, exp.UserID)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return j.exports.MarkReady(ctx, exp.ID, string(raw), now)
}

// sendNotices manda el aviso de las exportaciones listas que aún no lo tienen: las recién
// armadas y las que fallaron en corridas anteriores, mientras no venzan (72 h). El envío es
// síncrono (con sus reintentos) y cada intento crea un enlace nuevo que invalida el anterior.
func (j *Job) sendNotices(ctx context.Context, now time.Time, res *Result) {
	waiting, err := j.exports.AwaitingNotice(ctx, now, batchSize)
	if err != nil {
		j.log.ErrorContext(ctx, "trabajo nocturno: no se pudieron listar los avisos pendientes", "error", err.Error())
		return
	}
	for _, exp := range waiting {
		if err := j.links.SendExportLink(ctx, exp.UserID); err != nil {
			// Queda sin aviso_enviado_en: la siguiente corrida lo reintenta si no venció.
			res.NoticeFailed++
			continue
		}
		if err := j.exports.MarkNotified(ctx, exp, time.Now().UTC()); err != nil {
			res.NoticeFailed++
			j.log.ErrorContext(ctx, "trabajo nocturno: no se pudo anotar el aviso", "error", err.Error())
			continue
		}
		res.Notified++
	}
}

func (j *Job) acquire(ctx context.Context) (string, bool, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", false, err
	}
	token := hex.EncodeToString(buf)
	ok, err := j.rdb.SetNX(ctx, LockKey, token, lockTTL).Result()
	return token, ok, err
}

func (j *Job) release(token string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := releaseScript.Run(ctx, j.rdb, []string{LockKey}, token).Err(); err != nil {
		j.log.WarnContext(ctx, "trabajo nocturno: no se pudo soltar el candado; vence solo", "error", err.Error())
	}
}
