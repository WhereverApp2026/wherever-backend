package alta

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/expediente"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Políticas de peticiones (docs/rate-limit-matrix.md).
const (
	politicaExpediente = "expediente"
	limiteExpediente   = 10
	ventanaExpediente  = time.Hour
	politicaAdmin      = "admin"
	limiteAdmin        = 200
	ventanaAdmin       = time.Minute
)

// Handler traduce HTTP ↔ service. No toca la base ni atrapa pánicos (test de arquitectura).
type Handler struct {
	svc       *Service
	validator *validation.Validator
	active    fiber.Handler
}

// NewHandler recibe el service y el validador por constructor.
func NewHandler(svc *Service, v *validation.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// WithActiveGuard agrega usuarios.RequireActive a todas las rutas del dueño.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra el alta, el expediente y la revisión. Cumple httpapi.Module. `/negocios/mios`
// va antes de las rutas con `:id`.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	dueno := middleware.RequireRoles(middleware.RoleMerchant)
	expedienteRate := limiter.WindowUser(politicaExpediente, limiteExpediente, ventanaExpediente)

	v1.Post("/negocios", h.with(middleware.RequireRoles(middleware.RoleConsumer, middleware.RoleMerchant), h.crear)...)
	v1.Get("/negocios/mios", h.with(dueno, h.mios)...)
	v1.Patch("/negocios/:id/alta/:paso", h.with(dueno, h.paso)...)
	v1.Post("/negocios/:id/fotos", h.with(dueno, h.agregarFoto)...)
	v1.Put("/negocios/:id/fotos/orden", h.with(dueno, h.ordenarFotos)...)
	v1.Delete("/negocios/:id/fotos/:foto_id", h.with(dueno, h.quitarFoto)...)
	v1.Post("/negocios/:id/documentos", h.with(dueno, expedienteRate, h.subirDocumento)...)
	v1.Get("/negocios/:id/documentos", h.with(dueno, h.documentos)...)
	v1.Get("/negocios/:id/documentos/:tipo", h.with(dueno, h.leerDocumento)...)
	v1.Put("/negocios/:id/datos-fiscales", h.with(dueno, expedienteRate, h.fiscales)...)
	v1.Put("/negocios/:id/cuenta-deposito", h.with(dueno, expedienteRate, h.cuenta)...)
	v1.Post("/negocios/:id/enviar-revision", h.with(dueno, h.enviar)...)

	admin := []fiber.Handler{limiter.Window(politicaAdmin, limiteAdmin, ventanaAdmin), middleware.RequireRoles(middleware.RoleAdmin)}
	v1.Get("/admin/negocios", append(admin, h.cola)...)
	v1.Get("/admin/negocios/:id/documentos/:tipo", append(admin, h.leerAdmin)...)
	v1.Patch("/admin/negocios/:id/revision", append(admin, h.decidir)...)
}

// with arma la cadena: rol (401/403), cuenta activa y lo demás.
func (h *Handler) with(role fiber.Handler, rest ...fiber.Handler) []fiber.Handler {
	chain := []fiber.Handler{role}
	if h.active != nil {
		chain = append(chain, h.active)
	}
	return append(chain, rest...)
}

func claims(c *fiber.Ctx) *middleware.Claims {
	cl, _ := middleware.ClaimsFrom(c)
	return cl
}

func (h *Handler) bind(c *fiber.Ctx, dto any) error {
	if err := c.BodyParser(dto); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	return h.validator.Struct(dto)
}

// ifMatch lee la versión de `If-Match` (`3`, `"3"` o `W/"3"`). Falta o mal formada = 400.
func ifMatch(c *fiber.Ctx) (int, error) {
	raw := strings.Trim(strings.TrimPrefix(strings.TrimSpace(c.Get(fiber.HeaderIfMatch)), "W/"), `"`)
	v, err := strconv.Atoi(raw)
	if raw == "" || err != nil || v < 1 {
		return 0, apperr.BadRequest("falta la cabecera If-Match con la versión")
	}
	return v, nil
}

func noStore(c *fiber.Ctx, status int, out any) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(status).JSON(out)
}

func (h *Handler) crear(c *fiber.Ctx) error {
	var req CrearRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Crear(c.UserContext(), claims(c), req)
	if err != nil {
		return err
	}
	return noStore(c, fiber.StatusCreated, out)
}

func (h *Handler) mios(c *fiber.Ctx) error {
	out, err := h.svc.Mios(c.UserContext(), claims(c))
	if err != nil {
		return err
	}
	return noStore(c, fiber.StatusOK, fiber.Map{"items": out})
}

