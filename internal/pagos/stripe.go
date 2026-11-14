package pagos

import (
	"context"
	"errors"
	"net/http"
	"time"

	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"
)

// StripeAPI es la única frontera con Stripe (interfaz externa; en pruebas, el doble de
// internal/testutil/stripetest). Cada escritura lleva su llave de idempotencia.
type StripeAPI interface {
	CreatePaymentIntent(ctx context.Context, p IntentParams) (Intent, error)
	GetPaymentIntent(ctx context.Context, id string) (Intent, error)
	CreateRefund(ctx context.Context, p RefundParams) (string, error)
	CreateAccount(ctx context.Context, negocioID, idem string) (string, error)
	CreateAccountLink(ctx context.Context, account, refreshURL, returnURL string) (string, error)
	CreateTransfer(ctx context.Context, p TransferParams) (string, error)
	CreateExternalAccount(ctx context.Context, account, clabe, titular, idem string) (string, error)
}

// IntentParams crea el PaymentIntent del anticipo.
type IntentParams struct {
	MontoCentavos int64
	Metadata      map[string]string
	Idem          string
}

// Intent es lo que se lee de un PaymentIntent; `Tarjeta` sale de `latest_charge`.
type Intent struct {
	ID           string
	ClientSecret string
	Status       string
	Monto        int64
	Metadata     map[string]string
	Tarjeta      *Tarjeta
	Error        string
}

// RefundParams pide un reembolso del PaymentIntent.
type RefundParams struct {
	PaymentIntent string
	MontoCentavos int64
	Idem          string
	Metadata      map[string]string
}

// TransferParams transfiere la liquidación a la cuenta de Connect del negocio.
type TransferParams struct {
	Destino       string
	MontoCentavos int64
	Grupo         string
	Idem          string
}

// Tolerancia de la firma del webhook.
const Tolerancia = 5 * time.Minute

// stripeTimeout acota cada llamada: el webhook debe responder en menos de 10 s.
const stripeTimeout = 8 * time.Second

// Client implementa StripeAPI con stripe-go.
type Client struct{ sc *stripe.Client }

// NewClient arma el cliente; `baseURL` vacío = la API real (STRIPE_API_URL solo en pruebas).
func NewClient(secretKey, baseURL string) *Client {
	retries := int64(1)
	cfg := &stripe.BackendConfig{
		HTTPClient:        &http.Client{Timeout: stripeTimeout},
		MaxNetworkRetries: &retries,
		LeveledLogger:     &stripe.LeveledLogger{Level: stripe.LevelNull},
	}
	if baseURL != "" {
		cfg.URL = stripe.String(baseURL)
	}
	return &Client{sc: stripe.NewClient(secretKey, stripe.WithBackends(stripe.NewBackendsWithConfig(cfg)))}
}

// CreatePaymentIntent crea el cobro en MXN con los métodos automáticos (la PaymentSheet).
func (c *Client) CreatePaymentIntent(ctx context.Context, p IntentParams) (Intent, error) {
	params := &stripe.PaymentIntentCreateParams{
		Amount:                  stripe.Int64(p.MontoCentavos),
		Currency:                stripe.String(Moneda),
		AutomaticPaymentMethods: &stripe.PaymentIntentCreateAutomaticPaymentMethodsParams{Enabled: stripe.Bool(true)},
		Metadata:                p.Metadata,
	}
	params.SetIdempotencyKey(p.Idem)
	pi, err := c.sc.V1PaymentIntents.Create(ctx, params)
	if err != nil {
		return Intent{}, err
	}
	return intentDe(pi), nil
}

// GetPaymentIntent lee el PaymentIntent con `latest_charge` expandido (marca y últimos 4).
func (c *Client) GetPaymentIntent(ctx context.Context, id string) (Intent, error) {
	params := &stripe.PaymentIntentRetrieveParams{}
	params.AddExpand("latest_charge")
	pi, err := c.sc.V1PaymentIntents.Retrieve(ctx, id, params)
	if err != nil {
		return Intent{}, err
	}
	return intentDe(pi), nil
}

