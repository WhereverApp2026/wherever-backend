package push_test

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/push"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/fcmtest"
)

func client(t *testing.T) (*push.FCM, *fcmtest.Server) {
	t.Helper()
	srv := fcmtest.New(t)
	creds, key, err := push.ParseCredentials(srv.Credentials)
	require.NoError(t, err)
	return push.NewFCM(fcmtest.Project, srv.URL, creds, key, 5*time.Second), srv
}

func TestFCM_EnviaYGuardaElTokenDeAcceso(t *testing.T) {
	f, srv := client(t)
	msg := push.Message{Title: "Wherever", Body: "Te invitaron a un plan", Data: map[string]string{"tipo": "invitacion_plan"}}
	require.NoError(t, f.Send(context.Background(), "token-a", msg))
	require.NoError(t, f.Send(context.Background(), "token-b", msg))
	sent := srv.Sent()
	require.Len(t, sent, 2)
	assert.Equal(t, "Te invitaron a un plan", sent[0].Body)
	assert.Equal(t, "invitacion_plan", sent[0].Data["tipo"])
	assert.Equal(t, 1, srv.TokenCalls(), "el token de acceso se reutiliza")
}

func TestFCM_ClasificaLosErrores(t *testing.T) {
	f, srv := client(t)
	srv.Unregister("viejo")
	srv.FailTimes("lento", 1)
	assert.ErrorIs(t, f.Send(context.Background(), "viejo", push.Message{}), push.ErrUnregistered)
	assert.ErrorIs(t, f.Send(context.Background(), "lento", push.Message{}), push.ErrRetryable)
	assert.NoError(t, f.Send(context.Background(), "lento", push.Message{}))
}

func TestFCM_SinServidorEsTemporal(t *testing.T) {
	srv := fcmtest.New(t)
	creds, key, err := push.ParseCredentials(srv.Credentials)
	require.NoError(t, err)
	// El intercambio de token responde; el envío va a un puerto sin nadie.
	down := push.NewFCM(fcmtest.Project, "http://127.0.0.1:1", creds, key, time.Second)
	assert.ErrorIs(t, down.Send(context.Background(), "x", push.Message{}), push.ErrRetryable)
}

func TestParseCredentials_Invalidas(t *testing.T) {
	cases := map[string]string{
		"no base64":       "%%%",
		"no json":         base64.StdEncoding.EncodeToString([]byte("hola")),
		"sin llave":       base64.StdEncoding.EncodeToString([]byte(`{"client_email":"a@b.c","token_uri":"https://oauth2.googleapis.com/token"}`)),
		"token_uri http":  base64.StdEncoding.EncodeToString([]byte(`{"client_email":"a@b.c","private_key":"x","token_uri":"http://ejemplo.test/token"}`)),
		"llave no es pem": base64.StdEncoding.EncodeToString([]byte(`{"client_email":"a@b.c","private_key":"x","token_uri":"https://oauth2.googleapis.com/token"}`)),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := push.ParseCredentials(raw)
			assert.Error(t, err)
		})
	}
}
