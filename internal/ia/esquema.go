package ia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Validable es una salida que sabe revisar sus propias reglas, como que cada id esté en la
// lista de candidatos que la API entregó.
type Validable interface {
	Validar() error
}

// Decodificar lee la salida con reglas estrictas: un solo objeto JSON, sin campos que el
// esquema no declara y sin texto extra. Después corre Validar si el destino la tiene.
func Decodificar(data []byte, destino any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(destino); err != nil {
		return fmt.Errorf("%w: %v", ErrRespuestaInvalida, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: texto después del JSON", ErrRespuestaInvalida)
	}
	if v, ok := destino.(Validable); ok {
		if err := v.Validar(); err != nil {
			return fmt.Errorf("%w: %v", ErrRespuestaInvalida, err)
		}
	}
	return nil
}

// GenerarValidado llama al modelo y decodifica la salida en destino. Una salida inválida se
// descarta y se reintenta una sola vez; un límite, un plazo o un error del proveedor no se
// reintentan. Devuelve el consumo de todas las llamadas hechas, para el gasto.
func GenerarValidado(ctx context.Context, m Modelo, pet Peticion, destino any) (Respuesta, error) {
	var total Respuesta
	var err error
	for intento := 0; intento < 2; intento++ {
		var r Respuesta
		r, err = m.Generar(ctx, pet)
		total.TokensEntrada += r.TokensEntrada
		total.TokensSalida += r.TokensSalida
		total.Duracion += r.Duracion
		if err == nil {
			total.JSON = r.JSON
			err = Decodificar(r.JSON, destino)
		}
		if err == nil || !errors.Is(err, ErrRespuestaInvalida) || ctx.Err() != nil {
			return total, err
		}
	}
	return total, err
}
