// Package account es el módulo de usuarios de nv-05 (hoja A1): registro, perfil, ajustes y
// derechos ARCO. Va aparte de internal/usuarios (modelo y repositorio) porque usa a auth para
// abrir sesiones, y auth usa al repositorio de usuarios: juntos harían un ciclo de imports.
package account

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"slices"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/auth"
	"github.com/WhereverApp2026/wherever-backend/internal/ciudades"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// DeleteConfirmation es lo que hay que escribir para eliminar la cuenta (hoja A1).
const DeleteConfirmation = "ELIMINAR"

// Config son los parámetros del servicio, resueltos desde config.Config en main.
type Config struct {
	PhoneHMACKey []byte
	GraceDays    int
}

// Service implementa los casos de uso de A1. Recibe todo por constructor y no guarda estado.
type Service struct {
	users     *usuarios.Repository
	sessions  *auth.Service
	cities    *ciudades.Service
	exports   *privacidad.Exports
	registry  *privacidad.Registry
	hasher    *auth.Hasher
	phoneKey  []byte
	graceDays int
	log       *slog.Logger
}

// NewService arma el servicio; falla solo si no puede preparar el hasher.
func NewService(users *usuarios.Repository, sessions *auth.Service, cities *ciudades.Service, exports *privacidad.Exports,
	registry *privacidad.Registry, cfg Config, log *slog.Logger) (*Service, error) {
	hasher, err := auth.NewHasher()
	if err != nil {
		return nil, err
	}
	return &Service{users: users, sessions: sessions, cities: cities, exports: exports, registry: registry,
		hasher: hasher, phoneKey: cfg.PhoneHMACKey, graceDays: cfg.GraceDays, log: log}, nil
}

// RegisterInput son los datos de POST /usuarios ya validados.
type RegisterInput struct {
	Request RegisterRequest
	IP      string
}

// Register crea la cuenta (rol consumidor, 50 puntos), manda la verificación y abre sesión.
func (s *Service) Register(ctx context.Context, in RegisterInput) (auth.TokenResponse, error) {
	req := in.Request
	now := time.Now().UTC()
	birth, err := time.Parse(validation.DateLayout, req.BirthDate)
	if err != nil || !validation.IsAdult(birth, now) {
		return auth.TokenResponse{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "fecha_nacimiento", Message: "debes tener 18 años o más"})
	}
	if !req.AcceptTerms {
		return auth.TokenResponse{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "acepta_terminos", Message: "debes aceptar los términos"})
	}
	hash, err := s.hasher.Hash(req.Password)
	if err != nil {
		return auth.TokenResponse{}, err
	}
	user := usuarios.User{
		Name: strings.TrimSpace(req.Name), NormalizedName: text.Normalize(req.Name), Email: req.Email,
		PasswordHash: hash, BirthDate: &birth, Roles: []string{middleware.RoleConsumer}, Mode: usuarios.ModeExplore,
		Preferences: usuarios.DefaultPreferences(),
		Privacy:     usuarios.Privacy{TermsVersion: req.TermsVersion, AcceptedAt: now, PromoEmail: req.PromoEmail},
		Points:      usuarios.WelcomePoints, Level: usuarios.InitialLevel, Status: usuarios.StatusActive,
	}
	if err := s.users.Create(ctx, &user); err != nil {
		return auth.TokenResponse{}, s.userError(err)
	}
	if err := s.sessions.SendVerification(ctx, user); err != nil {
		s.log.ErrorContext(ctx, "registro: no se pudo enviar la verificación", "error", err.Error())
	}
	var device auth.Device
	if req.Device != nil {
		device = auth.Device(*req.Device)
	}
	return s.sessions.CreateSession(ctx, user, auth.MethodEmail, device, in.IP)
}

