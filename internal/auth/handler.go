package auth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Handler traduce HTTP ↔ service. No toca la base ni atrapa pánicos (test de arquitectura).
type Handler struct {
	svc       *Service
	validator *validation.Validator
}

// NewHandler recibe el service y el validador compartido por constructor.
func NewHandler(svc *Service, v *validation.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// bind lee el JSON y lo valida: cuerpo ilegible → 400, reglas → 422.
func (h *Handler) bind(c *fiber.Ctx, dto any) error {
	if err := c.BodyParser(dto); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	return h.validator.Struct(dto)
}

func claims(c *fiber.Ctx) *middleware.Claims {
	cl, _ := middleware.ClaimsFrom(c)
	return cl
}

func (h *Handler) login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	res, err := h.svc.Login(c.UserContext(), LoginInput{
		Email: req.Email, Password: req.Password, Device: req.Device.toDevice(), IP: middleware.ClientIP(c),
	})
	if err != nil {
		return err
	}
	return c.JSON(res)
}

func (h *Handler) refresh(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	res, err := h.svc.Refresh(c.UserContext(), req.RefreshToken, middleware.ClientIP(c))
	if err != nil {
		return err
	}
	return c.JSON(res)
}

func (h *Handler) logout(c *fiber.Ctx) error {
	if err := h.svc.Logout(c.UserContext(), claims(c)); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) logoutAll(c *fiber.Ctx) error {
	if err := h.svc.LogoutAll(c.UserContext(), claims(c)); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) listSessions(c *fiber.Ctx) error {
	sessions, err := h.svc.ListSessions(c.UserContext(), claims(c))
	if err != nil {
		return err
	}
	return c.JSON(sessions)
}

func (h *Handler) revokeSession(c *fiber.Ctx) error {
	if err := h.svc.RevokeSession(c.UserContext(), claims(c), c.Params("id")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) recoverPassword(c *fiber.Ctx) error {
	var req RecoverRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	if err := h.svc.RequestPasswordReset(c.UserContext(), req.Email); err != nil {
		return err
	}
	c.Status(fiber.StatusAccepted) // sin cuerpo: SendStatus escribiría "Accepted" como texto
	return nil
}

func (h *Handler) resetPassword(c *fiber.Ctx) error {
	var req ResetRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	in := ResetInput{Token: req.Token, NewPassword: req.NewPassword, IP: middleware.ClientIP(c)}
	if req.Device != nil {
		in.Device = req.Device.toDevice()
	}
	res, err := h.svc.ResetPassword(c.UserContext(), in)
	if err != nil {
		return err
	}
	return c.JSON(res)
}

func (h *Handler) verifyEmail(c *fiber.Ctx) error {
	var req VerifyEmailRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	if err := h.svc.VerifyEmail(c.UserContext(), req.Token); err != nil {
		return err
	}
	return c.JSON(VerifyEmailResponse{EmailVerified: true})
}

func (h *Handler) resendVerification(c *fiber.Ctx) error {
	if err := h.svc.ResendVerification(c.UserContext(), claims(c)); err != nil {
		return err
	}
	c.Status(fiber.StatusAccepted) // sin cuerpo: SendStatus escribiría "Accepted" como texto
	return nil
}

func (h *Handler) changePassword(c *fiber.Ctx) error {
	var req ChangePasswordRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	if err := h.svc.ChangePassword(c.UserContext(), claims(c), req.CurrentPassword, req.NewPassword); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
