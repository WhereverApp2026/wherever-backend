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
	auth := middleware.RequireAuth()

	g.Post("/login", limiter.Window("login", loginLimit, loginWindow), h.login)
	g.Post("/refresh", perIP("auth_refresh"), h.refresh)
	g.Post("/logout", auth, h.logout)
	g.Post("/logout-todos", auth, h.logoutAll)
	g.Get("/sesiones", auth, h.listSessions)
	g.Delete("/sesiones/:id", auth, h.revokeSession)
	g.Post("/recuperar", perIP("auth_recuperar"), h.recoverPassword)
	g.Post("/restablecer", perIP("auth_restablecer"), h.resetPassword)
	g.Post("/verificar-correo", perIP("auth_verificar"), h.verifyEmail)
	g.Post("/reenviar-verificacion", auth, h.resendVerification)
	g.Put("/password", auth, h.changePassword)
}