func intentDe(pi *stripe.PaymentIntent) Intent {
	out := Intent{ID: pi.ID, ClientSecret: pi.ClientSecret, Status: string(pi.Status), Monto: pi.Amount, Metadata: pi.Metadata}
	if pi.LastPaymentError != nil {
		out.Error = pi.LastPaymentError.Msg
	}
	if ch := pi.LatestCharge; ch != nil && ch.PaymentMethodDetails != nil && ch.PaymentMethodDetails.Card != nil {
		out.Tarjeta = &Tarjeta{Marca: string(ch.PaymentMethodDetails.Card.Brand), Ultimos4: ch.PaymentMethodDetails.Card.Last4}
	}
	return out
}

// CreateRefund pide el reembolso. Con cargos y transferencias separados el reembolso siempre
// ocurre antes del corte semanal (solo se cancela una reserva futura), así que no hay
// transferencia que revertir.
func (c *Client) CreateRefund(ctx context.Context, p RefundParams) (string, error) {
	params := &stripe.RefundCreateParams{PaymentIntent: stripe.String(p.PaymentIntent), Amount: stripe.Int64(p.MontoCentavos), Metadata: p.Metadata}
	params.SetIdempotencyKey(p.Idem)
	r, err := c.sc.V1Refunds.Create(ctx, params)
	if err != nil {
		return "", err
	}
	return r.ID, nil
}

// CreateAccount crea la cuenta Express de México que recibe las transferencias.
func (c *Client) CreateAccount(ctx context.Context, negocioID, idem string) (string, error) {
	params := &stripe.AccountCreateParams{
		Type:         stripe.String("express"),
		Country:      stripe.String("MX"),
		Capabilities: &stripe.AccountCreateCapabilitiesParams{Transfers: &stripe.AccountCreateCapabilitiesTransfersParams{Requested: stripe.Bool(true)}},
		Metadata:     map[string]string{"negocio_id": negocioID},
	}
	params.SetIdempotencyKey(idem)
	a, err := c.sc.V1Accounts.Create(ctx, params)
	if err != nil {
		return "", err
	}
	return a.ID, nil
}

// CreateAccountLink da el enlace de registro (KYC) de Stripe.
func (c *Client) CreateAccountLink(ctx context.Context, account, refreshURL, returnURL string) (string, error) {
	l, err := c.sc.V1AccountLinks.Create(ctx, &stripe.AccountLinkCreateParams{
		Account: stripe.String(account), RefreshURL: stripe.String(refreshURL), ReturnURL: stripe.String(returnURL),
		Type: stripe.String("account_onboarding"),
	})
	if err != nil {
		return "", err
	}
	return l.URL, nil
}

// CreateTransfer transfiere la liquidación semanal.
func (c *Client) CreateTransfer(ctx context.Context, p TransferParams) (string, error) {
	params := &stripe.TransferCreateParams{
		Amount: stripe.Int64(p.MontoCentavos), Currency: stripe.String(Moneda), Destination: stripe.String(p.Destino), TransferGroup: stripe.String(p.Grupo),
	}
	params.SetIdempotencyKey(p.Idem)
	t, err := c.sc.V1Transfers.Create(ctx, params)
	if err != nil {
		return "", err
	}
	return t.ID, nil
}

// CreateExternalAccount registra la CLABE como cuenta externa de la cuenta de Connect.
func (c *Client) CreateExternalAccount(ctx context.Context, account, clabe, titular, idem string) (string, error) {
	params := &stripe.BankAccountCreateParams{
		Account: stripe.String(account), AccountNumber: stripe.String(clabe), AccountHolderName: stripe.String(titular),
		Country: stripe.String("MX"), Currency: stripe.String(Moneda), DefaultForCurrency: stripe.Bool(true),
	}
	params.SetIdempotencyKey(idem)
	b, err := c.sc.V1BankAccounts.Create(ctx, params)
	if err != nil {
		return "", err
	}
	return b.ID, nil
}

// ErrFirma es una firma ausente, mala o vieja.
var ErrFirma = errors.New("firma de Stripe inválida")

// VerificarFirma valida `Stripe-Signature` sobre el cuerpo crudo, con 5 minutos de tolerancia.
func VerificarFirma(payload []byte, header, secret string) error {
	if header == "" || secret == "" {
		return ErrFirma
	}
	if err := webhook.ValidatePayloadWithTolerance(payload, header, secret, Tolerancia); err != nil {
		return ErrFirma
	}
	return nil
}

// mensajeStripe da el mensaje de un error de Stripe sin datos de tarjeta (solo su texto).
func mensajeStripe(err error) string {
	var se *stripe.Error
	if errors.As(err, &se) {
		return string(se.Code)
	}
	return "stripe"
}
