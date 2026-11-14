package pagos

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// Reservas es lo que pagos usa de nv-59.
type Reservas interface {
	FindByID(ctx context.Context, id bson.ObjectID) (reservas.Reserva, error)
	ConfirmarPago(ctx context.Context, id bson.ObjectID, pagoID string) (reservas.Reserva, error)
}

// Negocios lee el negocio (dueño, cobros y cuenta de depósito).
type Negocios interface {
	FindByID(ctx context.Context, id bson.ObjectID) (negocios.Business, error)
}

// CLABEs lee la CLABE cifrada del expediente (nv-55) para registrarla al conectar la cuenta.
type CLABEs interface {
	CuentaDeposito(ctx context.Context, negocio bson.ObjectID) (clabe, titular string, ok bool, err error)
}

// Config son los datos fijos del módulo.
type Config struct {
	// WebhookSecret firma los eventos (STRIPE_WEBHOOK_SECRET); vacío = webhook 503.
	WebhookSecret string
	// LinkBase es la base de los enlaces de regreso del registro en Stripe (APP_LINK_BASE).
	LinkBase string
}

// Service cobra, confirma, reembolsa y liquida (nv-60).
type Service struct {
	repo     *Repository
	stripe   StripeAPI
	idem     Idempotencia
	reservas Reservas
	negocios Negocios
	clabes   CLABEs
	cursor   *pagination.Signer
	cfg      Config
	loc      *time.Location
	log      *slog.Logger
	now      func() time.Time
}

// NewService recibe sus dependencias por constructor. `stripe` nil = sin llave: las rutas de
// pago responden 503 `pagos_no_disponibles` y la API arranca.
func NewService(repo *Repository, api StripeAPI, idem Idempotencia, res Reservas, neg Negocios, cursor *pagination.Signer, cfg Config, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, stripe: api, idem: idem, reservas: res, negocios: neg, cursor: cursor, cfg: cfg,
		loc: negocios.MexicoCity(), log: log, now: time.Now}
}

// WithCLABEs conecta el expediente para registrar la CLABE al conectar la cuenta.
func (s *Service) WithCLABEs(c CLABEs) *Service {
	s.clabes = c
	return s
}

// WithClock fija el reloj (pruebas del corte semanal).
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

func errStore() error           { return apperr.Unavailable("servicio no disponible") }
func errNoDisponible() error    { return apperr.Unavailable("pagos_no_disponibles") }
func errPagoNotFound() error    { return apperr.NotFound("pago no encontrado") }
func errReservaNotFound() error { return apperr.NotFound("reserva no encontrada") }
func errNegocioNotFound() error { return apperr.NotFound("negocio no encontrado") }

func self(cl *middleware.Claims) (bson.ObjectID, error) {
	if cl == nil {
		return bson.ObjectID{}, apperr.Unauthorized("autenticación requerida")
	}
	id, err := bson.ObjectIDFromHex(cl.Subject)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("autenticación requerida")
	}
	return id, nil
}

func parseID(raw string) (bson.ObjectID, bool) {
	id, err := bson.ObjectIDFromHex(raw)
	return id, err == nil
}

// Habilitado cumple reservas.Anticipos: hay llave de Stripe y la cuenta del negocio está activa.
func (s *Service) Habilitado(_ context.Context, b negocios.Business) (bool, error) {
	return s.stripe != nil && b.Cobros != nil && b.Cobros.Estado == negocios.CobrosActivo && b.Cobros.StripeAccountID != "", nil
}

// --- Cobro del anticipo ---

// Cobrar crea o reutiliza el PaymentIntent del anticipo de una reserva `bloqueada` vigente.
func (s *Service) Cobrar(ctx context.Context, cl *middleware.Claims, rawReserva, llave string) (PagoResponse, error) {
	me, err := self(cl)
	if err != nil {
		return PagoResponse{}, err
	}
	if s.stripe == nil {
		return PagoResponse{}, errNoDisponible()
	}
	r, err := s.reservaPropia(ctx, me, rawReserva)
	if err != nil {
		return PagoResponse{}, err
	}
	clave := me.Hex() + ":" + llave
	prev, err := s.idem.TomarPago(ctx, clave)
	if err != nil {
		return PagoResponse{}, errStore()
	}
	if prev != "" {
		return s.repetido(ctx, me, prev)
	}
	out, err := s.cobrar(ctx, r)
	if err != nil {
		s.idem.SoltarPago(ctx, clave)
		return PagoResponse{}, err
	}
	if err := s.idem.GuardarPago(ctx, clave, out.PagoID); err != nil {
		s.log.Warn("pagos: no se guardó la llave de idempotencia", "evento", "pago_idem_fallo", "error", err.Error())
	}
	return out, nil
}

