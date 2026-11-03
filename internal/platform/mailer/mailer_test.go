package mailer

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemory_KeepsLastByRecipientAndKind(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	require.NoError(t, m.Send(ctx, Message{To: "a@x.com", Kind: KindVerifyEmail, Token: "1"}))
	require.NoError(t, m.Send(ctx, Message{To: "a@x.com", Kind: KindVerifyEmail, Token: "2"}))
	require.NoError(t, m.Send(ctx, Message{To: "b@x.com", Kind: KindPasswordReset, Token: "3"}))

	last, ok := m.Last("a@x.com", KindVerifyEmail)
	require.True(t, ok)
	assert.Equal(t, "2", last.Token)
	_, ok = m.Last("a@x.com", KindPasswordReset)
	assert.False(t, ok)
	assert.Len(t, m.Sent(), 3)

	m.Reset()
	assert.Empty(t, m.Sent())
}

func TestLog_NeverWritesTokenOrFullAddress(t *testing.T) {
	var buf bytes.Buffer
	l := NewLog(slog.New(slog.NewJSONHandler(&buf, nil)))

	require.NoError(t, l.Send(context.Background(), Message{To: "ana@correo.com", Kind: KindPasswordReset, Token: "secreto-del-enlace"}))

	out := buf.String()
	assert.Contains(t, out, "a***@correo.com")
	assert.Contains(t, out, string(KindPasswordReset))
	assert.NotContains(t, out, "secreto-del-enlace")
	assert.NotContains(t, out, "ana@correo.com")
}

func TestMask(t *testing.T) {
	assert.Equal(t, "a***@x.com", Mask("ana@x.com"))
	assert.Equal(t, "***", Mask("sin-arroba"))
	assert.Equal(t, "***", Mask("@x.com"))
}
