package auth

import "time"

// DeviceInput es el `dispositivo{}` del cuerpo (hoja J1: modelo, sistema, app_version).
type DeviceInput struct {
	Model      string `json:"modelo" validate:"required,max=80"`
	System     string `json:"sistema" validate:"required,max=40"`
	AppVersion string `json:"app_version" validate:"omitempty,max=40"`
}

func (d DeviceInput) toDevice() Device {
	return Device(d)
}

// LoginRequest es el cuerpo de POST /auth/login. La política de contraseña no se exige
// aquí: solo al fijar una nueva, para no rechazar contraseñas viejas.
type LoginRequest struct {
	Email    string      `json:"correo" validate:"required,email,max=254"`
	Password string      `json:"password" validate:"required,max=128"`
	Device   DeviceInput `json:"dispositivo" validate:"required"`
}

// GoogleRequest es el cuerpo de POST /auth/google (nv-72). Los datos de registro solo hacen
// falta si el correo no tiene cuenta (D-1 camino B); si faltan, 422 `registro_incompleto`.
type GoogleRequest struct {
	IDToken      string      `json:"id_token" validate:"required,max=4096"`
	Device       DeviceInput `json:"dispositivo" validate:"required"`
	BirthDate    string      `json:"fecha_nacimiento" validate:"omitempty,adult"`
	AcceptTerms  *bool       `json:"acepta_terminos"`
	TermsVersion string      `json:"version_terminos" validate:"omitempty,max=20"`
	PromoEmail   bool        `json:"promociones_correo"`
}

// RefreshRequest es el cuerpo de POST /auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required,max=128"`
}

// RecoverRequest es el cuerpo de POST /auth/recuperar.
type RecoverRequest struct {
	Email string `json:"correo" validate:"required,email,max=254"`
}

// ResetRequest es el cuerpo de POST /auth/restablecer: el correo y el código de 6 dígitos que
// llegó a ese correo (nv-94). `dispositivo` es opcional: la sesión que se abre al restablecer
// lo registra si llega.
type ResetRequest struct {
	Email       string       `json:"correo" validate:"required,email,max=254"`
	Code        string       `json:"codigo" validate:"required,len=6,number"`
	NewPassword string       `json:"password_nuevo" validate:"required,password"`
	Device      *DeviceInput `json:"dispositivo" validate:"omitempty"`
}

// VerifyEmailRequest es el cuerpo de POST /auth/verificar-correo: el código de 6 dígitos del
// correo (nv-91).
type VerifyEmailRequest struct {
	Code string `json:"codigo" validate:"required,len=6,number"`
}

// ChangePasswordRequest es el cuerpo de PUT /auth/password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"password_actual" validate:"required,max=128"`
	NewPassword     string `json:"password_nuevo" validate:"required,password"`
}

// UserResponse es el usuario que acompaña a los tokens; nunca lleva hashes ni google_id.
type UserResponse struct {
	ID            string   `json:"id"`
	Name          string   `json:"nombre"`
	Email         string   `json:"correo"`
	EmailVerified bool     `json:"correo_verificado"`
	Roles         []string `json:"roles"`
}

// TokenResponse es la respuesta de login, refresh y restablecer.
// `expira_en` es el vencimiento del access token (UTC, ISO 8601).
type TokenResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresAt    time.Time    `json:"expira_en"`
	User         UserResponse `json:"usuario"`
}

// DeviceResponse es el dispositivo de una sesión.
type DeviceResponse struct {
	Model      string `json:"modelo"`
	System     string `json:"sistema"`
	AppVersion string `json:"app_version"`
}

// SessionResponse es un elemento de GET /auth/sesiones.
type SessionResponse struct {
	ID       string         `json:"id"`
	Device   DeviceResponse `json:"dispositivo"`
	LastUsed time.Time      `json:"ultimo_uso"`
	Current  bool           `json:"actual"`
}

// VerifyEmailResponse es la respuesta de POST /auth/verificar-correo.
type VerifyEmailResponse struct {
	EmailVerified bool `json:"correo_verificado"`
}
