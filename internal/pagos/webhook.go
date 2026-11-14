package pagos

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Tipos de evento que se aplican; cualquier otro responde 200 sin hacer nada.
const (
	EventoAprobado    = "payment_intent.succeeded"
	EventoProcesando  = "payment_intent.processing"
	EventoRechazado   = "payment_intent.payment_failed"
	EventoReembolsado = "charge.refunded"
	EventoCuenta      = "account.updated"
)

// evento es lo que se lee del JSON de Stripe, solo después de verificar la firma.
type evento struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type intentObj struct {
	ID               string            `json:"id"`
	Metadata         map[string]string `json:"metadata"`
	LastPaymentError *struct {
		Message string `json:"message"`
	} `json:"last_payment_error"`
}

type chargeObj struct {
	PaymentIntent  string `json:"payment_intent"`
	Amount         int64  `json:"amount"`
	AmountRefunded int64  `json:"amount_refunded"`
	Refunds        struct {
		Data []struct {
			ID       string            `json:"id"`
			Amount   int64             `json:"amount"`
			Created  int64             `json:"created"`
			Status   string            `json:"status"`
			Metadata map[string]string `json:"metadata"`
		} `json:"data"`
	} `json:"refunds"`
}

type accountObj struct {
	ID             string `json:"id"`
	PayoutsEnabled bool   `json:"payouts_enabled"`
	Capabilities   struct {
		Transfers string `json:"transfers"`
	} `json:"capabilities"`
	Requirements struct {
		DisabledReason string `json:"disabled_reason"`
	} `json:"requirements"`
}

// Webhook verifica la firma sobre el cuerpo crudo y aplica el evento una sola vez: primero
// `idem:stripe_evento:{id}` (SET NX) y después `eventos_procesados` en el pago. Firma mala = 400
// sin detalle; Redis caído o falla al aplicar = 503 para que Stripe reintente.
func (s *Service) Webhook(ctx context.Context, payload []byte, firma string) error {
	if s.cfg.WebhookSecret == "" || s.stripe == nil {
		return errNoDisponible()
	}
	if err := VerificarFirma(payload, firma, s.cfg.WebhookSecret); err != nil {
		return apperr.BadRequest("firma inválida")
	}
	var ev evento
	if err := json.Unmarshal(payload, &ev); err != nil || ev.ID == "" {
		return apperr.BadRequest("evento inválido")
	}
	if !aplicable(ev.Type) {
		return nil
	}
	nuevo, err := s.idem.TomarEvento(ctx, ev.ID)
	if err != nil {
		return errStore()
	}
	if !nuevo {
		return nil
	}
	if err := s.aplicar(ctx, ev); err != nil {
		s.idem.SoltarEvento(ctx, ev.ID)
		s.log.Warn("pagos: el evento de Stripe no se aplicó; Stripe lo reintenta", "evento", "stripe_evento_fallo", "tipo", ev.Type, "error", err.Error())
		return errStore()
	}
	return nil
}

func aplicable(tipo string) bool {
	switch tipo {
	case EventoAprobado, EventoProcesando, EventoRechazado, EventoReembolsado, EventoCuenta:
		return true
	}
	return false
}

func (s *Service) aplicar(ctx context.Context, ev evento) error {
	switch ev.Type {
	case EventoCuenta:
		var a accountObj
		if err := json.Unmarshal(ev.Data.Object, &a); err != nil {
			return err
		}
		return s.repo.EstadoCobros(ctx, a.ID, estadoCuenta(a))
	case EventoReembolsado:
		var ch chargeObj
		if err := json.Unmarshal(ev.Data.Object, &ch); err != nil {
			return err
		}
		return s.reembolsado(ctx, ev.ID, ch)
	}
	var obj intentObj
	if err := json.Unmarshal(ev.Data.Object, &obj); err != nil {
		return err
	}
	p, err := s.repo.FindByIntent(ctx, obj.ID)
	if errors.Is(err, ErrNotFound) {
		// Un PaymentIntent que no es de un anticipo (p. ej., uno creado a mano): nada que hacer.
		return nil
	}
	if err != nil {
		return err
	}
	return s.aplicarIntent(ctx, ev, p, obj)
}