func (s *Service) reservaPropia(ctx context.Context, me bson.ObjectID, raw string) (reservas.Reserva, error) {
	id, ok := parseID(raw)
	if !ok {
		return reservas.Reserva{}, errReservaNotFound()
	}
	r, err := s.reservas.FindByID(ctx, id)
	if errors.Is(err, reservas.ErrNotFound) || (err == nil && r.UsuarioID != me) {
		return reservas.Reserva{}, errReservaNotFound()
	}
	if err != nil {
		return reservas.Reserva{}, errStore()
	}
	return r, nil
}

// repetido responde el pago que creó esa llave; si sigue en curso, 409.
func (s *Service) repetido(ctx context.Context, me bson.ObjectID, prev string) (PagoResponse, error) {
	id, ok := parseID(prev)
	if !ok {
		return PagoResponse{}, apperr.Conflict("la misma Idempotency-Key está en uso")
	}
	p, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, ErrNotFound) || (err == nil && p.UsuarioID != me) {
		return PagoResponse{}, apperr.Conflict("la misma Idempotency-Key está en uso")
	}
	if err != nil {
		return PagoResponse{}, errStore()
	}
	return s.secreto(ctx, p)
}

// cobrar valida el estado de la reserva, reutiliza un pago abierto o crea otro intento.
func (s *Service) cobrar(ctx context.Context, r reservas.Reserva) (PagoResponse, error) {
	now := s.now().UTC()
	if err := cobrable(r, now); err != nil {
		return PagoResponse{}, err
	}
	ultimo, err := s.repo.UltimoDeReserva(ctx, r.ID)
	switch {
	case err == nil && (ultimo.Abierto() || ultimo.Estado == EstadoRechazado):
		// Un pago abierto o rechazado se reintenta con el mismo PaymentIntent (no se crea otro).
		return s.secreto(ctx, ultimo)
	case err == nil:
		return PagoResponse{}, apperr.Conflict("ya_pagada")
	case !errors.Is(err, ErrNotFound):
		return PagoResponse{}, errStore()
	}
	b, err := s.negocios.FindByID(ctx, r.NegocioID)
	if err != nil {
		return PagoResponse{}, errStore()
	}
	if ok, _ := s.Habilitado(ctx, b); !ok {
		return PagoResponse{}, errNoDisponible()
	}
	n, err := s.repo.Intentos(ctx, r.ID)
	if err != nil {
		return PagoResponse{}, errStore()
	}
	return s.nuevoIntento(ctx, r, b, n+1, now)
}

// cobrable: solo una reserva `bloqueada` vigente con anticipo; vencida = 410.
func cobrable(r reservas.Reserva, now time.Time) error {
	switch {
	case r.Estado == reservas.EstadoExpirada, r.Estado == reservas.EstadoBloqueada && !r.Ocupa(now):
		return apperr.Gone("apartado_vencido")
	case r.Estado != reservas.EstadoBloqueada, r.Anticipo.MontoCentavos <= 0:
		return apperr.Conflict("estado_no_permite")
	}
	return nil
}

