package ia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
)

// Validable es una salida que sabe revisar sus propias reglas, como que cada id esté en la
// lista de candidatos que la API entregó.
type Validable interface {
	Validar() error
}

// SoloDeLaLista revisa que cada id elegido esté en la lista que la API entregó y no se repita
// (nv-28): aunque una frase con instrucciones engañe al modelo, no puede inventar ids ni traer
// otros. Las salidas lo llaman desde su Validar.
func SoloDeLaLista(elegidos, permitidos []string) error {
	lista := make(map[string]bool, len(permitidos))
	for _, id := range permitidos {
		lista[id] = true
	}
	vistos := make(map[string]bool, len(elegidos))
	for _, id := range elegidos {
		if !lista[id] {
			return fmt.Errorf("id %q fuera de la lista", id)
		}
		if vistos[id] {
			return fmt.Errorf("id %q repetido", id)
		}
		vistos[id] = true
	}
	return nil
}

// Decodificar lee la salida con reglas estrictas: un solo objeto JSON, sin campos que el
// esquema no declara y sin texto extra. Después corre Validar si el destino la tiene. Si
// falla, deja el destino en cero: una salida rechazada (un id inyectado) no queda a mano.
func Decodificar(data []byte, destino any) (err error) {
	defer func() {
		if err != nil {
			limpiar(destino)
		}
	}()
	return decodificar(data, destino)
}

func limpiar(destino any) {
	if v := reflect.ValueOf(destino); v.Kind() == reflect.Pointer && !v.IsNil() {
		v.Elem().SetZero()
	}
}

func decodificar(data []byte, destino any) error {
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