// aplicarIntent aplica los eventos del PaymentIntent de un pago.
func (s *Service) aplicarIntent(ctx context.Context, ev evento, p Pago, obj intentObj) error {
	var err error
	switch ev.Type {
	case EventoAprobado:
		intent, errGet := s.stripe.GetPaymentIntent(ctx, obj.ID)
		if errGet != nil {
			return errGet
		}
		_, err = s.aprobar(ctx, p, intent, ev.ID)
		return err
	case EventoProcesando:
		_, err = s.repo.Aplicar(ctx, p.ID, []string{EstadoRequierePago}, ev.ID, bson.D{{Key: "estado", Value: EstadoProcesando}}, nil)
	default:
		motivo := "pago rechazado"
		if obj.LastPaymentError != nil && obj.LastPaymentError.Message != "" {
			motivo = obj.LastPaymentError.Message
		}
		_, err = s.repo.Aplicar(ctx, p.ID, []string{EstadoRequierePago, EstadoProcesando}, ev.ID,
			bson.D{{Key: "estado", Value: EstadoRechazado}, {Key: "motivo_rechazo", Value: motivo}}, nil)
	}
	if errors.Is(err, ErrNoAplica) {
		return nil
	}
	return err
}

// aprobar deja el pago `aprobado` con la marca y los últimos 4, y confirma la reserva. Si el
// pago llegó tarde y el cupo se perdió, reembolso completo automático.
func (s *Service) aprobar(ctx context.Context, p Pago, intent Intent, eventoID string) (Pago, error) {
	set := bson.D{{Key: "estado", Value: EstadoAprobado}}
	if intent.Tarjeta != nil {
		set = append(set, bson.E{Key: "tarjeta", Value: *intent.Tarjeta})
	}
	aprobado, err := s.repo.Aplicar(ctx, p.ID, []string{EstadoRequierePago, EstadoProcesando, EstadoRechazado}, eventoID, set, nil)
	if errors.Is(err, ErrNoAplica) {
		return p, nil
	}
	if err != nil {
		return Pago{}, err
	}
	_, err = s.reservas.ConfirmarPago(ctx, p.ReservaID, p.ID.Hex())
	switch {
	case errors.Is(err, reservas.ErrCupoPerdido):
		s.log.Info("pagos: pago tardío sin cupo; reembolso completo", "evento", "pago_cupo_perdido")
		return aprobado, s.reembolsar(ctx, aprobado, aprobado.MontoCentavos, reservas.MotivoCupoPerdido)
	case errors.Is(err, reservas.ErrNoAplica), errors.Is(err, reservas.ErrNotFound):
		// La reserva ya se canceló o no existe: se devuelve todo.
		r, errR := s.reservas.FindByID(ctx, p.ReservaID)
		if errR == nil && r.PagoID == p.ID.Hex() && r.Estado != reservas.EstadoCancelada {
			return aprobado, nil
		}
		return aprobado, s.reembolsar(ctx, aprobado, aprobado.MontoCentavos, "reserva_no_disponible")
	case err != nil:
		return Pago{}, err
	}
	return aprobado, nil
}

// reembolsado concilia los reembolsos con lo que reporta Stripe en el cargo.
func (s *Service) reembolsado(ctx context.Context, eventoID string, ch chargeObj) error {
	p, err := s.repo.FindByIntent(ctx, ch.PaymentIntent)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	lista := conciliar(p.Reembolsos, ch)
	estado := EstadoReembolsadoParcial
	if ch.Amount > 0 && ch.AmountRefunded >= ch.Amount {
		estado = EstadoReembolsado
	}
	_, err = s.repo.Aplicar(ctx, p.ID, []string{EstadoAprobado, EstadoReembolsadoParcial}, eventoID,
		bson.D{{Key: "estado", Value: estado}, {Key: "reembolsos", Value: lista}}, nil)
	if errors.Is(err, ErrNoAplica) {
		return nil
	}
	return err
}

// conciliar arma la lista de reembolsos del cargo (sin los fallidos) y conserva el motivo anotado.
func conciliar(previos []Reembolso, ch chargeObj) []Reembolso {
	motivos := make(map[string]string, len(previos))
	for _, r := range previos {
		motivos[r.RefundID] = r.Motivo
	}
	lista := make([]Reembolso, 0, len(ch.Refunds.Data))
	for _, r := range ch.Refunds.Data {
		if r.Status == "failed" || r.Status == "canceled" {
			continue
		}
		motivo := r.Metadata["motivo"]
		if motivo == "" {
			motivo = motivos[r.ID]
		}
		lista = append(lista, Reembolso{RefundID: r.ID, MontoCentavos: r.Amount, Fecha: time.Unix(r.Created, 0).UTC(), Motivo: motivo})
	}
	return lista
}

// estadoCuenta traduce la cuenta de Connect a `cobros.estado`.
func estadoCuenta(a accountObj) string {
	switch {
	case a.Requirements.DisabledReason != "":
		return negocios.CobrosRestringido
	case a.PayoutsEnabled && a.Capabilities.Transfers == "active":
		return negocios.CobrosActivo
	}
	return negocios.CobrosPendiente
}
