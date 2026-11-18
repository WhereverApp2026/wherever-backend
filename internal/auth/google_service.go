package auth

import (
	"context"
	"errors"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Mensajes públicos de POST /auth/google (nv-72). El de 401 es el mismo para cualquier falla.
const (
	googleInvalid    = "credenciales inválidas"
	googleIncomplete = "registro_incompleto"
	googleLinked     = "cuenta_vinculada"
)

// GoogleInput son los datos de POST /auth/google ya validados.
type GoogleInput struct {
	IDToken      string
	Device       Device
	BirthDate    string
	AcceptTerms  *bool
	TermsVersion string
	PromoEmail   bool
	IP           string
}

// WithGoogle conecta el verificador de id_token. Sin él (falta GOOGLE_CLIENT_ID) la ruta da 503.
func (s *Service) WithGoogle(v GoogleVerifier) *Service {
	s.google = v
	return s
}

// GoogleSignIn entra con un id_token de Google. Busca por `google_id`; si no, por correo y lo
// vincula; si no hay cuenta, la crea solo con mayoría de edad y términos (D-1 camino B).
// Devuelve created = true cuando la cuenta es nueva (201).
func (s *Service) GoogleSignIn(ctx context.Context, in GoogleInput) (TokenResponse, bool, error) {
	if s.google == nil {
		return TokenResponse{}, false, apperr.Unavailable("inicio con Google no disponible")
	}
	id, err := s.google.Verify(ctx, in.IDToken)
	if errors.Is(err, ErrGoogleUnavailable) {
		return TokenResponse{}, false, apperr.Unavailable("no se pudo verificar con Google")
	}
	if err != nil {
		return TokenResponse{}, false, apperr.Unauthorized(googleInvalid)
	}
	user, err := s.users.FindByGoogleID(ctx, id.Subject)
	if err == nil {
		return s.googleSession(ctx, user, in, false)
	}
	if !errors.Is(err, usuarios.ErrNotFound) {
		return TokenResponse{}, false, dbError(err)
	}
	user, err = s.users.FindByEmail(ctx, id.Email)
	if err == nil {
		return s.linkGoogle(ctx, user, id, in)
	}
	if !errors.Is(err, usuarios.ErrNotFound) {
		return TokenResponse{}, false, dbError(err)
	}
	return s.createFromGoogle(ctx, id, in)
}

// linkGoogle vincula una cuenta de correo existente. Si el correo no estaba verificado, quien
// la creó no probó ser dueño: se borra la contraseña y se cierran sus sesiones.
func (s *Service) linkGoogle(ctx context.Context, user usuarios.User, id GoogleIdentity, in GoogleInput) (TokenResponse, bool, error) {
	if user.GoogleID != "" {
		return TokenResponse{}, false, apperr.Conflict(googleLinked)
	}
	if !user.CanSignIn() {
		return TokenResponse{}, false, apperr.Unauthorized(googleInvalid)
	}
	unverified := !user.EmailVerified
	linked, err := s.users.LinkGoogle(ctx, user.ID, id.Subject, unverified)
	if errors.Is(err, usuarios.ErrNotFound) || errors.Is(err, usuarios.ErrGoogleTaken) {
		return TokenResponse{}, false, apperr.Conflict(googleLinked)
	}
	if err != nil {
		return TokenResponse{}, false, dbError(err)
	}
	if unverified {
		if err := s.CloseAllSessions(ctx, linked.ID); err != nil {
			return TokenResponse{}, false, err
		}
	}
	return s.googleSession(ctx, linked, in, false)
}

// createFromGoogle crea la cuenta (camino B): sin fecha de nacimiento, aceptación y versión de
// términos responde 422 `registro_incompleto` sin guardar nada.
func (s *Service) createFromGoogle(ctx context.Context, id GoogleIdentity, in GoogleInput) (TokenResponse, bool, error) {
	if missing := missingSignup(in); len(missing) > 0 {
		return TokenResponse{}, false, apperr.Validation(googleIncomplete, missing...)
	}
	now := time.Now().UTC()
	birth, err := time.Parse(validation.DateLayout, in.BirthDate)
	if err != nil || !validation.IsAdult(birth, now) {
		return TokenResponse{}, false, apperr.Validation("datos inválidos", apperr.FieldError{Field: "fecha_nacimiento", Message: "debes tener 18 años o más"})
	}
	if !*in.AcceptTerms {
		return TokenResponse{}, false, apperr.Validation("datos inválidos", apperr.FieldError{Field: "acepta_terminos", Message: "debes aceptar los términos"})
	}
	user := usuarios.NewConsumer(googleName(id.Name, id.Email), id.Email, birth, in.TermsVersion, in.PromoEmail, now)
	user.EmailVerified = true
	user.GoogleID = id.Subject
	user.PhotoURL = googlePhoto(id.Picture)
	err = s.users.Create(ctx, &user)
	if errors.Is(err, usuarios.ErrGoogleTaken) || errors.Is(err, usuarios.ErrEmailTaken) {
		// Otra petición con el mismo token creó la cuenta un instante antes.
		return TokenResponse{}, false, apperr.Conflict(googleLinked)
	}
	if err != nil {
		return TokenResponse{}, false, dbError(err)
	}
	return s.googleSession(ctx, user, in, true)
}

// missingSignup lista los datos de registro que faltan para crear la cuenta (`faltantes[]`,
// en `campos` del error uniforme).
func missingSignup(in GoogleInput) []apperr.FieldError {
	var out []apperr.FieldError
	if in.BirthDate == "" {
		out = append(out, apperr.FieldError{Field: "fecha_nacimiento", Message: "falta"})
	}
	if in.AcceptTerms == nil {
		out = append(out, apperr.FieldError{Field: "acepta_terminos", Message: "falta"})
	}
	if in.TermsVersion == "" {
		out = append(out, apperr.FieldError{Field: "version_terminos", Message: "falta"})
	}
	return out
}

func (s *Service) googleSession(ctx context.Context, user usuarios.User, in GoogleInput, created bool) (TokenResponse, bool, error) {
	if !user.CanSignIn() {
		return TokenResponse{}, false, apperr.Unauthorized(googleInvalid)
	}
	res, err := s.CreateSession(ctx, user, MethodGoogle, in.Device, in.IP)
	return res, created, err
}
