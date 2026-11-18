// Package googletest es el doble de Google para POST /auth/google (nv-72): un par de llaves
// RSA local y un servidor que sirve sus llaves públicas como lo hace Google. El cliente HTTP
// que entrega redirige ahí cualquier petición, así que el verificador real (idtoken) corre
// completo sin salir a la red.
package googletest

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// ClientID es la audiencia de prueba (el cliente OAuth Web de Google); no es de ningún entorno.
const ClientID = "wherever-pruebas.apps.googleusercontent.com"

// Issuer es el `iss` de los id_token de Google.
const Issuer = "https://accounts.google.com"

const keyID = "llave-pruebas"

// Emisor firma id_token como Google y sirve sus llaves públicas.
type Emisor struct {
	key  *rsa.PrivateKey
	srv  *httptest.Server
	down atomic.Bool
}

// Nuevo levanta el servidor de llaves; se apaga al terminar la prueba.
func Nuevo(t testing.TB) *Emisor {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	e := &Emisor{key: key}
	e.srv = httptest.NewServer(http.HandlerFunc(e.serveKeys))
	t.Cleanup(e.srv.Close)
	return e
}

func (e *Emisor) serveKeys(w http.ResponseWriter, _ *http.Request) {
	if e.down.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	// Sin Cache-Control: idtoken no guarda las llaves y cada prueba ve el estado actual.
	pub := e.key.PublicKey
	enc := base64.RawURLEncoding
	_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "alg": "RS256", "use": "sig", "kid": keyID,
		"n": enc.EncodeToString(pub.N.Bytes()), "e": enc.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}}})
}

// Caer simula a Google sin responder las llaves (503 al bajarlas).
func (e *Emisor) Caer(caido bool) { e.down.Store(caido) }

// Cliente es el cliente HTTP para auth.NewGoogleVerifier: toda petición va al servidor local.
func (e *Emisor) Cliente() *http.Client {
	target, _ := url.Parse(e.srv.URL)
	return &http.Client{Timeout: 5 * time.Second, Transport: redirect{target: target}}
}

type redirect struct{ target *url.URL }

func (r redirect) RoundTrip(req *http.Request) (*http.Response, error) {
	out := req.Clone(req.Context())
	out.URL.Scheme, out.URL.Host, out.Host = r.target.Scheme, r.target.Host, r.target.Host
	return http.DefaultTransport.RoundTrip(out)
}

// Identidad son los claims de un id_token de Google.
type Identidad struct {
	Sub      string
	Correo   string
	Nombre   string
	Foto     string
	Aud      string // vacío = ClientID
	Iss      string // vacío = Issuer
	Vence    time.Time
	SinVerif bool // email_verified = false
}

// Token firma un id_token con la llave del emisor.
func (e *Emisor) Token(t testing.TB, id Identidad) string {
	t.Helper()
	return firmar(t, e.key, id)
}

// TokenAjeno firma con otra llave (mismo kid): la firma no corresponde a las llaves publicadas.
func (e *Emisor) TokenAjeno(t testing.TB, id Identidad) string {
	t.Helper()
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return firmar(t, other, id)
}

func firmar(t testing.TB, key *rsa.PrivateKey, id Identidad) string {
	t.Helper()
	now := time.Now()
	if id.Aud == "" {
		id.Aud = ClientID
	}
	if id.Iss == "" {
		id.Iss = Issuer
	}
	if id.Vence.IsZero() {
		id.Vence = now.Add(time.Hour)
	}
	claims := jwt.MapClaims{
		"iss": id.Iss, "aud": id.Aud, "sub": id.Sub, "email": id.Correo, "email_verified": !id.SinVerif,
		"iat": now.Unix(), "exp": id.Vence.Unix(),
	}
	if id.Nombre != "" {
		claims["name"] = id.Nombre
	}
	if id.Foto != "" {
		claims["picture"] = id.Foto
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = keyID
	raw, err := tok.SignedString(key)
	require.NoError(t, err)
	return raw
}
