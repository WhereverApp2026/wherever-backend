package account

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Versions son las versiones vigentes de los términos y del aviso de privacidad. Salen de
// config.VersionApp (TERMINOS_VERSION y AVISO_VERSION, nv-16), las mismas que publica
// GET /configuracion.
type Versions struct {
	Terms  string
	Notice string
}

// WithConsents agrega el historial de aceptaciones y las versiones vigentes (spec nv-19). Sin
// él, POST /usuarios/yo/consentimientos falla cerrado (503) y `consentimiento_vigente` es falso.
func (s *Service) WithConsents(repo *usuarios.ConsentRepository, v Versions) *Service {
	s.consents = repo
	s.versions = v
	return s
}

// consentCurrent dice si la persona aceptó las versiones vigentes de términos y aviso.
func (s *Service) consentCurrent(p usuarios.Privacy) bool {
	return s.versions.Terms != "" && p.TermsVersion == s.versions.Terms && p.NoticeVersion == s.versions.Notice
}

func (s *Service) privacyOf(p usuarios.Privacy) PrivacyResponse {
	out := privacyResponse(p)
	current := s.consentCurrent(p)
	out.ConsentCurrent = &current
	return out
}

func (s *Service) profileOf(u usuarios.User) ProfileResponse {
	out := profileResponse(u)
	out.Privacy = s.privacyOf(u.Privacy)
	return out
}

// AcceptConsent registra la aceptación de las versiones vigentes (US-003): agrega una fila a
// `consentimientos` y copia la aceptación a `usuarios.privacidad`. Una versión que no es la
// vigente da 422: nadie acepta un texto que ya no se muestra.
func (s *Service) AcceptConsent(ctx context.Context, claims *middleware.Claims, req ConsentRequest) (ConsentResponse, error) {
	if s.consents == nil {
		return ConsentResponse{}, apperr.Unavailable("servicio no disponible")
	}
	if fields := s.staleVersions(req); len(fields) > 0 {
		return ConsentResponse{}, apperr.Validation("datos inválidos", fields...)
	}
	user, err := s.current(ctx, claims)
	if err != nil {
		return ConsentResponse{}, err
	}
	promo := user.Privacy.PromoEmail
	if req.PromoEmail != nil {
		promo = *req.PromoEmail
	}
	// Mongo guarda milisegundos: se trunca para que la respuesta y lo guardado coincidan.
	c := usuarios.Consent{UserID: user.ID, TermsVersion: req.TermsVersion, NoticeVersion: req.NoticeVersion,
		PromoEmail: promo, AcceptedAt: time.Now().UTC().Truncate(time.Millisecond)}
	if err := s.consents.Append(ctx, &c); err != nil {
		return ConsentResponse{}, dbError(err)
	}
	updated, err := s.users.Update(ctx, user.ID, bson.D{
		{Key: "privacidad.version_terminos", Value: c.TermsVersion},
		{Key: "privacidad.version_aviso", Value: c.NoticeVersion},
		{Key: "privacidad.fecha_aceptacion", Value: c.AcceptedAt},
		{Key: "privacidad.promociones_correo", Value: c.PromoEmail},
	})
	if err != nil {
		return ConsentResponse{}, s.userError(err)
	}
	return consentResponse(c, s.consentCurrent(updated.Privacy)), nil
}

// staleVersions devuelve un error por cada versión que no es la vigente.
func (s *Service) staleVersions(req ConsentRequest) []apperr.FieldError {
	var fields []apperr.FieldError
	if req.TermsVersion != s.versions.Terms {
		fields = append(fields, apperr.FieldError{Field: "version_terminos", Message: "no es la versión vigente de los términos"})
	}
	if req.NoticeVersion != s.versions.Notice {
		fields = append(fields, apperr.FieldError{Field: "version_aviso_privacidad", Message: "no es la versión vigente del aviso de privacidad"})
	}
	return fields
}
