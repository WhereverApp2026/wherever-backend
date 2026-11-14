// Package stripetest es el doble de la API de Stripe (nv-60): un httptest.Server con las rutas
// que usa pagos.StripeAPI, que responde como Stripe, registra cada llamada con su
// `Idempotency-Key` (y repite la respuesta de una llave ya usada, como Stripe) y firma eventos
// del webhook con un secreto de prueba. Es el único doble del módulo: MongoDB y Redis son reales
// (constitución: mocks solo de interfaces externas). Nunca toca la API real.
package stripetest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v86/webhook"
)

// Llaves de prueba (no son de Stripe: solo valen contra este doble).
const (
	SecretKey     = "sk_test_doble"         // #nosec G101 -- llave del doble, no de Stripe
	WebhookSecret = "whsec_doble_de_prueba" // #nosec G101 -- secreto del doble, no de Stripe
)

// Call es una petición recibida.
type Call struct {
	Method         string
	Path           string
	IdempotencyKey string
	Form           url.Values
}

type intent struct {
	ID       string
	Amount   int64
	Status   string
	Metadata map[string]string
}

// Server es el doble de Stripe.
type Server struct {
	*httptest.Server
	mu      sync.Mutex
	calls   []Call
	idem    map[string][]byte
	intents map[string]*intent
	seq     int
	// failNext hace fallar la siguiente escritura con 500 (Stripe caído).
	failNext bool
}

// New levanta el doble y lo cierra al terminar la prueba.
func New(t testing.TB) *Server {
	t.Helper()
	s := &Server{idem: map[string][]byte{}, intents: map[string]*intent{}}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)
	return s
}

// Reset borra llamadas y estado.
func (s *Server) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls, s.idem, s.intents, s.failNext = nil, map[string][]byte{}, map[string]*intent{}, false
}

// FailNext hace que la próxima escritura responda 500.
func (s *Server) FailNext() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failNext = true
}

// Calls devuelve las llamadas cuyo método y ruta empiezan así.
func (s *Server) Calls(method, prefix string) []Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Call
	for _, c := range s.calls {
		if c.Method == method && strings.HasPrefix(c.Path, prefix) {
			out = append(out, c)
		}
	}
	return out
}

// SetStatus cambia el estado de un PaymentIntent (lo que la app logró con la PaymentSheet).
func (s *Server) SetStatus(id, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pi, ok := s.intents[id]; ok {
		pi.Status = status
	}
}