// nuevoIntento crea el PaymentIntent con `reserva:{id}:intento:{n}` y guarda el pago.
func (s *Service) nuevoIntento(ctx context.Context, r reservas.Reserva, b negocios.Business, n int64, now time.Time) (PagoResponse, error) {
	p := Pago{
		ID: bson.NewObjectID(), ReservaID: r.ID, UsuarioID: r.UsuarioID, NegocioID: r.NegocioID, Concepto: ConceptoAnticipo,
		MontoCentavos: r.Anticipo.MontoCentavos, ComisionCentavos: Comision(r.Anticipo.MontoCentavos, comisionPct(b)), Moneda: Moneda,
		Estado: EstadoRequierePago, Reembolsos: []Reembolso{}, EventosProcesados: []string{}, FechaReserva: r.FechaHora,
		IdempotencyKey: "reserva:" + r.ID.Hex() + ":intento:" + strconv.FormatInt(n, 10),
		ConservarHasta: ConservarHasta(now), CreadoEn: now, ActualizadoEn: now,
	}
	intent, err := s.stripe.CreatePaymentIntent(ctx, IntentParams{
		MontoCentavos: p.MontoCentavos, Idem: p.IdempotencyKey,
		Metadata: map[string]string{"reserva_id": r.ID.Hex(), "pago_id": p.ID.Hex()},
	})
	if err != nil {
		s.log.Warn("pagos: Stripe no creó el cobro", "evento", "pago_stripe_fallo", "dependencia", "stripe", "error", mensajeStripe(err))
		return PagoResponse{}, errNoDisponible()
	}
	p.PaymentIntentID = intent.ID
	err = s.repo.Insert(ctx, &p)
	if errors.Is(err, ErrDuplicate) {
		// Otra petición creó el mismo intento: se responde ese.
		otro, errOtro := s.repo.FindByIntent(ctx, intent.ID)
		if errOtro != nil {
			return PagoResponse{}, apperr.Conflict("la misma Idempotency-Key está en uso")
		}
		p = otro
	} else if err != nil {
		return PagoResponse{}, errStore()
	}
	return PagoResponse{PagoID: p.ID.Hex(), ClientSecret: intent.ClientSecret, MontoCentavos: p.MontoCentavos, Moneda: Moneda}, nil
}

// secreto lee el `client_secret` de Stripe: no se guarda en la base.
func (s *Service) secreto(ctx context.Context, p Pago) (PagoResponse, error) {
	intent, err := s.stripe.GetPaymentIntent(ctx, p.PaymentIntentID)
	if err != nil {
		return PagoResponse{}, errNoDisponible()
	}
	return PagoResponse{PagoID: p.ID.Hex(), ClientSecret: intent.ClientSecret, MontoCentavos: p.MontoCentavos, Moneda: p.Moneda}, nil
}

func comisionPct(b negocios.Business) int {
	if b.Cobros != nil && b.Cobros.ComisionPct > 0 {
		return b.Cobros.ComisionPct
	}
	return ComisionPctInicial
}

// --- Estado al volver de 3D Secure ---

// Estado da el estado del pago al dueño; si sigue abierto más de 30 s consulta a Stripe una vez.
func (s *Service) Estado(ctx context.Context, cl *middleware.Claims, raw string) (EstadoResponse, error) {
	p, err := s.pagoPropio(ctx, cl, raw)
	if err != nil {
		return EstadoResponse{}, err
	}
	if p.Abierto() && s.stripe != nil && s.now().Sub(p.ActualizadoEn) > ConsultaProcesando {
		p = s.consultar(ctx, p)
	}
	out := EstadoResponse{Estado: p.Estado}
	if r, err := s.reservas.FindByID(ctx, p.ReservaID); err == nil {
		out.ReservaEstado = r.Estado
	}
	return out, nil
}

// pagoPropio trae el pago si es de la persona; ajeno o inexistente = 404.
func (s *Service) pagoPropio(ctx context.Context, cl *middleware.Claims, raw string) (Pago, error) {
	me, err := self(cl)
	if err != nil {
		return Pago{}, err
	}
	id, ok := parseID(raw)
	if !ok {
		return Pago{}, errPagoNotFound()
	}
	p, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, ErrNotFound) || (err == nil && p.UsuarioID != me) {
		return Pago{}, errPagoNotFound()
	}
	if err != nil {
		return Pago{}, errStore()
	}
	return p, nil
}

