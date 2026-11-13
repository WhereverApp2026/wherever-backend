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

// RetiroDatosNegocio aplica la oposición al retirar el consentimiento `datos_negocio` (nv-55):
// borra el expediente del negocio. Lo cumple expediente.Privacy.
type RetiroDatosNegocio interface {
	RetirarDatosNegocio(ctx context.Context, userID bson.ObjectID) error
}

// WithRetiroDatosNegocio conecta la oposición del expediente del negocio.
func (s *Service) WithRetiroDatosNegocio(r RetiroDatosNegocio) *Service {
	s.retiro = r
	return s
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
	business, err := s.businessConsent(ctx, user, req.BusinessData)
	if err != nil {
		return ConsentResponse{}, err
	}
	// Mongo guarda milisegundos: se trunca para que la respuesta y lo guardado coincidan.
	c := usuarios.Consent{UserID: user.ID, TermsVersion: req.TermsVersion, NoticeVersion: req.NoticeVersion,
		PromoEmail: promo, BusinessData: business, AcceptedAt: time.Now().UTC().Truncate(time.Millisecond)}
	if err := s.consents.Append(ctx, &c); err != nil {
		return ConsentResponse{}, dbError(err)
	}
	updated, err := s.users.Update(ctx, user.ID, bson.D{
		{Key: "privacidad.version_terminos", Value: c.TermsVersion},
		{Key: "privacidad.version_aviso", Value: c.NoticeVersion},
		{Key: "privacidad.fecha_aceptacion", Value: c.AcceptedAt},
		{Key: "privacidad.promociones_correo", Value: c.PromoEmail},
		{Key: "privacidad.datos_negocio", Value: c.BusinessData},
	})
	if err != nil {
		return ConsentResponse{}, s.userError(err)
	}
	return consentResponse(c, s.consentCurrent(updated.Privacy)), nil
}

// businessConsent resuelve el consentimiento `datos_negocio` (nv-55). Retirarlo es la oposición:
// primero se borra el expediente; si falla, el consentimiento sigue y la persona reintenta.
func (s *Service) businessConsent(ctx context.Context, user usuarios.User, requested *bool) (bool, error) {
	if requested == nil {
		return user.Privacy.BusinessData, nil
	}
	if user.Privacy.BusinessData && !*requested && s.retiro != nil {
		if err := s.retiro.RetirarDatosNegocio(ctx, user.ID); err != nil {
			return false, apperr.Unavailable("servicio no disponible")
		}
	}
	return *requested, nil
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
