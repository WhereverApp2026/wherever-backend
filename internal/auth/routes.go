package auth

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
)

// Límites por ruta (docs/rate-limit-matrix.md). Corren antes de exigir el token.
const (
	loginLimit  = 5
	loginWindow = 5 * time.Minute
	linkLimit   = 10
	linkWindow  = time.Minute
)

// Routes registra /api/v1/auth. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	g := v1.Group("/auth")
	perIP := func(policy string) fiber.Handler { return limiter.Window(policy, linkLimit, linkWindow) }
	auth := []fiber.Handler{middleware.RequireAuth()}
	if h.active != nil {
		auth = append(auth, h.active)
	}
	with := func(handler fiber.Handler) []fiber.Handler {
		return append(append([]fiber.Handler{}, auth...), handler)
	}

	g.Post("/login", limiter.Window("login", loginLimit, loginWindow), h.login)
	g.Post("/refresh", perIP("auth_refresh"), h.refresh)
	// Cerrar sesión no cambia el estado de la cuenta: también se permite con eliminación pendiente.
	g.Post("/logout", middleware.RequireAuth(), h.logout)
	g.Post("/logout-todos", middleware.RequireAuth(), h.logoutAll)
	g.Get("/sesiones", with(h.listSessions)...)
	g.Delete("/sesiones/:id", with(h.revokeSession)...)
	g.Post("/recuperar", perIP("auth_recuperar"), h.recoverPassword)
	g.Post("/restablecer", perIP("auth_restablecer"), h.resetPassword)
	g.Post("/verificar-correo", perIP("auth_verificar"), h.verifyEmail)
	g.Post("/reenviar-verificacion", with(h.resendVerification)...)
	g.Put("/password", with(h.changePassword)...)
}
