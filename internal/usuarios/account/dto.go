package account

import (
	"strings"
	"time"
	"unicode"

	"github.com/WhereverApp2026/wherever-backend/internal/auth"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// RegisterRequest es el cuerpo de POST /usuarios (hoja A1). `dispositivo` es opcional: la
// sesión que se abre al registrarse lo guarda si llega.
type RegisterRequest struct {
	Name         string            `json:"nombre" validate:"required,min=2,max=40"`
	BirthDate    string            `json:"fecha_nacimiento" validate:"required,adult"`
	Email        string            `json:"correo" validate:"required,email,max=254"`
	Password     string            `json:"password" validate:"required,password"`
	AcceptTerms  bool              `json:"acepta_terminos"`
	TermsVersion string            `json:"version_terminos" validate:"required,max=20"`
	PromoEmail   bool              `json:"promociones_correo"`
	Device       *auth.DeviceInput `json:"dispositivo" validate:"omitempty"`
}

// EmailQuery es `?correo=` de GET /usuarios/correo-disponible.
type EmailQuery struct {
	Email string `query:"correo" validate:"required,email,max=254"`
}

// PatchProfileRequest es el cuerpo parcial de PATCH /usuarios/yo: solo cambia lo que llega.
type PatchProfileRequest struct {
	Name     *string `json:"nombre" validate:"omitempty,min=2,max=40"`
	Username *string `json:"nombre_usuario" validate:"omitempty,username"`
	PhotoURL *string `json:"foto_url" validate:"omitempty,url,startswith=https://,max=500"`
	CityID   *string `json:"ciudad_id" validate:"omitempty,objectid"`
}

// ChangeEmailRequest es el cuerpo de PUT /usuarios/yo/correo.
type ChangeEmailRequest struct {
	NewEmail string `json:"correo_nuevo" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,max=128"`
}

// TastesRequest es el cuerpo de PUT /usuarios/yo/gustos (P-10 a P-12).
type TastesRequest struct {
	Categories   []string `json:"categorias" validate:"required,min=3,max=10,unique,dive,required,max=40"`
	BudgetMin    int      `json:"presupuesto_min" validate:"min=0"`
	BudgetMax    int      `json:"presupuesto_max" validate:"gtefield=BudgetMin"`
	Restrictions []string `json:"restricciones" validate:"omitempty,max=10,dive,required,max=40"`
	Allergies    string   `json:"alergias" validate:"max=200"`
}

// QuietHoursInput es el horario silencioso.
type QuietHoursInput struct {
	From string `json:"desde" validate:"required,hhmm"`
	To   string `json:"hasta" validate:"required,hhmm"`
}

// NotificationsInput son los interruptores de notificaciones; solo cambia lo que llega.
type NotificationsInput struct {
	Bookings   *bool `json:"reservas"`
	Coupons    *bool `json:"cupones"`
	Groups     *bool `json:"grupos"`
	Reviews    *bool `json:"resenas"`
	Promotions *bool `json:"promociones"`
}

// PreferencesRequest es el cuerpo parcial de PATCH /usuarios/yo/preferencias. Los valores de
// movilidad, tema y unidades son provisionales hasta que la app fije su catálogo.
type PreferencesRequest struct {
	Mobility      *string             `json:"movilidad_default" validate:"omitempty,oneof=a_pie bicicleta auto transporte_publico"`
	Theme         *string             `json:"tema" validate:"omitempty,oneof=claro oscuro sistema"`
	Units         *string             `json:"unidades" validate:"omitempty,oneof=km mi"`
	QuietHours    *QuietHoursInput    `json:"horario_silencioso" validate:"omitempty"`
	Notifications *NotificationsInput `json:"notificaciones" validate:"omitempty"`
}

// PrivacyRequest es el cuerpo parcial de PATCH /usuarios/yo/privacidad (P-62).
type PrivacyRequest struct {
	Location           *bool   `json:"ubicacion"`
	Learning           *bool   `json:"aprendizaje_personalizado"`
	Mood               *bool   `json:"estado_animo"`
	SensitiveData      *bool   `json:"datos_sensibles"`
	UsageData          *bool   `json:"datos_uso"`
	PhoneSearchable    *bool   `json:"buscable_por_telefono"`
	UsernameSearchable *bool   `json:"buscable_por_usuario"`
	PromoEmail         *bool   `json:"promociones_correo"`
	TermsVersion       *string `json:"version_terminos" validate:"omitempty,max=20"`
}

// PhoneRequest es el cuerpo de PUT /usuarios/yo/telefono. Se valida ya normalizado.
type PhoneRequest struct {
	Phone string `json:"telefono" validate:"required,e164"`
}

// NormalizePhone quita espacios, guiones, puntos y paréntesis: "+52 (55) 1234-5678" →
// "+525512345678". Lo que quede debe ser E.164 (con `+` y lada del país).
func NormalizePhone(raw string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("-.()", r) {
			return -1
		}
		return r
	}, raw)
}