// consultar aplica lo que diga Stripe sin esperar al webhook; un error deja el pago como está.
func (s *Service) consultar(ctx context.Context, p Pago) Pago {
	intent, err := s.stripe.GetPaymentIntent(ctx, p.PaymentIntentID)
	if err != nil {
		return p
	}
	var aplicado Pago
	switch intent.Status {
	case "succeeded":
		aplicado, err = s.aprobar(ctx, p, intent, "")
	case "processing":
		aplicado, err = s.repo.Aplicar(ctx, p.ID, []string{EstadoRequierePago, EstadoProcesando}, "", bson.D{{Key: "estado", Value: EstadoProcesando}}, nil)
	default:
		// Sigue esperando a la persona: solo se marca la consulta para no repetirla antes de 30 s.
		aplicado, err = s.repo.Aplicar(ctx, p.ID, []string{p.Estado}, "", bson.D{}, nil)
	}
	if err != nil {
		return p
	}
	return aplicado
}

// --- Reembolso (puerto reservas.Refunder) ---

// Reembolsar pide a Stripe el reembolso de la reserva cancelada; el estado final llega con
// `charge.refunded`. La idempotencia es por reserva: un reintento no devuelve dos veces.
func (s *Service) Reembolsar(ctx context.Context, r reservas.Reserva, centavos int64) error {
	id, ok := parseID(r.PagoID)
	if !ok {
		return ErrNotFound
	}
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	return s.reembolsar(ctx, p, centavos, "cancelacion")
}

func (s *Service) reembolsar(ctx context.Context, p Pago, centavos int64, motivo string) error {
	if s.stripe == nil {
		return errNoDisponible()
	}
	centavos = min(centavos, p.MontoCentavos-p.Reembolsado())
	if centavos <= 0 {
		return nil
	}
	refund, err := s.stripe.CreateRefund(ctx, RefundParams{
		PaymentIntent: p.PaymentIntentID, MontoCentavos: centavos, Idem: "reembolso:" + p.ReservaID.Hex(),
		Metadata: map[string]string{"reserva_id": p.ReservaID.Hex(), "pago_id": p.ID.Hex(), "motivo": motivo},
	})
	if err != nil {
		return errors.New("stripe: " + mensajeStripe(err))
	}
	// Se anota al pedirlo para que el corte semanal no lo pague; `charge.refunded` lo concilia.
	nuevo := Reembolso{RefundID: refund, MontoCentavos: centavos, Fecha: s.now().UTC(), Motivo: motivo}
	_, err = s.repo.Aplicar(ctx, p.ID, nil, "", bson.D{}, bson.D{{Key: "reembolsos", Value: nuevo}})
	if errors.Is(err, ErrNoAplica) {
		return nil
	}
	return err
}

// --- Negocio: Connect, cuenta de depósito y consultas ---

// negocioPropio trae el negocio si es del comerciante; ajeno o inexistente = 404.
func (s *Service) negocioPropio(ctx context.Context, cl *middleware.Claims, raw string) (negocios.Business, error) {
	me, err := self(cl)
	if err != nil {
		return negocios.Business{}, err
	}
	id, ok := parseID(raw)
	if !ok {
		return negocios.Business{}, errNegocioNotFound()
	}
	b, err := s.negocios.FindByID(ctx, id)
	if errors.Is(err, negocios.ErrNotFound) {
		return negocios.Business{}, errNegocioNotFound()
	}
	if err != nil {
		return negocios.Business{}, errStore()
	}
	if b.OwnerID != me || b.DeletedAt != nil {
		return negocios.Business{}, errNegocioNotFound()
	}
	return b, nil
}

