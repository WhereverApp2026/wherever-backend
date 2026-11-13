// Package push envía avisos al teléfono con Firebase Cloud Messaging (API HTTP v1, nv-41).
// Usa la cuenta de servicio de Firebase: firma un JWT (RS256) con su llave, lo cambia por un
// token de acceso de Google y lo guarda en memoria hasta un minuto antes de que venza. Sin
// librerías nuevas: golang-jwt ya está en el proyecto.
package push

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Alcance de FCM y adelanto con el que se renueva el token de acceso.
const (
	scope        = "https://www.googleapis.com/auth/firebase.messaging"
	renewBefore  = time.Minute
	assertionTTL = time.Hour
	// DefaultBaseURL es FCM; las pruebas usan un servidor local.
	DefaultBaseURL = "https://fcm.googleapis.com"
)

// Errores de envío. ErrUnregistered = el token ya no sirve (se borra); ErrRetryable = vale la
// pena reintentar (5xx, 429, red).
var (
	ErrUnregistered = errors.New("push: token no registrado")
	ErrRetryable    = errors.New("push: fallo temporal")
	ErrRejected     = errors.New("push: rechazado")
)

// Credentials es lo que se usa de la cuenta de servicio de Firebase.
type Credentials struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// ParseCredentials lee el JSON de la cuenta de servicio en base64 (una línea, para
// Parameter Store y `.env`) y valida la llave.
func ParseCredentials(b64 string) (Credentials, *rsa.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return Credentials{}, nil, errors.New("push: las credenciales no son base64")
	}
	var c Credentials
	if err := json.Unmarshal(raw, &c); err != nil {
		return Credentials{}, nil, errors.New("push: las credenciales no son el JSON de una cuenta de servicio")
	}
	if c.ClientEmail == "" || c.PrivateKey == "" || c.TokenURI == "" {
		return Credentials{}, nil, errors.New("push: faltan client_email, private_key o token_uri")
	}
	if u, err := url.Parse(c.TokenURI); err != nil || (u.Scheme != "https" && u.Hostname() != "127.0.0.1") {
		return Credentials{}, nil, errors.New("push: token_uri debe ser https")
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(c.PrivateKey))
	if err != nil {
		return Credentials{}, nil, errors.New("push: la llave privada no es válida")
	}
	return c, key, nil
}

// Message es lo que se manda: texto genérico y datos con ids. Nunca contenido sensible.
type Message struct {
	Title string
	Body  string
	Data  map[string]string
}

// FCM manda mensajes a un proyecto. Es seguro para varias goroutines.
type FCM struct {
	project string
	base    string
	creds   Credentials
	key     *rsa.PrivateKey
	http    *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
	now     func() time.Time
}

// NewFCM arma el cliente con las credenciales ya validadas.
func NewFCM(project, base string, creds Credentials, key *rsa.PrivateKey, timeout time.Duration) *FCM {
	if base == "" {
		base = DefaultBaseURL
	}
	return &FCM{project: project, base: strings.TrimRight(base, "/"), creds: creds, key: key,
		http: &http.Client{Timeout: timeout}, now: time.Now}
}

// Reset olvida el token de acceso guardado (pruebas y rotación de llave).
func (f *FCM) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.token, f.expires = "", time.Time{}
}

// accessToken devuelve el token de acceso vigente o pide otro.
func (f *FCM) accessToken(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.now()
	if f.token != "" && now.Before(f.expires.Add(-renewBefore)) {
		return f.token, nil
	}
	assertion, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": f.creds.ClientEmail, "scope": scope, "aud": f.creds.TokenURI,
		"iat": now.Unix(), "exp": now.Add(assertionTTL).Unix(),
	}).SignedString(f.key)
	if err != nil {
		return "", fmt.Errorf("%w: firmar", ErrRejected)
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {assertion}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.creds.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", ErrRejected
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := f.http.Do(req)
	if err != nil {
		return "", ErrRetryable
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", classify(resp.StatusCode, nil)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&out); err != nil || out.AccessToken == "" {
		return "", ErrRetryable
	}
	f.token, f.expires = out.AccessToken, now.Add(time.Duration(out.ExpiresIn)*time.Second)
	return f.token, nil
}

// Send manda un mensaje a un token. Los errores no llevan la respuesta cruda de Google.
func (f *FCM) Send(ctx context.Context, deviceToken string, m Message) error {
	access, err := f.accessToken(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"message": map[string]any{
		"token":        deviceToken,
		"notification": map[string]string{"title": m.Title, "body": m.Body},
		"data":         m.Data,
		"android":      map[string]string{"priority": "high"},
	}})
	if err != nil {
		return ErrRejected
	}
	endpoint := f.base + "/v1/projects/" + url.PathEscape(f.project) + "/messages:send"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ErrRejected
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access)
	resp, err := f.http.Do(req)
	if err != nil {
		return ErrRetryable
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		f.Reset() // el token de acceso venció antes de lo esperado
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	return classify(resp.StatusCode, raw)
}

// classify traduce la respuesta de FCM. UNREGISTERED (404) o un token mal formado
// (INVALID_ARGUMENT con errorCode del token) = el token ya no sirve.
func classify(status int, raw []byte) error {
	switch {
	case status == http.StatusNotFound || bytes.Contains(raw, []byte("UNREGISTERED")):
		return ErrUnregistered
	case status == http.StatusTooManyRequests || status >= 500 || status == http.StatusUnauthorized:
		return ErrRetryable
	default:
		return ErrRejected
	}
}