// EmailAvailable responde si el correo está libre.
func (s *Service) EmailAvailable(ctx context.Context, email string) (bool, error) {
	exists, err := s.users.EmailExists(ctx, email)
	if err != nil {
		return false, dbError(err)
	}
	return !exists, nil
}

// Profile devuelve el perfil propio.
func (s *Service) Profile(ctx context.Context, claims *middleware.Claims) (ProfileResponse, error) {
	user, err := s.current(ctx, claims)
	if err != nil {
		return ProfileResponse{}, err
	}
	return profileResponse(user), nil
}

// PatchProfile edita nombre, nombre de usuario, foto o ciudad solo si `version` sigue vigente (If-Match).
func (s *Service) PatchProfile(ctx context.Context, claims *middleware.Claims, version int, req PatchProfileRequest) (ProfileResponse, error) {
	userID, err := userIDOf(claims)
	if err != nil {
		return ProfileResponse{}, err
	}
	set, err := s.profileChanges(ctx, req)
	if err != nil {
		return ProfileResponse{}, err
	}
	user, err := s.users.UpdateIfVersion(ctx, userID, version, set)
	if err != nil {
		return ProfileResponse{}, s.userError(err)
	}
	return profileResponse(user), nil
}

func (s *Service) profileChanges(ctx context.Context, req PatchProfileRequest) (bson.D, error) {
	var set bson.D
	if req.Name != nil {
		set = append(set, bson.E{Key: "nombre", Value: strings.TrimSpace(*req.Name)}, bson.E{Key: "nombre_normalizado", Value: text.Normalize(*req.Name)})
	}
	if req.Username != nil {
		set = append(set, bson.E{Key: "nombre_usuario", Value: *req.Username})
	}
	if req.PhotoURL != nil {
		set = append(set, bson.E{Key: "foto_url", Value: *req.PhotoURL})
	}
	if req.CityID != nil {
		id, err := bson.ObjectIDFromHex(*req.CityID)
		if err != nil {
			return nil, apperr.Validation("datos inválidos", apperr.FieldError{Field: "ciudad_id", Message: "no es un identificador válido"})
		}
		city, err := s.cities.Find(ctx, id)
		if errors.Is(err, ciudades.ErrNotFound) {
			return nil, apperr.Validation("datos inválidos", apperr.FieldError{Field: "ciudad_id", Message: "la ciudad no existe"})
		}
		if err != nil {
			return nil, dbError(err)
		}
		set = append(set, bson.E{Key: "ciudad", Value: usuarios.City{ID: city.ID, Name: city.Name}})
	}
	if len(set) == 0 {
		return nil, apperr.Validation("envía al menos un campo para cambiar")
	}
	return set, nil
}

// ChangeEmail pide la contraseña actual, deja el correo sin verificar y manda la verificación.
func (s *Service) ChangeEmail(ctx context.Context, claims *middleware.Claims, req ChangeEmailRequest) (ProfileResponse, error) {
	user, err := s.current(ctx, claims)
	if err != nil {
		return ProfileResponse{}, err
	}
	if !s.hasher.Verify(req.Password, user.PasswordHash) {
		return ProfileResponse{}, apperr.Unauthorized("la contraseña no coincide")
	}
	updated, err := s.users.Update(ctx, user.ID, bson.D{
		{Key: "correo", Value: usuarios.NormalizeEmail(req.NewEmail)}, {Key: "correo_verificado", Value: false},
	})
	if err != nil {
		return ProfileResponse{}, s.userError(err)
	}
	if err := s.sessions.SendVerification(ctx, updated); err != nil {
		s.log.ErrorContext(ctx, "cambio de correo: no se pudo enviar la verificación", "error", err.Error())
	}
	return profileResponse(updated), nil
}