// ModeRequest es el cuerpo de POST /usuarios/yo/modo.
type ModeRequest struct {
	Mode string `json:"modo" validate:"required,oneof=explorar negocio"`
}

// DeleteRequest es el cuerpo de DELETE /usuarios/yo: hay que escribir ELIMINAR.
type DeleteRequest struct {
	Confirmation string `json:"confirmacion" validate:"required,max=20"`
}

// DownloadExportRequest es el cuerpo de POST /usuarios/exportaciones/descargar (D-2).
type DownloadExportRequest struct {
	Token string `json:"token" validate:"required,max=128"`
}

// CityResponse es la ciudad elegida.
type CityResponse struct {
	ID   string `json:"id"`
	Name string `json:"nombre"`
}

// TastesResponse son los gustos guardados.
type TastesResponse struct {
	Categories   []string `json:"categorias"`
	BudgetMin    int      `json:"presupuesto_min"`
	BudgetMax    int      `json:"presupuesto_max"`
	Restrictions []string `json:"restricciones"`
	Allergies    string   `json:"alergias"`
}

// QuietHoursResponse es el horario silencioso.
type QuietHoursResponse struct {
	From string `json:"desde"`
	To   string `json:"hasta"`
}

// NotificationsResponse son los interruptores de notificaciones.
type NotificationsResponse struct {
	Bookings   bool `json:"reservas"`
	Coupons    bool `json:"cupones"`
	Groups     bool `json:"grupos"`
	Reviews    bool `json:"resenas"`
	Promotions bool `json:"promociones"`
}

// PreferencesResponse son los ajustes de la app.
type PreferencesResponse struct {
	Mobility      string                `json:"movilidad_default"`
	Theme         string                `json:"tema"`
	Units         string                `json:"unidades"`
	QuietHours    *QuietHoursResponse   `json:"horario_silencioso"`
	Notifications NotificationsResponse `json:"notificaciones"`
}

// PrivacyResponse son los interruptores de privacidad y la aceptación de términos.
type PrivacyResponse struct {
	Location           bool      `json:"ubicacion"`
	Learning           bool      `json:"aprendizaje_personalizado"`
	Mood               bool      `json:"estado_animo"`
	SensitiveData      bool      `json:"datos_sensibles"`
	UsageData          bool      `json:"datos_uso"`
	PhoneSearchable    bool      `json:"buscable_por_telefono"`
	UsernameSearchable bool      `json:"buscable_por_usuario"`
	TermsVersion       string    `json:"version_terminos"`
	NoticeVersion      string    `json:"version_aviso_privacidad"`
	AcceptedAt         time.Time `json:"fecha_aceptacion"`
	PromoEmail         bool      `json:"promociones_correo"`
	// ConsentCurrent dice si aceptó las versiones vigentes (nv-19); si es falso, la app vuelve
	// a pedir la aceptación. No va en "Descargar mis datos".
	ConsentCurrent *bool `json:"consentimiento_vigente,omitempty"`
}

// ConsentRequest es el cuerpo de POST /usuarios/yo/consentimientos (nv-19). Las dos versiones
// deben ser las vigentes de GET /configuracion; `promociones_correo` ausente conserva el valor.
type ConsentRequest struct {
	TermsVersion  string `json:"version_terminos" validate:"required,max=20"`
	NoticeVersion string `json:"version_aviso_privacidad" validate:"required,max=20"`
	PromoEmail    *bool  `json:"promociones_correo"`
}

// ConsentResponse es la aceptación registrada.
type ConsentResponse struct {
	TermsVersion   string    `json:"version_terminos"`
	NoticeVersion  string    `json:"version_aviso_privacidad"`
	PromoEmail     bool      `json:"promociones_correo"`
	AcceptedAt     time.Time `json:"fecha_aceptacion"`
	ConsentCurrent bool      `json:"consentimiento_vigente"`
}

func consentResponse(c usuarios.Consent, current bool) ConsentResponse {
	return ConsentResponse{TermsVersion: c.TermsVersion, NoticeVersion: c.NoticeVersion, PromoEmail: c.PromoEmail,
		AcceptedAt: c.AcceptedAt.UTC(), ConsentCurrent: current}
}

