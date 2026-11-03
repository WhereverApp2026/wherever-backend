// Package mailer envía los correos de la API (spec nv-06): adaptador SMTP genérico, cola en
// segundo plano con reintentos, plantillas en español y enlaces con el token en el fragmento.
// El buzón en memoria (pruebas) y el Log (dev y staging sin servidor) cumplen la misma interfaz.
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

// Message es un correo con el token del enlace. El adaptador arma el enlace y el cuerpo;
// Name es el nombre de pila para el saludo.
type Message struct {
	To    string
	Name  string
	Kind  Kind
	Token string
}

// Mailer entrega correos.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}
