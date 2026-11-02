package testutil

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
)

// claims arma claims válidos con el emisor y la audiencia de prueba.
func claims(sub, jti string, roles []string, expiresIn time.Duration) middleware.Claims {
	now := time.Now()
	return middleware.Claims{
		Roles:     roles,
		SessionID: "sid-" + jti,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			ID:        jti,
			Issuer:    Issuer,
			Audience:  jwt.ClaimStrings{Audience},
			IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiresIn)),
		},
	}
}

func sign(t *testing.T, c middleware.Claims) string {
	t.Helper()
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(SigningKey))
	if err != nil {
		t.Fatalf("firmar token: %v", err)
	}
	return signed
}

// ValidToken firma un access token HS256 vigente (15 min).
func ValidToken(t *testing.T, jti string, roles ...string) string {
	return sign(t, claims("usuario-1", jti, roles, 15*time.Minute))
}

// ExpiredToken firma un token que venció hace un minuto.
func ExpiredToken(t *testing.T) string {
	return sign(t, claims("usuario-1", "jti-vencido", nil, -time.Minute))
}

// UnsignedToken quita la firma a un token HS256 válido (`header.payload.`).
func UnsignedToken(t *testing.T) string {
	parts := strings.Split(ValidToken(t, "jti-sin-firma"), ".")
	return parts[0] + "." + parts[1] + "."
}

// AlgNoneToken arma un token con `alg: none`.
func AlgNoneToken(t *testing.T) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	parts := strings.Split(ValidToken(t, "jti-alg-none"), ".")
	return header + "." + parts[1] + "."
}
