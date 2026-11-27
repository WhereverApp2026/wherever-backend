package auth

import (
	"bytes"
	"encoding/json"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Handler traduce HTTP ↔ service. No toca la base ni atrapa pánicos (test de arquitectura).
type Handler struct {
	svc       *Service
	validator *validation.Validator
	active    fiber.Handler
}

// NewHandler recibe el service y el validador compartido por constructor.
func NewHandler(svc *Service, v *validation.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// WithActiveGuard agrega el guardia de cuenta activa (usuarios.RequireActive) a las rutas con
// sesión, salvo logout y logout-todos: con la cuenta en eliminación pendiente responden 403.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// bind lee el JSON y lo valida: cuerpo ilegible → 400, reglas → 422.
func (h *Handler) bind(c *fiber.Ctx, dto any) error {
	if err := c.BodyParser(dto); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	return h.validator.Struct(dto)
}

// bindStrict es bind con decodificación estricta: campo de más = 400 (nv-72).
func (h *Handler) bindStrict(c *fiber.Ctx, dto any) error {
	dec := json.NewDecoder(bytes.NewReader(c.Body()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dto); err != nil || dec.More() {
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

// google entra con Google: 200 si la cuenta ya existía (o se vinculó), 201 si se creó.
func (h *Handler) google(c *fiber.Ctx) error {
	var req GoogleRequest
	if err := h.bindStrict(c, &req); err != nil {
		return err
	}
	res, created, err := h.svc.GoogleSignIn(c.UserContext(), GoogleInput{
		IDToken: req.IDToken, Device: req.Device.toDevice(), BirthDate: req.BirthDate,
		AcceptTerms: req.AcceptTerms, TermsVersion: req.TermsVersion, PromoEmail: req.PromoEmail,
		IP: middleware.ClientIP(c),
	})
	if err != nil {
		return err
	}
	if created {
		c.Status(fiber.StatusCreated)
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
	c.Set(fiber.HeaderCacheControl, "no-store") // datos del usuario: sin caché ni 304 (nv-17)
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
	in := ResetInput{Email: req.Email, Code: req.Code, NewPassword: req.NewPassword, IP: middleware.ClientIP(c)}
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
	if err := h.svc.VerifyEmail(c.UserContext(), claims(c), req.Code); err != nil {
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