// SetTastes guarda los gustos (solo consumidor). Los 50 puntos de bienvenida se dan al
// registrarse; aquí no se suman otra vez.
func (s *Service) SetTastes(ctx context.Context, claims *middleware.Claims, req TastesRequest) (*TastesResponse, error) {
	if claims == nil || !slices.Contains(claims.Roles, middleware.RoleConsumer) {
		return nil, apperr.Forbidden("solo una cuenta de consumidor guarda gustos")
	}
	userID, err := userIDOf(claims)
	if err != nil {
		return nil, err
	}
	if err := s.requireSensitiveConsent(ctx, claims, req.Allergies); err != nil {
		return nil, err
	}
	tastes := usuarios.Tastes{Categories: req.Categories, BudgetMin: req.BudgetMin, BudgetMax: req.BudgetMax, Restrictions: req.Restrictions, Allergies: req.Allergies}
	user, err := s.users.Update(ctx, userID, bson.D{{Key: "gustos", Value: tastes}})
	if err != nil {
		return nil, s.userError(err)
	}
	return tastesResponse(user.Tastes), nil
}

// requireSensitiveConsent aplica US-010: sin consentimiento expreso las alergias no se guardan.
func (s *Service) requireSensitiveConsent(ctx context.Context, claims *middleware.Claims, allergies string) error {
	if strings.TrimSpace(allergies) == "" {
		return nil
	}
	user, err := s.current(ctx, claims)
	if err != nil {
		return err
	}
	if !user.Privacy.SensitiveData {
		return apperr.Validation("datos inválidos", apperr.FieldError{Field: "alergias", Message: "requiere el consentimiento de datos sensibles"})
	}
	return nil
}

// clearAllergiesOnRevoke borra las alergias al retirar el consentimiento. Si la cuenta no tiene
// gustos no hay nada que borrar, y fijar un subcampo sobre `gustos` nulo fallaría en MongoDB.
func (s *Service) clearAllergiesOnRevoke(ctx context.Context, claims *middleware.Claims, consent *bool, set *bson.D) error {
	if consent == nil || *consent {
		return nil
	}
	user, err := s.current(ctx, claims)
	if err != nil {
		return err
	}
	if user.Tastes != nil {
		*set = append(*set, bson.E{Key: "gustos.alergias", Value: ""})
	}
	return nil
}

// PatchPreferences cambia solo los ajustes que llegan; las notificaciones de reservas no se apagan.
func (s *Service) PatchPreferences(ctx context.Context, claims *middleware.Claims, req PreferencesRequest) (PreferencesResponse, error) {
	userID, err := userIDOf(claims)
	if err != nil {
		return PreferencesResponse{}, err
	}
	set := bson.D{}
	setIf(&set, "preferencias.movilidad_default", req.Mobility)
	setIf(&set, "preferencias.tema", req.Theme)
	setIf(&set, "preferencias.unidades", req.Units)
	if req.QuietHours != nil {
		set = append(set, bson.E{Key: "preferencias.horario_silencioso", Value: usuarios.QuietHours{From: req.QuietHours.From, To: req.QuietHours.To}})
	}
	if n := req.Notifications; n != nil {
		if n.Bookings != nil && !*n.Bookings {
			return PreferencesResponse{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "reservas", Message: "las notificaciones de reservas no se pueden apagar"})
		}
		setIf(&set, "preferencias.notificaciones.cupones", n.Coupons)
		setIf(&set, "preferencias.notificaciones.grupos", n.Groups)
		setIf(&set, "preferencias.notificaciones.resenas", n.Reviews)
		setIf(&set, "preferencias.notificaciones.promociones", n.Promotions)
	}
	user, err := s.users.Update(ctx, userID, set)
	if err != nil {
		return PreferencesResponse{}, s.userError(err)
	}
	return preferencesResponse(user.Preferences), nil
}

