package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/api/idtoken"
)

// Errores del verificador; el service los traduce a 401 y 503 sin decir cuál falló.
var (
	// ErrGoogleInvalid: firma, `aud`, `iss`, `exp`, `email_verified` o `sub` no pasan.
	ErrGoogleInvalid = errors.New("id_token de Google inválido")
	// ErrGoogleUnavailable: no se pudieron bajar las llaves públicas de Google.
	ErrGoogleUnavailable = errors.New("no se pudieron obtener las llaves de Google")
)

// googleIssuers son los `iss` que Google usa en sus id_token.
var googleIssuers = map[string]bool{"accounts.google.com": true, "https://accounts.google.com": true}

// googleKeysTimeout acota la descarga de las llaves: idtoken no le pasa plazo a la petición.
const googleKeysTimeout = 5 * time.Second

// GoogleIdentity es lo que la app usa del id_token ya validado. Nunca va a los logs.
type GoogleIdentity struct {
	Subject string
	Email   string
	Name    string
	Picture string
}

// GoogleVerifier valida un id_token de Google. Es la única frontera externa del módulo: las
// pruebas usan la implementación real con llaves locales (internal/testutil/googletest).
type GoogleVerifier interface {
	Verify(ctx context.Context, idToken string) (GoogleIdentity, error)
}

type idtokenVerifier struct {
	validator *idtoken.Validator
	clientID  string
}

// NewGoogleVerifier envuelve a google.golang.org/api/idtoken, que valida firma (llaves en
// caché según el Cache-Control de Google), `aud` y `exp`; aquí se revisan `iss`, `sub` y
// `email_verified`. client nil = cliente por defecto con plazo de 5 s.
func NewGoogleVerifier(ctx context.Context, clientID string, client *http.Client) (GoogleVerifier, error) {
	if strings.TrimSpace(clientID) == "" {
		return nil, errors.New("google: falta el client id")
	}
	if client == nil {
		client = &http.Client{Timeout: googleKeysTimeout}
	}
	wrapped := *client
	base := wrapped.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	wrapped.Transport = keysTransport{base: base}
	v, err := idtoken.NewValidator(ctx, idtoken.WithHTTPClient(&wrapped))
	if err != nil {
		return nil, fmt.Errorf("google: %w", err)
	}
	return &idtokenVerifier{validator: v, clientID: clientID}, nil
}

func (v *idtokenVerifier) Verify(ctx context.Context, idToken string) (GoogleIdentity, error) {
	payload, err := v.validator.Validate(ctx, idToken, v.clientID)
	if err != nil {
		var netErr *url.Error
		if errors.As(err, &netErr) || errors.Is(err, ErrGoogleUnavailable) {
			return GoogleIdentity{}, ErrGoogleUnavailable
		}
		return GoogleIdentity{}, ErrGoogleInvalid
	}
	return identityFromClaims(payload.Issuer, payload.Subject, payload.Claims)
}

// identityFromClaims revisa lo que idtoken no revisa y arma la identidad.
func identityFromClaims(issuer, subject string, claims map[string]any) (GoogleIdentity, error) {
	email, _ := claims["email"].(string)
	if !googleIssuers[issuer] || subject == "" || email == "" || !emailVerified(claims["email_verified"]) {
		return GoogleIdentity{}, ErrGoogleInvalid
	}
	name, _ := claims["name"].(string)
	picture, _ := claims["picture"].(string)
	return GoogleIdentity{Subject: subject, Email: email, Name: name, Picture: picture}, nil
}

// emailVerified acepta el booleano y el texto "true" (tokens viejos de Google).
func emailVerified(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true"
	}
	return false
}

// keysTransport marca como ErrGoogleUnavailable cualquier respuesta que no sea 200 al bajar
// las llaves; así se distingue «Google caído» (503) de «token malo» (401).
type keysTransport struct{ base http.RoundTripper }

func (t keysTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGoogleUnavailable, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: estado %d", ErrGoogleUnavailable, resp.StatusCode)
	}
	return resp, nil
}

// Límites del nombre de la cuenta (hoja A1: 2 a 40 caracteres).
const (
	minNameLen = 2
	maxNameLen = 40
)

// googleName toma el `name` de Google recortado a 40 caracteres; si queda con menos de 2, usa
// la parte local del correo (también recortada).
func googleName(name, email string) string {
	name = truncateRunes(strings.TrimSpace(name), maxNameLen)
	if utf8.RuneCountInString(name) >= minNameLen {
		return name
	}
	local, _, _ := strings.Cut(email, "@")
	local = truncateRunes(local, maxNameLen)
	if utf8.RuneCountInString(local) >= minNameLen {
		return local
	}
	return "Viajero"
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n]))
}

// googlePhoto solo acepta `picture` con https:// y host; lo demás se descarta.
func googlePhoto(picture string) string {
	u, err := url.Parse(picture)
	if err != nil || u.Scheme != "https" || u.Host == "" || len(picture) > 2048 {
		return ""
	}
	return picture
}