// ProfileResponse es el perfil propio. Nunca lleva password_hash, telefono_hash ni google_id.
type ProfileResponse struct {
	ID              string              `json:"id"`
	Name            string              `json:"nombre"`
	Username        *string             `json:"nombre_usuario"`
	Email           string              `json:"correo"`
	EmailVerified   bool                `json:"correo_verificado"`
	PhotoURL        string              `json:"foto_url"`
	FriendCode      string              `json:"codigo_amigo"`
	Roles           []string            `json:"roles"`
	Mode            string              `json:"modo_actual"`
	City            *CityResponse       `json:"ciudad"`
	Tastes          *TastesResponse     `json:"gustos"`
	Preferences     PreferencesResponse `json:"preferencias"`
	Privacy         PrivacyResponse     `json:"privacidad"`
	PhoneRegistered bool                `json:"telefono_registrado"`
	Points          int                 `json:"puntos"`
	Level           string              `json:"nivel"`
	StreakWeeks     int                 `json:"racha_semanas"`
	Status          string              `json:"estado"`
	DeleteAt        *time.Time          `json:"eliminar_en"`
	Version         int                 `json:"version"`
	CreatedAt       time.Time           `json:"creado_en"`
	UpdatedAt       time.Time           `json:"actualizado_en"`
}

// EmailAvailableResponse es la respuesta de GET /usuarios/correo-disponible.
type EmailAvailableResponse struct {
	Available bool `json:"disponible"`
}

// PhoneResponse confirma que el teléfono quedó guardado (como HMAC, nunca en claro).
type PhoneResponse struct {
	PhoneRegistered bool `json:"telefono_registrado"`
}

// ModeResponse es el modo con el que abre la app.
type ModeResponse struct {
	Mode string `json:"modo_actual"`
}

// DeletionResponse es la fecha en que se borrará la cuenta.
type DeletionResponse struct {
	DeleteAt time.Time `json:"eliminar_en"`
}

func tastesResponse(t *usuarios.Tastes) *TastesResponse {
	if t == nil {
		return nil
	}
	restrictions := t.Restrictions
	if restrictions == nil {
		restrictions = []string{}
	}
	return &TastesResponse{Categories: t.Categories, BudgetMin: t.BudgetMin, BudgetMax: t.BudgetMax, Restrictions: restrictions, Allergies: t.Allergies}
}

func preferencesResponse(p usuarios.Preferences) PreferencesResponse {
	out := PreferencesResponse{
		Mobility: p.Mobility, Theme: p.Theme, Units: p.Units,
		Notifications: NotificationsResponse(p.Notifications),
	}
	if p.QuietHours != nil {
		out.QuietHours = &QuietHoursResponse{From: p.QuietHours.From, To: p.QuietHours.To}
	}
	return out
}

func privacyResponse(p usuarios.Privacy) PrivacyResponse {
	return PrivacyResponse{
		Location: p.Location, Learning: p.Learning, Mood: p.Mood, SensitiveData: p.SensitiveData, UsageData: p.UsageData,
		PhoneSearchable: p.PhoneSearchable, UsernameSearchable: p.SearchableByUsername(),
		TermsVersion: p.TermsVersion, NoticeVersion: p.NoticeVersion, AcceptedAt: p.AcceptedAt, PromoEmail: p.PromoEmail,
	}
}

func profileResponse(u usuarios.User) ProfileResponse {
	roles := u.Roles
	if roles == nil {
		roles = []string{}
	}
	out := ProfileResponse{
		ID: u.ID.Hex(), Name: u.Name, Email: u.Email, EmailVerified: u.EmailVerified, PhotoURL: u.PhotoURL,
		FriendCode: u.FriendCode, Roles: roles, Mode: u.Mode, Tastes: tastesResponse(u.Tastes),
		Preferences: preferencesResponse(u.Preferences), Privacy: privacyResponse(u.Privacy),
		PhoneRegistered: u.PhoneHash != "", Points: u.Points, Level: u.Level, StreakWeeks: u.StreakWeeks,
		Status: u.Status, DeleteAt: u.DeleteAt, Version: u.Version, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
	if u.Username != "" {
		out.Username = &u.Username
	}
	if u.City != nil {
		out.City = &CityResponse{ID: u.City.ID.Hex(), Name: u.City.Name}
	}
	// Las alergias guardadas antes de pedir este consentimiento tampoco se usan ni se muestran.
	if out.Tastes != nil && !u.Privacy.SensitiveData {
		out.Tastes.Allergies = ""
	}
	return out
}

// SettingsOf da los gustos, las preferencias y la privacidad con la misma forma de
// GET /usuarios/yo; la usa la sincronización (nv-46).
func SettingsOf(u usuarios.User) (*TastesResponse, PreferencesResponse, PrivacyResponse) {
	return tastesResponse(u.Tastes), preferencesResponse(u.Preferences), privacyResponse(u.Privacy)
}