// PatchPrivacy cambia los interruptores que llegan; una versión de términos nueva guarda la fecha.
func (s *Service) PatchPrivacy(ctx context.Context, claims *middleware.Claims, req PrivacyRequest) (PrivacyResponse, error) {
	userID, err := userIDOf(claims)
	if err != nil {
		return PrivacyResponse{}, err
	}
	set := bson.D{}
	setIf(&set, "privacidad.ubicacion", req.Location)
	setIf(&set, "privacidad.aprendizaje_personalizado", req.Learning)
	setIf(&set, "privacidad.estado_animo", req.Mood)
	setIf(&set, "privacidad.datos_sensibles", req.SensitiveData)
	if err := s.clearAllergiesOnRevoke(ctx, claims, req.SensitiveData, &set); err != nil {
		return PrivacyResponse{}, err
	}
	setIf(&set, "privacidad.datos_uso", req.UsageData)
	setIf(&set, "privacidad.buscable_por_telefono", req.PhoneSearchable)
	setIf(&set, "privacidad.buscable_por_usuario", req.UsernameSearchable)
	setIf(&set, "privacidad.promociones_correo", req.PromoEmail)
	if req.TermsVersion != nil {
		set = append(set, bson.E{Key: "privacidad.version_terminos", Value: *req.TermsVersion}, bson.E{Key: "privacidad.fecha_aceptacion", Value: time.Now().UTC()})
	}
	user, err := s.users.Update(ctx, userID, set)
	if err != nil {
		return PrivacyResponse{}, s.userError(err)
	}
	return privacyResponse(user.Privacy), nil
}

// SetPhone guarda el teléfono como HMAC-SHA-256 (nunca en claro); repetido entre cuentas = 409.
func (s *Service) SetPhone(ctx context.Context, claims *middleware.Claims, phone string) error {
	userID, err := userIDOf(claims)
	if err != nil {
		return err
	}
	_, err = s.users.Update(ctx, userID, bson.D{{Key: "telefono_hash", Value: s.phoneHash(phone)}})
	return s.userError(err)
}

func (s *Service) phoneHash(phone string) string { return usuarios.HashPhone(s.phoneKey, phone) }

// SetMode cambia el modo; `negocio` exige el rol comerciante (403).
func (s *Service) SetMode(ctx context.Context, claims *middleware.Claims, mode string) (ModeResponse, error) {
	user, err := s.current(ctx, claims)
	if err != nil {
		return ModeResponse{}, err
	}
	if mode == usuarios.ModeBusiness && !slices.Contains(user.Roles, middleware.RoleMerchant) {
		return ModeResponse{}, apperr.Forbidden("el modo negocio necesita una cuenta de comerciante")
	}
	updated, err := s.users.Update(ctx, user.ID, bson.D{{Key: "modo_actual", Value: mode}})
	if err != nil {
		return ModeResponse{}, s.userError(err)
	}
	return ModeResponse{Mode: updated.Mode}, nil
}

// RequestExport pide "Descargar mis datos": una por día (429); la arma el trabajo nocturno y
// manda un enlace de un solo uso (D-2).
func (s *Service) RequestExport(ctx context.Context, claims *middleware.Claims) error {
	userID, err := userIDOf(claims)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	last, ok, err := s.exports.LastRequestedAt(ctx, userID)
	if err != nil {
		return dbError(err)
	}
	if wait := privacidad.ExportCooldown - now.Sub(last); ok && wait > 0 {
		return apperr.RateLimited(int(math.Ceil(wait.Seconds())))
	}
	if err := s.exports.Request(ctx, userID, now); err != nil {
		return dbError(err)
	}
	return nil
}

// DownloadExport usa el enlace una sola vez y devuelve el JSON ya armado; el archivo se borra
// de la base en ese momento.
func (s *Service) DownloadExport(ctx context.Context, token string) (string, error) {
	userID, err := s.sessions.ConsumeExportLink(ctx, token)
	if err != nil {
		return "", err
	}
	exp, err := s.exports.TakeReady(ctx, userID, time.Now().UTC())
	if errors.Is(err, privacidad.ErrNoExport) {
		return "", apperr.Gone("la exportación ya no está disponible; pide otra")
	}
	if err != nil {
		return "", dbError(err)
	}
	return exp.Data, nil
}

