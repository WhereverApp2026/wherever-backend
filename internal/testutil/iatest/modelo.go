// Package iatest tiene el doble de ia.Modelo para las pruebas: el proveedor de IA es una
// interfaz externa (AGENTS §3) y se sustituye; el resto de la app es real.
package iatest

import (
	"context"
	"sync"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
)

// Modelo responde en orden lo programado con Responder/Fallar y guarda cada petición para
// revisar qué viajó al proveedor. Sin respuestas programadas repite la última. Es seguro
// entre goroutines.
type Modelo struct {
	mu         sync.Mutex
	respuestas []resultado
	ultima     resultado
	peticiones []ia.Peticion
}

type resultado struct {
	json []byte
	err  error
}

// Nuevo arma un doble que, sin programar, responde `{}` (inválido para cualquier esquema).
func Nuevo() *Modelo { return &Modelo{ultima: resultado{json: []byte(`{}`)}} }

// Responder programa una respuesta JSON.
func (m *Modelo) Responder(json string) *Modelo {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.respuestas = append(m.respuestas, resultado{json: []byte(json)})
	return m
}

// Fallar programa un error del proveedor.
func (m *Modelo) Fallar(err error) *Modelo {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.respuestas = append(m.respuestas, resultado{err: err})
	return m
}

// Reset borra lo programado y las peticiones guardadas.
func (m *Modelo) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.respuestas, m.peticiones = nil, nil
	m.ultima = resultado{json: []byte(`{}`)}
}

// Peticiones devuelve una copia de lo que recibió el doble.
func (m *Modelo) Peticiones() []ia.Peticion {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]ia.Peticion(nil), m.peticiones...)
}

// Generar cumple ia.Modelo.
func (m *Modelo) Generar(ctx context.Context, pet ia.Peticion) (ia.Respuesta, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.peticiones = append(m.peticiones, pet)
	if err := ctx.Err(); err != nil {
		return ia.Respuesta{}, err
	}
	if len(m.respuestas) > 0 {
		m.ultima, m.respuestas = m.respuestas[0], m.respuestas[1:]
	}
	if m.ultima.err != nil {
		return ia.Respuesta{}, m.ultima.err
	}
	return ia.Respuesta{JSON: m.ultima.json, TokensEntrada: 100, TokensSalida: 40}, nil
}

// Nombre cumple ia.Modelo.
func (m *Modelo) Nombre() string { return "doble" }
