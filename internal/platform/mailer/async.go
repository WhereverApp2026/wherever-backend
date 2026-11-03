package mailer

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Reintentos de un envío (spec nv-06): hasta 2 intentos con 1 s de espera.
const (
	DefaultAttempts  = 2
	DefaultRetryWait = time.Second
)

// ErrQueueClosed indica que la API se está apagando.
var ErrQueueClosed = errors.New("cola de correo cerrada")

// Retrying envía con plazo por intento y reintenta tras una falla. Lo usan los trabajadores de
// la cola y, directo (síncrono), el trabajo nocturno, que necesita saber si el aviso salió.
type Retrying struct {
	inner    Mailer
	attempts int
	wait     time.Duration
	timeout  time.Duration
}

// NewRetrying recibe el mailer real, los intentos, la espera entre ellos y el plazo de cada uno.
func NewRetrying(inner Mailer, attempts int, wait, timeout time.Duration) *Retrying {
	return &Retrying{inner: inner, attempts: max(1, attempts), wait: wait, timeout: timeout}
}

// Send intenta hasta `attempts` veces; devuelve el último error.
func (r *Retrying) Send(ctx context.Context, msg Message) error {
	var err error
	for i := range r.attempts {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(r.wait):
			}
		}
		attemptCtx, cancel := context.WithTimeout(ctx, r.timeout)
		err = r.inner.Send(attemptCtx, msg)
		cancel()
		if err == nil || errors.Is(err, ErrHeaderInjection) {
			return err
		}
	}
	return err
}

// Async es la cola en segundo plano: Send nunca bloquea la petición. Con la cola llena el
// correo se descarta y se cuenta (sin contenido). No es estado global: vive en main.
type Async struct {
	inner   Mailer
	queue   chan Message
	log     *slog.Logger
	wg      sync.WaitGroup
	mu      sync.RWMutex
	closed  bool
	dropped atomic.Int64
}

// NewAsync arranca `workers` trabajadores sobre una cola de `size` mensajes.
func NewAsync(inner Mailer, workers, size int, log *slog.Logger) *Async {
	a := &Async{inner: inner, queue: make(chan Message, size), log: log}
	for range max(1, workers) {
		a.wg.Add(1)
		go a.work()
	}
	return a
}

// Send encola y vuelve de inmediato; nunca devuelve error por el servidor de correo.
func (a *Async) Send(ctx context.Context, msg Message) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.closed {
		return ErrQueueClosed
	}
	select {
	case a.queue <- msg:
	default:
		a.dropped.Add(1)
		a.log.WarnContext(ctx, "cola de correo llena: envío descartado", "tipo", string(msg.Kind), "descartados", a.dropped.Load())
	}
	return nil
}

// Dropped dice cuántos envíos se descartaron por cola llena.
func (a *Async) Dropped() int64 { return a.dropped.Load() }

func (a *Async) work() {
	defer a.wg.Done()
	for msg := range a.queue {
		// El resultado ya lo registra el mailer real; aquí no hay a quién devolverlo.
		_ = a.inner.Send(context.Background(), msg)
	}
}

// Shutdown deja de aceptar correos y espera a que se vacíe la cola, con el plazo de ctx.
func (a *Async) Shutdown(ctx context.Context) error {
	a.mu.Lock()
	if !a.closed {
		a.closed = true
		close(a.queue)
	}
	a.mu.Unlock()
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		a.log.Warn("apagado: quedaron correos sin enviar en la cola", "pendientes", len(a.queue))
		return ctx.Err()
	}
}