// PurgeHistory borra el historial de uso de todos los módulos registrados; la cuenta sigue.
func (s *Service) PurgeHistory(ctx context.Context, claims *middleware.Claims) error {
	userID, err := userIDOf(claims)
	if err != nil {
		return err
	}
	if err := s.registry.PurgeHistory(ctx, userID); err != nil {
		return apperr.Unavailable("no se pudo borrar todo el historial; intenta de nuevo").WithDetail(err.Error())
	}
	return nil
}

// RequestDeletion exige ELIMINAR, deja la cuenta pendiente por los días de gracia y cierra
// todas las sesiones. El borrado real lo hace el trabajo nocturno.
func (s *Service) RequestDeletion(ctx context.Context, claims *middleware.Claims, confirmation string) (DeletionResponse, error) {
	if confirmation != DeleteConfirmation {
		return DeletionResponse{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "confirmacion", Message: "escribe ELIMINAR para confirmar"})
	}
	userID, err := userIDOf(claims)
	if err != nil {
		return DeletionResponse{}, err
	}
	at := time.Now().UTC().AddDate(0, 0, s.graceDays)
	if _, err := s.users.MarkForDeletion(ctx, userID, at); err != nil {
		return DeletionResponse{}, s.userError(err)
	}
	if err := s.sessions.CloseAllSessions(ctx, userID); err != nil {
		return DeletionResponse{}, err
	}
	return DeletionResponse{DeleteAt: at}, nil
}

// CancelDeletion vuelve la cuenta a activa si sigue pendiente y no venció el plazo (si no, 409).
func (s *Service) CancelDeletion(ctx context.Context, claims *middleware.Claims) (ProfileResponse, error) {
	userID, err := userIDOf(claims)
	if err != nil {
		return ProfileResponse{}, err
	}
	user, err := s.users.CancelDeletion(ctx, userID, time.Now().UTC())
	if errors.Is(err, usuarios.ErrNotFound) {
		return ProfileResponse{}, apperr.Conflict("no hay una eliminación pendiente o ya venció el plazo")
	}
	if err != nil {
		return ProfileResponse{}, dbError(err)
	}
	return profileResponse(user), nil
}

func (s *Service) current(ctx context.Context, claims *middleware.Claims) (usuarios.User, error) {
	userID, err := userIDOf(claims)
	if err != nil {
		return usuarios.User{}, err
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return usuarios.User{}, s.userError(err)
	}
	return user, nil
}

// userError traduce los errores del repositorio; nil queda nil.
func (s *Service) userError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, usuarios.ErrNotFound):
		return apperr.Unauthorized("sesión inválida")
	case errors.Is(err, usuarios.ErrEmailTaken):
		return apperr.Conflict("el correo ya está registrado")
	case errors.Is(err, usuarios.ErrPhoneTaken):
		return apperr.Conflict("el teléfono ya está registrado en otra cuenta")
	case errors.Is(err, usuarios.ErrUsernameTaken):
		return apperr.Conflict("el nombre de usuario ya está en uso")
	case errors.Is(err, usuarios.ErrVersionConflict):
		return apperr.Conflict("el perfil cambió en otro dispositivo; vuelve a cargarlo")
	default:
		return dbError(err)
	}
}

func userIDOf(claims *middleware.Claims) (bson.ObjectID, error) {
	if claims == nil {
		return bson.ObjectID{}, apperr.Unauthorized("autenticación requerida")
	}
	id, err := bson.ObjectIDFromHex(claims.Subject)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("credenciales inválidas")
	}
	return id, nil
}

func setIf[T any](set *bson.D, key string, v *T) {
	if v != nil {
		*set = append(*set, bson.E{Key: key, Value: *v})
	}
}

func dbError(err error) error {
	return apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
}