// Conectar crea (o continúa) la cuenta Express y devuelve el enlace de registro de Stripe.
func (s *Service) Conectar(ctx context.Context, cl *middleware.Claims, raw string) (ConectarResponse, error) {
	b, err := s.negocioPropio(ctx, cl, raw)
	if err != nil {
		return ConectarResponse{}, err
	}
	if s.stripe == nil {
		return ConectarResponse{}, errNoDisponible()
	}
	cobros := negocios.Cobros{Estado: negocios.CobrosPendiente, ComisionPct: ComisionPctInicial}
	if b.Cobros != nil {
		cobros = *b.Cobros
		if cobros.ComisionPct == 0 {
			cobros.ComisionPct = ComisionPctInicial
		}
	}
	if cobros.StripeAccountID == "" {
		account, err := s.stripe.CreateAccount(ctx, b.ID.Hex(), "cuenta:"+b.ID.Hex())
		if err != nil {
			s.log.Warn("pagos: Stripe no creó la cuenta", "evento", "cuenta_stripe_fallo", "dependencia", "stripe", "error", mensajeStripe(err))
			return ConectarResponse{}, errNoDisponible()
		}
		cobros.StripeAccountID, cobros.Estado = account, negocios.CobrosPendiente
		if err := s.repo.SetCobros(ctx, b.ID, cobros); err != nil {
			return ConectarResponse{}, errStore()
		}
		b.Cobros = &cobros
		s.registrarGuardada(ctx, b)
	}
	base := s.cfg.LinkBase + "/negocios/" + b.ID.Hex() + "/cobros"
	url, err := s.stripe.CreateAccountLink(ctx, cobros.StripeAccountID, base+"?resultado=reintentar", base+"?resultado=listo")
	if err != nil {
		return ConectarResponse{}, errNoDisponible()
	}
	return ConectarResponse{URL: url}, nil
}

// registrarGuardada manda a Stripe la CLABE que el dueño guardó en el alta (nv-55).
func (s *Service) registrarGuardada(ctx context.Context, b negocios.Business) {
	if s.clabes == nil || b.CuentaDeposito == nil || b.CuentaDeposito.StripeExternalAccountID != "" {
		return
	}
	clabe, titular, ok, err := s.clabes.CuentaDeposito(ctx, b.ID)
	if err != nil || !ok {
		return
	}
	if _, err := s.RegistrarCuenta(ctx, b, clabe, titular); err != nil {
		s.log.Warn("pagos: la CLABE no se registró en Stripe", "evento", "cuenta_stripe_fallo", "dependencia", "stripe", "error", err.Error())
	}
}

// RegistrarCuenta cumple alta.Deposito: registra la CLABE como cuenta externa. Sin cuenta de
// Connect devuelve "" (se registra al conectar). La CLABE completa no se guarda aquí.
func (s *Service) RegistrarCuenta(ctx context.Context, b negocios.Business, clabe, titular string) (string, error) {
	if s.stripe == nil || b.Cobros == nil || b.Cobros.StripeAccountID == "" {
		return "", nil
	}
	externa, err := s.stripe.CreateExternalAccount(ctx, b.Cobros.StripeAccountID, clabe, titular, "cuenta_externa:"+b.ID.Hex()+":"+clabe[len(clabe)-4:])
	if err != nil {
		return "", errors.New("stripe: " + mensajeStripe(err))
	}
	if err := s.repo.CuentaRegistrada(ctx, b.ID, externa); err != nil {
		return "", err
	}
	return externa, nil
}

// Cobro es lo que recibe el negocio de una reserva pagada.
func (s *Service) Cobro(ctx context.Context, cl *middleware.Claims, rawNegocio, rawReserva string) (CobroResponse, error) {
	b, err := s.negocioPropio(ctx, cl, rawNegocio)
	if err != nil {
		return CobroResponse{}, err
	}
	id, ok := parseID(rawReserva)
	if !ok {
		return CobroResponse{}, errReservaNotFound()
	}
	r, err := s.reservas.FindByID(ctx, id)
	if err != nil || r.NegocioID != b.ID {
		return CobroResponse{}, errReservaNotFound()
	}
	pid, ok := parseID(r.PagoID)
	if !ok {
		return CobroResponse{}, errPagoNotFound()
	}
	p, err := s.repo.FindByID(ctx, pid)
	if errors.Is(err, ErrNotFound) {
		return CobroResponse{}, errPagoNotFound()
	}
	if err != nil {
		return CobroResponse{}, errStore()
	}
	d := DesgloseDe(p)
	out := CobroResponse{Anticipo: d.Anticipo, Reembolsos: d.Reembolsos, Comision: d.Comision, Neto: d.Neto, Estado: p.Estado}
	if p.LiquidacionSemana != nil {
		out.LiquidacionSemana = fecha(*p.LiquidacionSemana, s.loc)
	}
	return out, nil
}

