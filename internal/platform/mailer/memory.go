package mailer

import (
	"context"
	"sync"
)

// Memory es un buzón en memoria para pruebas; seguro entre goroutines.
type Memory struct {
	mu   sync.Mutex
	sent []Message
}

// NewMemory crea un buzón vacío.
func NewMemory() *Memory { return &Memory{} }

// Send guarda el correo.
func (m *Memory) Send(_ context.Context, msg Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	return nil
}

// Sent devuelve una copia de los correos enviados.
func (m *Memory) Sent() []Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Message(nil), m.sent...)
}

// Last devuelve el último correo de ese tipo para ese destinatario.
func (m *Memory) Last(to string, kind Kind) (Message, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.sent) - 1; i >= 0; i-- {
		if m.sent[i].To == to && m.sent[i].Kind == kind {
			return m.sent[i], true
		}
	}
	return Message{}, false
}

// Reset vacía el buzón.
func (m *Memory) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = nil
}
