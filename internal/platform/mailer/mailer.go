// Package mailer define el envío de correos de la API. El proveedor real (SMTP o servicio)
// es deuda de nv-04: hoy existen el buzón en memoria para pruebas y el Log para desarrollo.
package mailer

import "context"

// Kind es el tipo de correo; coincide con el `tipo` de tokens_un_uso.
type Kind string

// Tipos de correo de la hoja J1.
const (
	KindPasswordReset Kind = "recuperar_password" // #nosec G101 -- tipo de correo, no una credencial
	KindVerifyEmail   Kind = "verificar_correo"
	KindExport        Kind = "descargar_exportacion" // D-2 (nv-05): enlace de descarga de "mis datos"
)

// Message es un correo con el token del enlace; el proveedor arma el enlace.
type Message struct {
	To    string
	Kind  Kind
	Token string
}

// Mailer entrega correos.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}
