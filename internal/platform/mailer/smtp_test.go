package mailer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestRender_ThreeTemplatesInSpanish(t *testing.T) {
	for kind, want := range map[Kind]string{
		KindVerifyEmail:   "Confirma tu correo",
		KindPasswordReset: "contraseña",
		KindExport:        "copia de tus datos",
	} {
		r, err := Render(Message{Kind: kind, Name: "Ana"}, "https://wherever.test/x#token=abc")
		require.NoError(t, err, kind)
		assert.Contains(t, r.Text, "Hola, Ana:")
		assert.Contains(t, r.Text, want)
		assert.Contains(t, r.HTML, `href="https://wherever.test/x#token=abc"`)
		assert.NotEmpty(t, r.Subject)
	}
}

func TestRender_WithoutNameAndWithHTMLInTheName(t *testing.T) {
	plain, err := Render(Message{Kind: KindVerifyEmail}, "https://wherever.test/v#token=t")
	require.NoError(t, err)
	assert.Contains(t, plain.Text, "Hola:")

	evil, err := Render(Message{Kind: KindVerifyEmail, Name: `<script>alert("x")</script>`}, "https://wherever.test/v#token=t")
	require.NoError(t, err)
	assert.NotContains(t, evil.HTML, "<script>")
	assert.Contains(t, evil.HTML, "&lt;script&gt;")
}

func TestLinks_TokenGoesInTheFragment(t *testing.T) {
	links := NewLinks("https://wherever.test/app/")
	for kind, path := range map[Kind]string{KindVerifyEmail: "/verificar", KindPasswordReset: "/restablecer", KindExport: "/exportacion"} {
		u, err := links.URL(kind, "abc-_123")
		require.NoError(t, err)
		assert.Equal(t, "https://wherever.test/app"+path+"#token=abc-_123", u)
		assert.NotContains(t, u, "?")
	}
	_, err := links.URL("otro", "x")
	assert.Error(t, err)
}

func TestBuildMIME_RejectsHeaderInjection(t *testing.T) {
	r := Rendered{Subject: "Hola", Text: "t", HTML: "<p>h</p>"}
	for name, to := range map[string]string{"CRLF": "a@x.test\r\nBcc: b@x.test", "LF": "a@x.test\nBcc: b@x.test"} {
		_, err := BuildMIME("no-responder@wherever.test", to, r)
		assert.ErrorIs(t, err, ErrHeaderInjection, name)
	}
	_, err := BuildMIME("no-responder@wherever.test", "a@x.test", Rendered{Subject: "Hola\r\nBcc: b@x.test"})
	assert.ErrorIs(t, err, ErrHeaderInjection)

	raw, err := BuildMIME("no-responder@wherever.test", "a@x.test", r)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "multipart/alternative")
	assert.Contains(t, string(raw), "text/plain; charset=utf-8")
	assert.Contains(t, string(raw), "text/html; charset=utf-8")
}

func TestAuthFor_OnlyWithUser(t *testing.T) {
	assert.Nil(t, authFor(SMTPConfig{Host: "127.0.0.1"}), "Mailpit no autentica")
	assert.NotNil(t, authFor(SMTPConfig{Host: "smtp.test", User: "u", Password: "p"}))
}

// fake es el servidor de correo de las pruebas de cola: una interfaz externa, no una base.
type fake struct {
	mu    sync.Mutex
	sent  []Message
	fails int
	delay time.Duration
	calls atomic.Int32
}

func (f *fake) Send(ctx context.Context, msg Message) error {
	f.calls.Add(1)
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fails > 0 {
		f.fails--
		return errors.New("caído")
	}
	f.sent = append(f.sent, msg)
	return nil
}

func TestRetrying_SecondAttemptAfterFailure(t *testing.T) {
	f := &fake{fails: 1}
	r := NewRetrying(f, DefaultAttempts, time.Millisecond, time.Second)

	require.NoError(t, r.Send(context.Background(), Message{Kind: KindVerifyEmail}))

	assert.Equal(t, int32(2), f.calls.Load())
	assert.Len(t, f.sent, 1)
}

func TestRetrying_RespectsTimeoutPerAttempt(t *testing.T) {
	f := &fake{delay: time.Second}
	r := NewRetrying(f, 2, time.Millisecond, 50*time.Millisecond)

	start := time.Now()
	err := r.Send(context.Background(), Message{Kind: KindVerifyEmail})

	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 500*time.Millisecond)
}

func TestAsync_FullQueueDropsWithoutBlocking(t *testing.T) {
	f := &fake{delay: time.Second}
	a := NewAsync(f, 1, 1, quiet)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = a.Shutdown(ctx)
	})

	start := time.Now()
	for range 5 {
		require.NoError(t, a.Send(context.Background(), Message{Kind: KindVerifyEmail}))
	}

	assert.Less(t, time.Since(start), 100*time.Millisecond, "encolar nunca espera al servidor")
	assert.GreaterOrEqual(t, a.Dropped(), int64(3))
}

func TestAsync_ShutdownDrainsTheQueue(t *testing.T) {
	f := &fake{}
	a := NewAsync(f, 2, 10, quiet)
	for range 5 {
		require.NoError(t, a.Send(context.Background(), Message{Kind: KindPasswordReset}))
	}

	require.NoError(t, a.Shutdown(context.Background()))

	assert.Len(t, f.sent, 5)
	assert.ErrorIs(t, a.Send(context.Background(), Message{}), ErrQueueClosed)
}

func TestReason_NeverIncludesServerText(t *testing.T) {
	assert.Equal(t, "tiempo agotado", reason(context.DeadlineExceeded))
	assert.Equal(t, "rechazado", reason(errors.New("550 5.1.1 <ana@correo.test> usuario desconocido")))
	assert.False(t, strings.Contains(reason(errors.New("ana@correo.test")), "@"))
}
