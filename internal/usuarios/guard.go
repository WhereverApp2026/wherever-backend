package usuarios

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// RequireActive va después de RequireAuth en las rutas con sesión y deja pasar solo cuentas
// activas. Con eliminación pendiente la persona puede entrar (nv-05), pero solo para ver su
// perfil, cancelar y cerrar sesión: esas rutas no llevan este guardia y todo lo demás responde 403.
func RequireActive(repo *Repository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := middleware.ClaimsFrom(c)
		if !ok {
			return apperr.Unauthorized("autenticación requerida")
		}
		id, err := bson.ObjectIDFromHex(claims.Subject)
		if err != nil {
			return apperr.Unauthorized("credenciales inválidas")
		}
		user, err := repo.FindByID(c.UserContext(), id)
		switch {
		case errors.Is(err, ErrNotFound):
			return apperr.Unauthorized("sesión inválida")
		case err != nil:
			return apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
		case user.Status == StatusPendingDeletion:
			return apperr.Forbidden("tu cuenta está en eliminación; cancélala para seguir usándola")
		case user.Status != StatusActive:
			return apperr.Forbidden("la cuenta no está activa")
		}
		return c.Next()
	}
}
