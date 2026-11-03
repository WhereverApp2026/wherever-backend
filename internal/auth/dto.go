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

// RefreshRequest es el cuerpo de POST /auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required,max=128"`
}

// RecoverRequest es el cuerpo de POST /auth/recuperar.
type RecoverRequest struct {
	Email string `json:"correo" validate:"required,email,max=254"`
}

// ResetRequest es el cuerpo de POST /auth/restablecer. `dispositivo` es opcional: la
// sesión que se abre al restablecer lo registra si llega.
type ResetRequest struct {
	Token       string       `json:"token" validate:"required,max=128"`
	NewPassword string       `json:"password_nuevo" validate:"required,password"`
	Device      *DeviceInput `json:"dispositivo" validate:"omitempty"`
}

// VerifyEmailRequest es el cuerpo de POST /auth/verificar-correo.
type VerifyEmailRequest struct {
	Token string `json:"token" validate:"required,max=128"`
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
