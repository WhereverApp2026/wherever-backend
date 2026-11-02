package middleware

import (
	"fmt"
	"slices"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Roles de la constitución §3. "Dueño" no es rol: se valida en el service.
const (
	RoleConsumer = "consumidor"
	RoleMerchant = "comerciante"
	RoleAdmin    = "administrador"
)

var knownRoles = []string{RoleConsumer, RoleMerchant, RoleAdmin}

// RequireRoles exige token válido (401) y al menos uno de los roles (403).
// Un rol desconocido es un error de programación y se detecta al construir las rutas.
func RequireRoles(roles ...string) fiber.Handler {
	for _, role := range roles {
		if !slices.Contains(knownRoles, role) {
			panic(fmt.Sprintf("rol desconocido: %q", role))
		}
	}
	return func(c *fiber.Ctx) error {
		claims, ok := ClaimsFrom(c)
		if !ok {
			return apperr.Unauthorized("autenticación requerida")
		}
		for _, role := range roles {
			if slices.Contains(claims.Roles, role) {
				return c.Next()
			}
		}
		return apperr.Forbidden("sin permiso para este recurso")
	}
}