// Saldo: `en_espera` = la semana en curso, `disponible` = liquidaciones calculadas sin
// transferir (cuenta no activa) y `proximo_deposito` = el siguiente lunes.
func (s *Service) Saldo(ctx context.Context, cl *middleware.Claims, raw string) (SaldoResponse, error) {
	b, err := s.negocioPropio(ctx, cl, raw)
	if err != nil {
		return SaldoResponse{}, err
	}
	now := s.now()
	ps, err := s.repo.PorLiquidar(ctx, time.Time{}, ProximoLunes(now, s.loc), &b.ID)
	if err != nil {
		return SaldoResponse{}, errStore()
	}
	var espera Desglose
	for _, p := range ps {
		espera = espera.Sumar(DesgloseDe(p))
	}
	pendientes, err := s.repo.LiquidacionesPendientes(ctx, &b.ID)
	if err != nil {
		return SaldoResponse{}, errStore()
	}
	var disponible int64
	for _, l := range pendientes {
		disponible += l.Neto
	}
	return SaldoResponse{Disponible: disponible, EnEspera: espera.Neto, ProximoDeposito: fecha(ProximoLunes(now, s.loc), s.loc)}, nil
}

// Liquidaciones lista las del negocio por cursor (semana, de la más nueva a la más vieja).
func (s *Service) Liquidaciones(ctx context.Context, cl *middleware.Claims, raw string, q LiquidacionesQuery) (pagination.PageResult[LiquidacionResponse], error) {
	b, err := s.negocioPropio(ctx, cl, raw)
	if err != nil {
		return pagination.PageResult[LiquidacionResponse]{}, err
	}
	if s.cursor == nil {
		return pagination.PageResult[LiquidacionResponse]{}, errStore()
	}
	scope := s.cursor.Scope("liquidaciones", b.ID.Hex())
	antes, err := s.antesDe(scope, q.Cursor)
	if err != nil {
		return pagination.PageResult[LiquidacionResponse]{}, err
	}
	rows, err := s.repo.Liquidaciones(ctx, b.ID, antes, q.Limite)
	if err != nil {
		return pagination.PageResult[LiquidacionResponse]{}, errStore()
	}
	next := ""
	if len(rows) > q.Limite {
		rows = rows[:q.Limite]
		last := rows[len(rows)-1]
		if next, err = s.cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: last.SemanaInicio.UTC().Format(time.RFC3339), Scope: scope}); err != nil {
			return pagination.PageResult[LiquidacionResponse]{}, errStore()
		}
	}
	items := make([]LiquidacionResponse, 0, len(rows))
	for _, l := range rows {
		items = append(items, liquidacionResponse(l, s.loc))
	}
	return pagination.NewPageResult(items, next, q.Limite), nil
}

// antesDe lee la semana del cursor firmado; vacío = desde la más nueva; ajeno o alterado = 422.
func (s *Service) antesDe(scope, raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	invalido := apperr.Validation("datos inválidos", apperr.FieldError{Field: "cursor", Message: "cursor inválido"})
	c, ok, err := s.cursor.Decode(scope, raw)
	if err != nil || !ok {
		return nil, invalido
	}
	t, err := time.Parse(time.RFC3339, c.Sort)
	if err != nil {
		return nil, invalido
	}
	return &t, nil
}

// fecha es AAAA-MM-DD en la zona del negocio.
func fecha(t time.Time, loc *time.Location) string { return t.In(loc).Format("2006-01-02") }

// reservasPuerto junta el repositorio y el service de reservas en el puerto Reservas.
type reservasPuerto struct {
	repo *reservas.Repository
	svc  *reservas.Service
}

func (r reservasPuerto) FindByID(ctx context.Context, id bson.ObjectID) (reservas.Reserva, error) {
	return r.repo.FindByID(ctx, id)
}

func (r reservasPuerto) ConfirmarPago(ctx context.Context, id bson.ObjectID, pagoID string) (reservas.Reserva, error) {
	return r.svc.ConfirmarPago(ctx, id, pagoID)
}

// ReservasDe arma el puerto Reservas con nv-59.
func ReservasDe(repo *reservas.Repository, svc *reservas.Service) Reservas {
	return reservasPuerto{repo: repo, svc: svc}
}