func (s *Server) next(prefix string) string {
	s.seq++
	return fmt.Sprintf("%s_doble%06d", prefix, s.seq)
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	s.mu.Lock()
	defer s.mu.Unlock()
	llave := r.Header.Get("Idempotency-Key")
	s.calls = append(s.calls, Call{Method: r.Method, Path: r.URL.Path, IdempotencyKey: llave, Form: r.PostForm})
	if r.Header.Get("Authorization") != "Bearer "+SecretKey {
		writeJSON(w, http.StatusUnauthorized, stripeError("authentication_required"))
		return
	}
	if r.Method == http.MethodPost && llave != "" {
		if prev, ok := s.idem[r.URL.Path+"|"+llave]; ok {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(prev)
			return
		}
	}
	if r.Method == http.MethodPost && s.failNext {
		s.failNext = false
		writeJSON(w, http.StatusInternalServerError, stripeError("api_error"))
		return
	}
	status, body := s.route(r)
	raw, _ := json.Marshal(body)
	if r.Method == http.MethodPost && llave != "" && status == http.StatusOK {
		s.idem[r.URL.Path+"|"+llave] = raw
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

func (s *Server) route(r *http.Request) (int, any) {
	path, f := r.URL.Path, r.PostForm
	amount, _ := strconv.ParseInt(f.Get("amount"), 10, 64)
	if r.Method == http.MethodGet {
		return s.leerIntent(path)
	}
	switch path {
	case "/v1/payment_intents":
		pi := &intent{ID: s.next("pi"), Amount: amount, Status: "requires_payment_method", Metadata: metadata(f)}
		s.intents[pi.ID] = pi
		return http.StatusOK, intentJSON(pi)
	case "/v1/refunds":
		return http.StatusOK, map[string]any{"id": s.next("re"), "object": "refund", "amount": amount, "payment_intent": f.Get("payment_intent"), "status": "pending"}
	case "/v1/accounts":
		return http.StatusOK, map[string]any{"id": s.next("acct"), "object": "account", "type": f.Get("type"), "country": f.Get("country")}
	case "/v1/account_links":
		return http.StatusOK, map[string]any{"object": "account_link", "url": "https://connect.stripe.com/setup/e/" + f.Get("account")}
	case "/v1/transfers":
		return http.StatusOK, map[string]any{"id": s.next("tr"), "object": "transfer", "amount": amount, "destination": f.Get("destination")}
	}
	if strings.HasPrefix(path, "/v1/accounts/") && strings.HasSuffix(path, "/external_accounts") {
		return s.externa(f)
	}
	return http.StatusNotFound, stripeError("resource_missing")
}

// leerIntent responde GET /v1/payment_intents/{id}.
func (s *Server) leerIntent(path string) (int, any) {
	pi, ok := s.intents[strings.TrimPrefix(path, "/v1/payment_intents/")]
	if !ok || !strings.HasPrefix(path, "/v1/payment_intents/") {
		return http.StatusNotFound, stripeError("resource_missing")
	}
	return http.StatusOK, intentJSON(pi)
}

// externa registra la cuenta bancaria; responde solo los últimos 4.
func (s *Server) externa(f url.Values) (int, any) {
	numero := f.Get("external_account[account_number]")
	if numero == "" {
		numero = f.Get("account_number")
	}
	if len(numero) < 4 {
		return http.StatusBadRequest, stripeError("parameter_missing")
	}
	return http.StatusOK, map[string]any{"id": s.next("ba"), "object": "bank_account", "last4": numero[len(numero)-4:]}
}

func metadata(f url.Values) map[string]string {
	out := map[string]string{}
	for k, v := range f {
		if strings.HasPrefix(k, "metadata[") && len(v) > 0 {
			out[strings.TrimSuffix(strings.TrimPrefix(k, "metadata["), "]")] = v[0]
		}
	}
	return out
}

// intentJSON responde como Stripe; un PaymentIntent cobrado trae `latest_charge` expandido con
// una Visa terminada en 4242.
func intentJSON(pi *intent) map[string]any {
	out := map[string]any{
		"id": pi.ID, "object": "payment_intent", "amount": pi.Amount, "currency": "mxn", "status": pi.Status,
		"client_secret": pi.ID + "_secret_doble", "metadata": pi.Metadata,
	}
	if pi.Status == "succeeded" {
		out["latest_charge"] = map[string]any{
			"id": "ch_" + pi.ID, "object": "charge",
			"payment_method_details": map[string]any{"type": "card", "card": map[string]any{"brand": "visa", "last4": "4242"}},
		}
	}
	return out
}

func stripeError(code string) map[string]any {
	return map[string]any{"error": map[string]any{"type": "api_error", "code": code, "message": code}}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Event arma un evento de Stripe con su objeto.
func Event(id, tipo string, object any) []byte {
	raw, _ := json.Marshal(map[string]any{
		"id": id, "object": "event", "type": tipo, "api_version": "2026-08-26.dahlia", "created": time.Now().Unix(),
		"data": map[string]any{"object": object},
	})
	return raw
}

// SignedEvent firma el evento con el secreto de prueba en el momento `at` (cero = ahora).
func SignedEvent(payload []byte, secret string, at time.Time) string {
	return webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: secret, Timestamp: at}).Header
}

// Succeeded es el objeto de `payment_intent.succeeded`.
func Succeeded(intentID string) map[string]any {
	return map[string]any{"id": intentID, "object": "payment_intent", "status": "succeeded", "latest_charge": "ch_" + intentID}
}

// Failed es el objeto de `payment_intent.payment_failed`.
func Failed(intentID, mensaje string) map[string]any {
	return map[string]any{"id": intentID, "object": "payment_intent", "status": "requires_payment_method",
		"last_payment_error": map[string]any{"message": mensaje, "code": "card_declined"}}
}

// Refunded es el objeto de `charge.refunded` con los reembolsos dados.
func Refunded(intentID string, amount int64, refunds ...Refund) map[string]any {
	var total int64
	data := make([]map[string]any, 0, len(refunds))
	for _, r := range refunds {
		total += r.Amount
		data = append(data, map[string]any{"id": r.ID, "amount": r.Amount, "created": time.Now().Unix(), "status": "succeeded"})
	}
	return map[string]any{"id": "ch_" + intentID, "object": "charge", "payment_intent": intentID, "amount": amount,
		"amount_refunded": total, "refunds": map[string]any{"object": "list", "data": data}}
}

// Refund es un reembolso dentro de un cargo.
type Refund struct {
	ID     string
	Amount int64
}

// AccountUpdated es el objeto de `account.updated`; activa = transferencias y depósitos listos.
func AccountUpdated(account string, activa bool) map[string]any {
	transfers := "inactive"
	if activa {
		transfers = "active"
	}
	return map[string]any{"id": account, "object": "account", "payouts_enabled": activa, "capabilities": map[string]any{"transfers": transfers},
		"requirements": map[string]any{"disabled_reason": nil}}
}
