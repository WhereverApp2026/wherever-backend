package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
)

// opaqueBytes es la entropía del refresh y de los tokens de enlace (hoja J1: 32 bytes).
const opaqueBytes = 32

// AccessToken es un JWT emitido con su jti y vencimiento.
type AccessToken struct {
	Raw       string
	JTI       string
	ExpiresAt time.Time
}

// Issuer firma access tokens HS256 con sub, sid, jti, roles, iss, aud, iat y exp.
type Issuer struct {
	key      []byte
	issuer   string
	audience string
	ttl      time.Duration
}

// NewIssuer recibe la misma configuración que valida el middleware JWT.
func NewIssuer(cfg middleware.JWTConfig, ttl time.Duration) *Issuer {
	return &Issuer{key: cfg.Key, issuer: cfg.Issuer, audience: cfg.Audience, ttl: ttl}
}

// Issue firma un token para el usuario y la sesión dados.
func (i *Issuer) Issue(userID, sessionID string, roles []string, now time.Time) (AccessToken, error) {
	jti, err := randomID()
	if err != nil {
		return AccessToken{}, err
	}
	exp := now.UTC().Add(i.ttl).Truncate(time.Second)
	claims := middleware.Claims{
		Roles:     roles,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        jti,
			Issuer:    i.issuer,
			Audience:  jwt.ClaimStrings{i.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.key)
	if err != nil {
		return AccessToken{}, err
	}
	return AccessToken{Raw: raw, JTI: jti, ExpiresAt: exp}, nil
}

// NewOpaque genera un token aleatorio (base64url) y su hash; en la base solo va el hash.
func NewOpaque() (raw, hash string, err error) {
	buf := make([]byte, opaqueBytes)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashOpaque(raw), nil
}

// HashOpaque es el SHA-256 en hexadecimal. Basta porque el token es aleatorio de alta
// entropía, no una contraseña (constitución §6).
func HashOpaque(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// newFamilyID es un UUID v4 (hoja J1: familia_id string UUID).
func newFamilyID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], nil
}

func randomID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
