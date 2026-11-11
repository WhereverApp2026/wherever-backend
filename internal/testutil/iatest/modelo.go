// Package iatest tiene el doble de ia.Modelo para las pruebas: el proveedor de IA es una
// interfaz externa (AGENTS §3) y se sustituye; el resto de la app es real.
package iatest

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	espera     time.Duration
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
	m.respuestas, m.peticiones, m.espera = nil, nil, 0
	m.ultima = resultado{json: []byte(`{}`)}
}

// Peticiones devuelve una copia de lo que recibió el doble.
func (m *Modelo) Peticiones() []ia.Peticion {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]ia.Peticion(nil), m.peticiones...)
}

// Tardar hace que cada llamada espere d antes de responder, o hasta que se cancele el
// contexto (como el adaptador real): para probar el plazo y la cancelación.
func (m *Modelo) Tardar(d time.Duration) *Modelo {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.espera = d
	return m
}

// Generar cumple ia.Modelo.
func (m *Modelo) Generar(ctx context.Context, pet ia.Peticion) (ia.Respuesta, error) {
	m.mu.Lock()
	m.peticiones = append(m.peticiones, pet)
	espera := m.espera
	m.mu.Unlock()
	if espera > 0 {
		t := time.NewTimer(espera)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ia.Respuesta{}, fmt.Errorf("%w: %w", ia.ErrTiempo, ctx.Err())
		case <-t.C:
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
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
