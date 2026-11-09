// Package ia es la IA generativa de la API (spec nv-26). Los llamadores dependen solo de la
// interfaz Modelo; el adaptador vigente es Gemini y el de Bedrock queda como opción futura.
package ia

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Modelo genera una salida JSON que cumple el esquema de la petición.
type Modelo interface {
	Generar(ctx context.Context, pet Peticion) (Respuesta, error)
	// Nombre identifica proveedor y modelo para registrar el uso, nunca la llave.
	Nombre() string
}

// Peticion separa siempre las instrucciones fijas de lo que viene del usuario (nv-28).
type Peticion struct {
	// Instrucciones son fijas y versionadas; nunca llevan texto del usuario.
	Instrucciones string
	// Contexto lo arma la API (candidatos, franjas, presupuesto); sin datos personales.
	Contexto string
	// TextoUsuario es la frase libre. El adaptador la anonimiza antes de enviarla.
	TextoUsuario string
	// Esquema es el JSON Schema de la salida.
	Esquema map[string]any
	// MaxTokens limita la salida; 0 usa el valor del adaptador.
	MaxTokens int
}

// Respuesta es la salida JSON del modelo y su consumo, para el tope de gasto (nv-27).
type Respuesta struct {
	JSON          []byte
	TokensEntrada int
	TokensSalida  int
	Duracion      time.Duration
}

// Errores que el llamador distingue. Con cualquiera de ellos responde el motor de reglas (nv-33).
var (
	// ErrLimiteProveedor: el proveedor respondió 429. No se reintenta.
	ErrLimiteProveedor = errors.New("ia: límite del proveedor")
	// ErrTiempo: se venció el plazo de la llamada.
	ErrTiempo = errors.New("ia: tiempo agotado")
	// ErrProveedor: error del servicio o de configuración (5xx, 4xx que no es 429).
	ErrProveedor = errors.New("ia: error del proveedor")
	// ErrRespuestaInvalida: la salida no es JSON o no cumple el esquema.
	ErrRespuestaInvalida = errors.New("ia: respuesta inválida")
	// ErrBloqueada: el proveedor bloqueó la entrada o la salida por sus filtros.
	ErrBloqueada = errors.New("ia: respuesta bloqueada por el proveedor")
)

// LimiteError es un 429 del proveedor con la espera que sugiere, si la dio.
type LimiteError struct {
	Espera time.Duration
}

func (e *LimiteError) Error() string {
	if e.Espera > 0 {
		return fmt.Sprintf("%s; reintentar en %s", ErrLimiteProveedor, e.Espera)
	}
	return ErrLimiteProveedor.Error()
}

// Is hace que errors.Is(err, ErrLimiteProveedor) funcione.
func (e *LimiteError) Is(target error) bool { return target == ErrLimiteProveedor }
