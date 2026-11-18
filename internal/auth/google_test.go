package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/googletest"
)

// Unit de nv-72: el verificador real (idtoken) contra las llaves locales de googletest, sin red.

func verifier(t *testing.T, e *googletest.Emisor) GoogleVerifier {
	t.Helper()
	v, err := NewGoogleVerifier(context.Background(), googletest.ClientID, e.Cliente())
	require.NoError(t, err)
	return v
}

func TestGoogleVerifier_ValidTokenGivesIdentity(t *testing.T) {
	e := googletest.Nuevo(t)
	token := e.Token(t, googletest.Identidad{Sub: "123", Correo: "luz@wherever.test", Nombre: "Luz", Foto: "https://lh3.googleusercontent.com/a"})

	id, err := verifier(t, e).Verify(context.Background(), token)

	require.NoError(t, err)
	assert.Equal(t, GoogleIdentity{Subject: "123", Email: "luz@wherever.test", Name: "Luz", Picture: "https://lh3.googleusercontent.com/a"}, id)
}

func TestGoogleVerifier_EveryBadTokenIsErrGoogleToken(t *testing.T) {
	e := googletest.Nuevo(t)
	v := verifier(t, e)
	base := googletest.Identidad{Sub: "123", Correo: "luz@wherever.test"}
	cases := map[string]string{
		"firma ajena":   e.TokenAjeno(t, base),
		"aud ajeno":     e.Token(t, googletest.Identidad{Sub: "123", Correo: "luz@wherever.test", Aud: "otra"}),
		"iss ajeno":     e.Token(t, googletest.Identidad{Sub: "123", Correo: "luz@wherever.test", Iss: "https://evil.test"}),
		"vencido":       e.Token(t, googletest.Identidad{Sub: "123", Correo: "luz@wherever.test", Vence: time.Now().Add(-time.Minute)}),
		"sin verificar": e.Token(t, googletest.Identidad{Sub: "123", Correo: "luz@wherever.test", SinVerif: true}),
		"sin sub":       e.Token(t, googletest.Identidad{Correo: "luz@wherever.test"}),
		"sin correo":    e.Token(t, googletest.Identidad{Sub: "123"}),
		"basura":        "no-es-jwt",
	}
	for name, token := range cases {
		_, err := v.Verify(context.Background(), token)
		assert.ErrorIs(t, err, ErrGoogleToken, name)
	}
}

func TestGoogleVerifier_KeysDownIsUnavailable(t *testing.T) {
	e := googletest.Nuevo(t)
	token := e.Token(t, googletest.Identidad{Sub: "123", Correo: "luz@wherever.test"})
	e.Caer(true)

	_, err := verifier(t, e).Verify(context.Background(), token)

	assert.ErrorIs(t, err, ErrGoogleUnavailable)
}

func TestGoogleVerifier_NeedsClientID(t *testing.T) {
	_, err := NewGoogleVerifier(context.Background(), " ", &http.Client{})
	assert.Error(t, err)
}

func TestEmailVerified(t *testing.T) {
	assert.True(t, emailVerified(true))
	assert.True(t, emailVerified("true"))
	assert.False(t, emailVerified(false))
	assert.False(t, emailVerified("false"))
	assert.False(t, emailVerified(nil))
	assert.False(t, emailVerified(1))
}

func TestGoogleName(t *testing.T) {
	assert.Equal(t, "Luz María", googleName("  Luz María ", "luz@wherever.test"))
	assert.Equal(t, strings.Repeat("ñ", 40), googleName(strings.Repeat("ñ", 50), "x@wherever.test"))
	assert.Equal(t, "luz.maria", googleName("L", "luz.maria@wherever.test"), "nombre corto = parte local")
	assert.Equal(t, "Viajero", googleName("", "x@wherever.test"))
}

func TestGooglePhoto(t *testing.T) {
	assert.Equal(t, "https://lh3.googleusercontent.com/a/x", googlePhoto("https://lh3.googleusercontent.com/a/x"))
	assert.Empty(t, googlePhoto("http://lh3.googleusercontent.com/a/x"))
	assert.Empty(t, googlePhoto("javascript:alert(1)"))
	assert.Empty(t, googlePhoto("https://"))
	assert.Empty(t, googlePhoto(""))
}

func TestMissingSignup(t *testing.T) {
	yes := true
	fields := func(in GoogleInput) []string {
		out := []string{}
		for _, f := range missingSignup(in) {
			out = append(out, f.Field)
		}
		return out
	}
	assert.Equal(t, []string{"fecha_nacimiento", "acepta_terminos", "version_terminos"}, fields(GoogleInput{}))
	assert.Equal(t, []string{"acepta_terminos"}, fields(GoogleInput{BirthDate: "1990-01-01", TermsVersion: "2026-10"}))
	assert.Empty(t, fields(GoogleInput{BirthDate: "1990-01-01", AcceptTerms: &yes, TermsVersion: "2026-10"}))
}

func TestGoogleRequest_Validation(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	device := DeviceInput{Model: "Pixel 8", System: "Android 15"}
	ok := GoogleRequest{IDToken: "a.b.c", Device: device}
	require.NoError(t, v.Struct(ok), "sin datos de registro también es válido (el service decide)")

	for name, req := range map[string]GoogleRequest{
		"sin token":       {Device: device},
		"token enorme":    {IDToken: strings.Repeat("a", 4097), Device: device},
		"sin dispositivo": {IDToken: "a.b.c"},
		"menor de edad":   {IDToken: "a.b.c", Device: device, BirthDate: time.Now().UTC().AddDate(-17, 0, 0).Format(validation.DateLayout)},
		"fecha mala":      {IDToken: "a.b.c", Device: device, BirthDate: "14/03/1996"},
		"versión larga":   {IDToken: "a.b.c", Device: device, TermsVersion: strings.Repeat("v", 21)},
	} {
		err := v.Struct(req)
		var appErr *apperr.Error
		require.True(t, errors.As(err, &appErr), name)
		assert.Equal(t, apperr.KindValidation, appErr.Kind, name)
	}
}

func TestGoogleSignIn_WithoutVerifierIs503(t *testing.T) {
	_, _, err := (&Service{}).GoogleSignIn(context.Background(), GoogleInput{IDToken: "a.b.c"})

	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperr.KindUnavailable, appErr.Kind)
}
