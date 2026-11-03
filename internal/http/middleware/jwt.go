package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	goredis "github.com/redis/go-redis/v9"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

const (
	bearerPrefix   = "Bearer "
	localClaims    = "claims"
	revokedKeyBase = "jwt_revocado:"
)

// Claims son los datos del access token (constitución §6: sub, jti, roles, sid).
type Claims struct {
	Roles     []string `json:"roles"`
	SessionID string   `json:"sid"`
	jwt.RegisteredClaims
}

// JWTConfig fija clave, emisor y audiencia. La emisión vive en el módulo auth.
type JWTConfig struct {
	Key      []byte
	Issuer   string
	Audience string
}

// AccessParser valida un access token: solo HS256, iss/aud, `exp` e `iat` obligatorios,
// leeway 0, y `sub`/`jti` presentes. Lo comparten el middleware y el módulo auth.
type AccessParser struct {
	parser *jwt.Parser
	key    []byte
}

// NewAccessParser arma el validador con la clave, el emisor y la audiencia dados.
func NewAccessParser(cfg JWTConfig) *AccessParser {
	return &AccessParser{
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithIssuer(cfg.Issuer),
			jwt.WithAudience(cfg.Audience),
			jwt.WithExpirationRequired(),
			jwt.WithIssuedAt(),
			jwt.WithLeeway(0),
		),
		key: cfg.Key,
	}
}

// Parse devuelve los claims de un token válido; cualquier falla es apperr.Unauthorized.
func (p *AccessParser) Parse(raw string) (*Claims, error) {
	claims := &Claims{}
	keyFunc := func(*jwt.Token) (any, error) { return p.key, nil }
	if _, err := p.parser.ParseWithClaims(raw, claims, keyFunc); err != nil || claims.Subject == "" || claims.ID == "" {
		return nil, apperr.Unauthorized("credenciales inválidas")
	}
	return claims, nil
}

// JWT valida el Bearer si viene. Sin encabezado la petición sigue como anónima y las
// rutas protegidas lo exigen con RequireAuth o RequireRoles. Un token presente e inválido
// siempre es 401. La revocación `jwt_revocado:{jti}` se consulta en Redis y es fail-closed (503).
func JWT(cfg JWTConfig, rdb *goredis.Client) fiber.Handler {
	parser := NewAccessParser(cfg)
	return func(c *fiber.Ctx) error {
		header := c.Get(fiber.HeaderAuthorization)
		if header == "" {
			return c.Next()
		}
		raw, ok := strings.CutPrefix(header, bearerPrefix)
		if !ok || raw == "" {
			return apperr.Unauthorized("credenciales inválidas")
		}
		claims, err := parser.Parse(raw)
		if err != nil {
			return err
		}
		revoked, err := rdb.Exists(c.UserContext(), revokedKeyBase+claims.ID).Result()
		if err != nil {
			return apperr.Unavailable("no se pudo verificar la sesión").WithDetail(err.Error())
		}
		if revoked > 0 {
			return apperr.Unauthorized("sesión revocada")
		}
		c.Locals(localClaims, claims)
		return c.Next()
	}
}

// ClaimsFrom devuelve los claims del token validado, si hay.
func ClaimsFrom(c *fiber.Ctx) (*Claims, bool) {
	claims, ok := c.Locals(localClaims).(*Claims)
	return claims, ok
}

// RevokedKey arma la llave de Redis de un jti revocado.
func RevokedKey(jti string) string { return revokedKeyBase + jti }
