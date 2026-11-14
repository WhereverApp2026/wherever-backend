package pagos

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Corte semanal (A5): cada lunes, dentro del candado del trabajo nocturno (una sola instancia),
// junta los pagos cobrados de las reservas de la semana anterior, resta comisión y reembolsos y
// transfiere el neto a la cuenta de Connect del negocio. Sin cuenta `activo`, la liquidación
// queda `pendiente` y se reintenta el lunes siguiente. La llave de idempotencia de Stripe es
// `liquidacion:{negocio}:{semana}`: repetir el corte no transfiere dos veces.

// Tarea es el corte semanal como privacidad.Tarea.
type Tarea struct{ svc *Service }

// NewTarea recibe el service por constructor.
func NewTarea(svc *Service) *Tarea { return &Tarea{svc: svc} }

// Name cumple privacidad.Tarea.
func (t *Tarea) Name() string { return LiquidacionesCollection }

// Run solo corta los lunes (hora de México); los demás días no hace nada.
func (t *Tarea) Run(ctx context.Context) (int, error) {
	now := t.svc.now()
	if now.In(t.svc.loc).Weekday() != time.Monday {
		return 0, nil
	}
	return t.svc.Cortar(ctx, SemanaInicio(now, t.svc.loc).In(t.svc.loc).AddDate(0, 0, -7).UTC())
}

// Cortar liquida la semana que empieza en `semana` y reintenta las pendientes. Devuelve cuántas
// liquidaciones transfirió.
func (s *Service) Cortar(ctx context.Context, semana time.Time) (int, error) {
	if s.stripe == nil {
		return 0, nil
	}
	fin := semana.In(s.loc).AddDate(0, 0, 7).UTC()
	ps, err := s.repo.PorLiquidar(ctx, semana, fin, nil)
	if err != nil {
		return 0, err
	}
	porNegocio := map[bson.ObjectID][]Pago{}
	for _, p := range ps {
		porNegocio[p.NegocioID] = append(porNegocio[p.NegocioID], p)
	}
	now := s.now().UTC()
	var errs []error
	for negocio, pagos := range porNegocio {
		if err := s.calcular(ctx, negocio, semana, pagos, now); err != nil {
			errs = append(errs, err)
		}
	}
	pendientes, err := s.repo.LiquidacionesPendientes(ctx, nil)
	if err != nil {
		return 0, errors.Join(append(errs, err)...)
	}
	n := 0
	for _, l := range pendientes {
		ok, err := s.transferir(ctx, l)
		if err != nil {
			errs = append(errs, err)
		}
		if ok {
			n++
		}
	}
	return n, errors.Join(errs...)
}

// calcular guarda la liquidación del negocio y marca sus pagos. Si la semana ya existía (otro
// corte la empezó), solo marca los pagos que falten.
func (s *Service) calcular(ctx context.Context, negocio bson.ObjectID, semana time.Time, pagos []Pago, now time.Time) error {
	var total Desglose
	ids := make([]bson.ObjectID, 0, len(pagos))
	for _, p := range pagos {
		total = total.Sumar(DesgloseDe(p))
		ids = append(ids, p.ID)
	}
	l := Liquidacion{
		ID: bson.NewObjectID(), NegocioID: negocio, SemanaInicio: semana, Pagos: len(pagos), TotalAnticipos: total.Anticipo,
		Comision: total.Comision, Reembolsos: total.Reembolsos, Neto: total.Neto, Estado: LiquidacionPendiente,
		ConservarHasta: ConservarHasta(now), CreadoEn: now, ActualizadoEn: now,
	}
	if err := s.repo.InsertLiquidacion(ctx, &l); err != nil && !errors.Is(err, ErrDuplicate) {
		return err
	}
	return s.repo.MarcarLiquidados(ctx, ids, semana)
}

// transferir manda el neto si la cuenta del negocio está activa; si no, sigue pendiente.
func (s *Service) transferir(ctx context.Context, l Liquidacion) (bool, error) {
	b, err := s.negocios.FindByID(ctx, l.NegocioID)
	if err != nil {
		return false, err
	}
	if ok, _ := s.Habilitado(ctx, b); !ok {
		return false, nil
	}
	semana := fecha(l.SemanaInicio, s.loc)
	transfer := ""
	if l.Neto > 0 {
		transfer, err = s.stripe.CreateTransfer(ctx, TransferParams{
			Destino: b.Cobros.StripeAccountID, MontoCentavos: l.Neto, Grupo: "liquidacion_" + semana,
			Idem: "liquidacion:" + l.NegocioID.Hex() + ":" + semana,
		})
		if err != nil {
			return false, errors.New("stripe: " + mensajeStripe(err))
		}
	}
	if err := s.repo.Transferida(ctx, l.ID, transfer); err != nil {
		return false, err
	}
	return true, nil
}
