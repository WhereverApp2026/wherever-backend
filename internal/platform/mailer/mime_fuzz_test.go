package mailer

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var allowedHeaders = []string{"From: ", "To: ", "Subject: ", "Date: ", "MIME-Version: ", "Content-Type: "}

func knownHeader(line string) bool {
	if strings.HasPrefix(line, " ") { // continuación de un asunto largo
		return true
	}
	for _, h := range allowedHeaders {
		if strings.HasPrefix(line, h) {
			return true
		}
	}
	return false
}

// FuzzBuildMIME: nada de lo que llega en remitente, destinatario o asunto crea otro encabezado.
func FuzzBuildMIME(f *testing.F) {
	f.Add("no@ejemplo.com", "ana@ejemplo.com", "Confirma tu correo", "texto", "<p>html</p>")
	f.Add("no@ejemplo.com", "ana@ejemplo.com\r\nBcc: x@y.z", "a", "", "")
	f.Add("no@ejemplo.com", "a@b.c", "asunto\r\nBcc: x@y.z", "", "")
	f.Add("a\nb", "c", "d", "", "")
	f.Add("", "", strings.Repeat("ñ", 500), "\xff", "\x00")
	f.Fuzz(func(t *testing.T, from, to, subject, text, html string) {
		raw, err := BuildMIME(from, to, Rendered{Subject: subject, Text: text, HTML: html})
		if strings.ContainsAny(from+to+subject, "\r\n") {
			assert.ErrorIs(t, err, ErrHeaderInjection)
			return
		}
		require.NoError(t, err)
		head, _, found := strings.Cut(string(raw), "\r\n\r\n")
		require.True(t, found)
		for _, line := range strings.Split(head, "\r\n") {
			assert.True(t, knownHeader(line), "encabezado inesperado: %q", line)
		}
	})
}

// FuzzLinkToken: el token va siempre en el fragmento y no cambia el destino del enlace.
func FuzzLinkToken(f *testing.F) {
	for _, seed := range []string{"abc", "", "a#b", "a?b=c", "../../x", "a b", "\xff", "@otro.com", "%00"} {
		f.Add(seed)
	}
	links := NewLinks("https://app.wherever.test")
	f.Fuzz(func(t *testing.T, token string) {
		raw, err := links.URL(KindPasswordReset, token)
		require.NoError(t, err)
		u, err := url.Parse(raw)
		require.NoError(t, err)
		assert.Equal(t, "app.wherever.test", u.Host)
		assert.Equal(t, "/restablecer", u.Path)
		assert.Empty(t, u.RawQuery, "el token nunca va en la consulta")
		_, value, found := strings.Cut(raw, "#token=")
		require.True(t, found)
		got, err := url.QueryUnescape(value)
		require.NoError(t, err)
		assert.Equal(t, token, got)
	})
}
