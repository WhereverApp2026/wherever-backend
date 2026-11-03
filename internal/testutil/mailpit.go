package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Mailpit lee los correos que recibió el servidor de prueba por su API web (nv-06).
type Mailpit struct {
	API      string
	SMTPHost string
	SMTPPort string
}

// MailpitMessage es un correo recibido.
type MailpitMessage struct {
	ID      string
	To      string
	Subject string
	Text    string
	HTML    string
}

// Port devuelve el puerto SMTP como entero.
func (m *Mailpit) Port() int {
	p, _ := strconv.Atoi(m.SMTPPort)
	return p
}

// Clear borra todos los correos.
func (m *Mailpit) Clear(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, m.API+"/api/v1/messages", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// Messages devuelve los correos recibidos para ese destinatario, con su texto y HTML.
func (m *Mailpit) Messages(ctx context.Context, to string) ([]MailpitMessage, error) {
	var list struct {
		Messages []struct {
			ID      string `json:"ID"`
			Subject string `json:"Subject"`
			To      []struct {
				Address string `json:"Address"`
			} `json:"To"`
		} `json:"messages"`
	}
	if err := m.get(ctx, "/api/v1/messages", &list); err != nil {
		return nil, err
	}
	var out []MailpitMessage
	for _, item := range list.Messages {
		if len(item.To) == 0 || !strings.EqualFold(item.To[0].Address, to) {
			continue
		}
		var full struct {
			Text string `json:"Text"`
			HTML string `json:"HTML"`
		}
		if err := m.get(ctx, "/api/v1/message/"+item.ID, &full); err != nil {
			return nil, err
		}
		out = append(out, MailpitMessage{ID: item.ID, To: item.To[0].Address, Subject: item.Subject, Text: full.Text, HTML: full.HTML})
	}
	return out, nil
}

// WaitFor espera hasta `timeout` a que lleguen `n` correos para ese destinatario.
func (m *Mailpit) WaitFor(ctx context.Context, to string, n int, timeout time.Duration) ([]MailpitMessage, error) {
	deadline := time.Now().Add(timeout)
	for {
		msgs, err := m.Messages(ctx, to)
		if err != nil {
			return nil, err
		}
		if len(msgs) >= n || time.Now().After(deadline) {
			return msgs, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (m *Mailpit) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.API+path, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mailpit %s: %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