func (h *Handler) paso(c *fiber.Ctx) error {
	n, err := strconv.Atoi(c.Params("paso"))
	if err != nil || n < 2 || n > 6 {
		return apperr.Validation("datos inválidos", apperr.FieldError{Field: "paso", Message: "debe ser del 2 al 6"})
	}
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	p := Paso{Numero: n}
	switch n {
	case 2:
		p.Dos = &Paso2Request{}
		err = h.bind(c, p.Dos)
	case 3:
		p.Tres = &Paso3Request{}
		err = h.bind(c, p.Tres)
	case 5:
		p.Cinco = &Paso5Request{}
		err = h.bind(c, p.Cinco)
	}
	if err != nil {
		return err
	}
	out, err := h.svc.GuardarPaso(c.UserContext(), claims(c), c.Params("id"), version, p)
	if err != nil {
		return err
	}
	return noStore(c, fiber.StatusOK, out)
}

func (h *Handler) agregarFoto(c *fiber.Ctx) error {
	var req FotoRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.AgregarFoto(c.UserContext(), claims(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return noStore(c, fiber.StatusCreated, out)
}

func (h *Handler) ordenarFotos(c *fiber.Ctx) error {
	var req OrdenRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.OrdenarFotos(c.UserContext(), claims(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return noStore(c, fiber.StatusOK, out)
}

func (h *Handler) quitarFoto(c *fiber.Ctx) error {
	if err := h.svc.QuitarFoto(c.UserContext(), claims(c), c.Params("id"), c.Params("foto_id")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// subirDocumento recibe multipart `tipo` y `archivo` (≤ 5 MB). Nunca registra el cuerpo.
func (h *Handler) subirDocumento(c *fiber.Ctx) error {
	file, err := c.FormFile("archivo")
	if err != nil {
		return apperr.Validation("datos inválidos", apperr.FieldError{Field: "archivo", Message: "falta el archivo"})
	}
	if file.Size > expediente.MaxBytes {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "el archivo pasa de 5 MB")
	}
	opened, err := file.Open()
	if err != nil {
		return apperr.BadRequest("no se pudo leer el archivo")
	}
	defer func() { _ = opened.Close() }()
	datos, err := io.ReadAll(io.LimitReader(opened, expediente.MaxBytes+1))
	if err != nil {
		return apperr.BadRequest("no se pudo leer el archivo")
	}
	out, err := h.svc.SubirDocumento(c.UserContext(), claims(c), c.Params("id"), c.FormValue("tipo"), datos)
	if errors.Is(err, expediente.ErrGrande) {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "el archivo pasa de 5 MB")
	}
	if err != nil {
		return err
	}
	return noStore(c, fiber.StatusCreated, out)
}

func (h *Handler) documentos(c *fiber.Ctx) error {
	out, err := h.svc.Documentos(c.UserContext(), claims(c), c.Params("id"))
	if err != nil {
		return err
	}
	return noStore(c, fiber.StatusOK, out)
}

func (h *Handler) leerDocumento(c *fiber.Ctx) error {
	out, err := h.svc.LeerDocumento(c.UserContext(), claims(c), c.Params("id"), c.Params("tipo"))
	if err != nil {
		return err
	}
	return entregar(c, out)
}

// entregar manda el documento descifrado como adjunto y sin caché (compresión apagada en
// middleware.carriesTokens).
func entregar(c *fiber.Ctx, out expediente.Contenido) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set(fiber.HeaderContentType, out.ContentType)
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+out.Nombre+`"`)
	c.Set("X-Content-Type-Options", "nosniff")
	return c.Status(fiber.StatusOK).Send(out.Datos)
}

func (h *Handler) fiscales(c *fiber.Ctx) error {
	var req FiscalesRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.GuardarFiscales(c.UserContext(), claims(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return noStore(c, fiber.StatusOK, out)
}

func (h *Handler) cuenta(c *fiber.Ctx) error {
	var req CuentaRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.GuardarCuenta(c.UserContext(), claims(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return noStore(c, fiber.StatusOK, out)
}

func (h *Handler) enviar(c *fiber.Ctx) error {
	out, err := h.svc.EnviarRevision(c.UserContext(), claims(c), c.Params("id"))
	if err != nil {
		return err
	}
	return noStore(c, fiber.StatusOK, out)
}

func (h *Handler) cola(c *fiber.Ctx) error {
	q := ColaQuery{Limit: pagination.DefaultLimit}
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.Cola(c.UserContext(), q)
	if err != nil {
		return err
	}
	return noStore(c, fiber.StatusOK, out)
}

func (h *Handler) leerAdmin(c *fiber.Ctx) error {
	var q LecturaQuery
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.LeerAdmin(c.UserContext(), claims(c), c.Params("id"), c.Params("tipo"), q.Motivo,
		middleware.ClientIP(c), middleware.RequestIDFrom(c))
	if err != nil {
		return err
	}
	return entregar(c, out)
}

func (h *Handler) decidir(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	var req DecisionRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Decidir(c.UserContext(), claims(c), c.Params("id"), version, req)
	if err != nil {
		return err
	}
	return noStore(c, fiber.StatusOK, out)
}
