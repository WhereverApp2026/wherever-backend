package reservas

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Negocios es lo que reservas lee de `negocios`.
type Negocios interface {
	FindVisible(ctx context.Context, id bson.ObjectID) (negocios.Business, bool, error)
	FindByID(ctx context.Context, id bson.ObjectID) (negocios.Business, error)
}

// Users trae la cuenta (correo verificado) y los nombres públicos para la agenda.
type Users interface {
	FindByID(ctx context.Context, id bson.ObjectID) (usuarios.User, error)
	FindPublicByIDs(ctx context.Context, ids []bson.ObjectID) ([]usuarios.User, error)
}

// Anticipos es el puerto que entrega nv-60: dice si el negocio puede cobrar el anticipo con
// Stripe. Sin registrar (hasta nv-60), una reserva con anticipo > 0 responde 422
// `anticipo_no_disponible`.
type Anticipos interface {
	Habilitado(ctx context.Context, negocio negocios.Business) (bool, error)
}

// Refunder es el puerto de reembolso que entrega nv-60. Sin registrar no hay pagos que
// devolver (el reembolso siempre es 0 sin `pago_id`).
type Refunder interface {
	Reembolsar(ctx context.Context, r Reserva, centavos int64) error
}

// Service maneja las reservas (nv-59).
type Service struct {
	repo      *Repository
	negocios  Negocios
	users     Users
	locker    Locker
	idem      Idempotencia
	cursor    *pagination.Signer
	anticipos Anticipos
	refunder  Refunder
	loc       *time.Location
	log       *slog.Logger
	now       func() time.Time
}

// NewService recibe sus dependencias por constructor; el candado y la idempotencia son Redis.
func NewService(repo *Repository, n Negocios, users Users, redis interface {
	Locker
	Idempotencia
}, cursor *pagination.Signer, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, negocios: n, users: users, locker: redis, idem: redis, cursor: cursor,
		loc: negocios.MexicoCity(), log: log, now: time.Now}
}

// WithAnticipos conecta el cobro del anticipo (nv-60).
func (s *Service) WithAnticipos(a Anticipos) *Service {
	s.anticipos = a
	return s
}

// WithRefunder conecta el reembolso real (nv-60).
func (s *Service) WithRefunder(r Refunder) *Service {
	s.refunder = r
	return s
}

// Errores públicos: se crean en cada llamada porque la etapa de errores puede anotarlos.
func errReservaNotFound() error { return apperr.NotFound("reserva no encontrada") }
func errNegocioNotFound() error { return apperr.NotFound("negocio no encontrado") }
func errOcupado() error         { return apperr.Conflict("horario_ocupado") }
func errStale() error {
	return apperr.Conflict("la reserva cambió; vuelve a leerla y usa su versión en If-Match")
}
func errEstado() error { return apperr.Conflict("estado_no_permite") }

// errStore no lleva el texto de MongoDB ni de Redis.
func errStore() error { return apperr.Unavailable("servicio no disponible") }

func invalid(campo, msg string) error {
	return apperr.Validation("datos inválidos", apperr.FieldError{Field: campo, Message: msg})
}

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

func optID(raw string) *bson.ObjectID {
	if id, ok := parseID(raw); ok {
		return &id
	}
	return nil
}

// config devuelve la configuración si el negocio acepta reservas.
func config(b negocios.Business) (negocios.ReservasConfig, bool) {
	if b.ReservasConfig == nil || !b.ReservasConfig.AceptaReservas || b.ReservasConfig.CapacidadPorHorario <= 0 || b.ReservasConfig.DuracionTurnoMin <= 0 {
		return negocios.ReservasConfig{}, false
	}
	return *b.ReservasConfig, true
}

// diaLocal es [00:00, 24:00) del día local de t, en UTC.
func (s *Service) diaLocal(t time.Time) (time.Time, time.Time) {
	y, m, d := t.In(s.loc).Date()
	inicio := time.Date(y, m, d, 0, 0, 0, 0, s.loc)
	return inicio.UTC(), inicio.AddDate(0, 0, 1).UTC()
}

// fechaLocal lee "AAAA-MM-DD" como día de México.
func (s *Service) fechaLocal(raw string) (time.Time, error) {
	return time.ParseInLocation(validation.DateLayout, raw, s.loc)
}

// --- Disponibilidad ---

// Disponibilidad da los horarios del día con cupo (público).
func (s *Service) Disponibilidad(ctx context.Context, rawID string, q DisponibilidadQuery) (DisponibilidadResponse, error) {
	id, _ := parseID(rawID)
	b, err := s.visible(ctx, id)
	if err != nil {
		return DisponibilidadResponse{}, err
	}
	now := s.now()
	fecha, err := s.fechaReservable(q.Fecha, now)
	if err != nil {
		return DisponibilidadResponse{}, err
	}
	out := DisponibilidadResponse{Fecha: q.Fecha, Horarios: []Horario{}}
	cfg, acepta := config(b)
	if !acepta {
		return out, nil
	}
	turnos := Turnos(b.Schedules, cfg.DuracionTurnoMin, fecha, s.loc)
	if len(turnos) == 0 {
		return out, nil
	}
	ultimo := turnos[len(turnos)-1]
	rs, err := s.repo.DelPeriodo(ctx, b.ID, turnos[0], ultimo.Add(time.Minute), activos)
	if err != nil {
		return DisponibilidadResponse{}, errStore()
	}
	bloqueos, err := s.repo.BloqueosEn(ctx, b.ID, turnos[0], ultimo.Add(time.Duration(cfg.DuracionTurnoMin)*time.Minute))
	if err != nil {
		return DisponibilidadResponse{}, errStore()
	}
	out.Horarios = Disponibilidad(turnos, cfg.CapacidadPorHorario, cfg.DuracionTurnoMin, q.Personas, Ocupadas(rs, now), bloqueos, now)
	return out, nil
}

// visible trae el negocio aprobado; si no existe o no es público, 404.
func (s *Service) visible(ctx context.Context, id bson.ObjectID) (negocios.Business, error) {
	if id.IsZero() {
		return negocios.Business{}, errNegocioNotFound()
	}
	b, found, err := s.negocios.FindVisible(ctx, id)
	if err != nil {
		return negocios.Business{}, errStore()
	}
	if !found {
		return negocios.Business{}, errNegocioNotFound()
	}
	return b, nil
}

// fechaReservable lee la fecha y exige que vaya de hoy a 60 días.
func (s *Service) fechaReservable(raw string, now time.Time) (time.Time, error) {
	fecha, err := s.fechaLocal(raw)
	if err != nil {
		return time.Time{}, invalid("fecha", "fecha inválida")
	}
	hoy, _ := s.diaLocal(now)
	if fecha.Before(hoy.In(s.loc)) || fecha.After(hoy.In(s.loc).AddDate(0, 0, MaxDiasAdelante)) {
		return time.Time{}, invalid("fecha", "la fecha va de hoy a 60 días")
	}
	return fecha, nil
}

// --- Crear ---

// Crear aparta el cupo. created=false cuando la misma `Idempotency-Key` ya creó la reserva.
func (s *Service) Crear(ctx context.Context, cl *middleware.Claims, llave string, req CrearRequest) (ReservaResponse, bool, error) {
	me, err := self(cl)
	if err != nil {
		return ReservaResponse{}, false, err
	}
	u, err := s.users.FindByID(ctx, me)
	if err != nil {
		return ReservaResponse{}, false, errStore()
	}
	if !u.EmailVerified {
		return ReservaResponse{}, false, apperr.Forbidden("correo_no_verificado")
	}
	clave := me.Hex() + ":" + llave
	prev, err := s.idem.Tomar(ctx, clave)
	if err != nil {
		return ReservaResponse{}, false, errStore()
	}
	if prev != "" {
		return s.repetida(ctx, me, prev)
	}
	r, err := s.crear(ctx, me, req)
	if err != nil {
		s.idem.Soltar(ctx, clave)
		return ReservaResponse{}, false, err
	}
	if err := s.idem.Guardar(ctx, clave, r.ID.Hex()); err != nil {
		s.log.Warn("reservas: no se guardó la llave de idempotencia", "evento", "reserva_idem_fallo", "error", err.Error())
	}
	return s.response(ctx, r, false), true, nil
}

// repetida responde la reserva que ya creó esa llave; si sigue en curso, 409.
func (s *Service) repetida(ctx context.Context, me bson.ObjectID, prev string) (ReservaResponse, bool, error) {
	id, ok := parseID(prev)
	if !ok {
		return ReservaResponse{}, false, apperr.Conflict("la misma Idempotency-Key está en uso")
	}
	r, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, ErrNotFound) || (err == nil && r.UsuarioID != me) {
		return ReservaResponse{}, false, apperr.Conflict("la misma Idempotency-Key está en uso")
	}
	if err != nil {
		return ReservaResponse{}, false, errStore()
	}
	return s.response(ctx, r, false), false, nil
}

func (s *Service) crear(ctx context.Context, me bson.ObjectID, req CrearRequest) (Reserva, error) {
	negocioID, _ := parseID(req.NegocioID)
	b, err := s.visible(ctx, negocioID)
	if err != nil {
		return Reserva{}, err
	}
	cfg, acepta := config(b)
	if !acepta {
		return Reserva{}, apperr.Validation("no_acepta_reservas")
	}
	hora, err := time.Parse(time.RFC3339, req.FechaHora)
	if err != nil {
		return Reserva{}, invalid("fecha_hora", "fecha y hora inválidas")
	}
	hora = hora.UTC()
	now := s.now().UTC()
	if err := s.validarHora(b, cfg, hora, now); err != nil {
		return Reserva{}, err
	}
	monto := CalcularAnticipo(cfg.Anticipo, b.AveragePrice, req.Personas)
	estado, expira, err := s.estadoInicial(ctx, me, b, cfg, monto, now)
	if err != nil {
		return Reserva{}, err
	}
	r := Reserva{
		ID: bson.NewObjectID(), UsuarioID: me, NegocioID: b.ID, PlanID: optID(req.PlanID), ParadaID: optID(req.ParadaID), GrupoID: optID(req.GrupoID),
		FechaHora: hora, Personas: req.Personas, Nota: strings.TrimSpace(req.Nota), Estado: estado, BloqueoExpiraEn: expira,
		Anticipo: Anticipo{MontoCentavos: monto, Tipo: cfg.Anticipo.Tipo}, Politica: PoliticaDe(cfg.PoliticaCancelacion),
		Historial: []Cambio{{Estado: estado, Fecha: now, Por: PorCliente}}, Version: 1, CreadoEn: now, ActualizadoEn: now,
	}
	if monto == 0 {
		r.Anticipo.Tipo = ""
	}
	err = s.conCupo(ctx, b.ID, hora, cfg.DuracionTurnoMin, nil, func(ocupadas int) error {
		if cfg.CapacidadPorHorario-ocupadas < req.Personas {
			return errOcupado()
		}
		return s.insertar(ctx, &r)
	})
	return r, err
}

// estadoInicial: con anticipo, `bloqueada` 10 minutos (tope de 3 apartados vigentes); sin
// anticipo, `pendiente` si el negocio confirma a mano y `confirmada` si no.
func (s *Service) estadoInicial(ctx context.Context, me bson.ObjectID, b negocios.Business, cfg negocios.ReservasConfig, monto int64, now time.Time) (string, *time.Time, error) {
	if monto == 0 {
		if cfg.Confirmacion == "manual" {
			return EstadoPendiente, nil, nil
		}
		return EstadoConfirmada, nil, nil
	}
	if err := s.anticipoDisponible(ctx, b); err != nil {
		return "", nil, err
	}
	n, err := s.repo.ApartadosVigentes(ctx, me, now)
	if err != nil {
		return "", nil, errStore()
	}
	if n >= MaxApartados {
		return "", nil, apperr.Conflict("demasiados_apartados")
	}
	t := now.Add(ApartadoDuracion)
	return EstadoBloqueada, &t, nil
}

// anticipoDisponible: sin el puerto de nv-60, o sin cobros activos, 422.
func (s *Service) anticipoDisponible(ctx context.Context, b negocios.Business) error {
	if s.anticipos == nil {
		return apperr.Validation("anticipo_no_disponible")
	}
	ok, err := s.anticipos.Habilitado(ctx, b)
	if err != nil {
		return errStore()
	}
	if !ok {
		return apperr.Validation("anticipo_no_disponible")
	}
	return nil
}

// validarHora: futura, a lo más 60 días, en un turno del día y sin bloqueo del negocio.
func (s *Service) validarHora(b negocios.Business, cfg negocios.ReservasConfig, hora, now time.Time) error {
	if !hora.After(now) || hora.After(now.AddDate(0, 0, MaxDiasAdelante+1)) {
		return invalid("fecha_hora", "la hora debe ser futura y dentro de 60 días")
	}
	if !EsTurno(Turnos(b.Schedules, cfg.DuracionTurnoMin, hora.In(s.loc), s.loc), hora) {
		return invalid("fecha_hora", "no es un horario del negocio")
	}
	return nil
}

// conCupo cuenta las personas del horario dentro del candado y corre fn. `excepto` no cuenta
// (la propia reserva al cambiarla). Un bloqueo del negocio deja el horario en 0.
func (s *Service) conCupo(ctx context.Context, negocio bson.ObjectID, hora time.Time, turnoMin int, excepto *bson.ObjectID, fn func(ocupadas int) error) error {
	release, err := s.locker.Lock(ctx, negocio.Hex()+":"+strconv.FormatInt(hora.Unix(), 10))
	if errors.Is(err, ErrLockBusy) {
		return errOcupado()
	}
	if err != nil {
		return errStore()
	}
	defer release()
	bloqueos, err := s.repo.BloqueosEn(ctx, negocio, hora, hora.Add(time.Duration(turnoMin)*time.Minute))
	if err != nil {
		return errStore()
	}
	if len(bloqueos) > 0 {
		return errOcupado()
	}
	rs, err := s.repo.DelHorario(ctx, negocio, hora)
	if err != nil {
		return errStore()
	}
	now := s.now()
	ocupadas := 0
	for _, r := range rs {
		if r.Ocupa(now) && (excepto == nil || r.ID != *excepto) {
			ocupadas += r.Personas
		}
	}
	return fn(ocupadas)
}

// insertar genera el código; si choca con ux_codigo, otro (hasta 5 intentos).
func (s *Service) insertar(ctx context.Context, r *Reserva) error {
	for range 5 {
		codigo, err := NuevoCodigo()
		if err != nil {
			return errStore()
		}
		r.Codigo = codigo
		err = s.repo.Insert(ctx, r)
		if errors.Is(err, ErrDuplicate) {
			continue
		}
		if err != nil {
			return errStore()
		}
		return nil
	}
	return errStore()
}

// --- Lectura ---

// response arma la vista con el reembolso posible y si se puede cambiar o cancelar.
func (s *Service) response(ctx context.Context, r Reserva, porNegocio bool) ReservaResponse {
	now := s.now()
	out := ReservaResponse{
		ID: r.ID.Hex(), Codigo: r.Codigo, NegocioID: r.NegocioID.Hex(), FechaHora: r.FechaHora.UTC(), Personas: r.Personas, Nota: r.Nota,
		Estado: r.Estado, BloqueoExpiraEn: r.BloqueoExpiraEn, Anticipo: r.Anticipo, Politica: r.Politica,
		ReembolsoCentavos: Reembolso(r, now, porNegocio), PuedeCancelar: r.Cancelable(), Version: r.Version, CreadoEn: r.CreadoEn.UTC(),
	}
	if r.PlanID != nil {
		out.PlanID = r.PlanID.Hex()
	}
	if r.ParadaID != nil {
		out.ParadaID = r.ParadaID.Hex()
	}
	if r.GrupoID != nil {
		out.GrupoID = r.GrupoID.Hex()
	}
	if !porNegocio && s.cambiable(r, now) {
		if b, err := s.negocios.FindByID(ctx, r.NegocioID); err == nil {
			cfg, ok := config(b)
			out.PuedeCambiar = ok && cfg.PermiteCambios
		}
	}
	return out
}

func (s *Service) cambiable(r Reserva, now time.Time) bool {
	return (r.Estado == EstadoConfirmada || r.Estado == EstadoPendiente) && r.FechaHora.Sub(now) >= MinHorasCambio*time.Hour
}

// Mias lista las reservas de la persona, próximas o pasadas.
func (s *Service) Mias(ctx context.Context, cl *middleware.Claims, q MiasQuery) (pagination.PageResult[ReservaResponse], error) {
	me, err := self(cl)
	if err != nil {
		return pagination.PageResult[ReservaResponse]{}, err
	}
	if s.cursor == nil {
		return pagination.PageResult[ReservaResponse]{}, errStore()
	}
	proximas := q.Cuando != "pasadas"
	scope := s.cursor.Scope("reservas", me.Hex(), q.Cuando, q.Estado)
	after, err := s.posicion(scope, q.Cursor)
	if err != nil {
		return pagination.PageResult[ReservaResponse]{}, err
	}
	rs, err := s.repo.Mias(ctx, me, proximas, s.now().UTC(), q.Estado, after, q.Limite)
	if err != nil {
		return pagination.PageResult[ReservaResponse]{}, errStore()
	}
	next := ""
	if len(rs) > q.Limite {
		rs = rs[:q.Limite]
		last := rs[len(rs)-1]
		if next, err = s.cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: last.FechaHora.UTC().Format(time.RFC3339Nano), Scope: scope}); err != nil {
			return pagination.PageResult[ReservaResponse]{}, errStore()
		}
	}
	items := make([]ReservaResponse, 0, len(rs))
	for _, r := range rs {
		items = append(items, s.response(ctx, r, false))
	}
	return pagination.NewPageResult(items, next, q.Limite), nil
}

// posicion lee el cursor firmado (hora y id de la última fila); ajeno o alterado = 422.
func (s *Service) posicion(scope, raw string) (*Posicion, error) {
	c, ok, err := validation.Position(s.cursor, scope, raw)
	if err != nil || !ok {
		return nil, err
	}
	id, okID := parseID(c.ID)
	hora, errT := time.Parse(time.RFC3339Nano, c.Sort)
	if !okID || errT != nil {
		return nil, invalid("cursor", "cursor inválido")
	}
	return &Posicion{Hora: hora, ID: id}, nil
}

// acceso dice quién mira la reserva: la persona dueña o el negocio (comerciante dueño). Otro = 404.
func (s *Service) acceso(ctx context.Context, cl *middleware.Claims, raw string) (Reserva, bool, error) {
	me, err := self(cl)
	if err != nil {
		return Reserva{}, false, err
	}
	id, ok := parseID(raw)
	if !ok {
		return Reserva{}, false, errReservaNotFound()
	}
	r, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return Reserva{}, false, errReservaNotFound()
	}
	if err != nil {
		return Reserva{}, false, errStore()
	}
	if r.UsuarioID == me {
		return r, false, nil
	}
	if slices.Contains(cl.Roles, middleware.RoleMerchant) {
		b, err := s.negocios.FindByID(ctx, r.NegocioID)
		if err == nil && b.OwnerID == me {
			return r, true, nil
		}
	}
	return Reserva{}, false, errReservaNotFound()
}

// propia trae la reserva solo si es de la persona.
func (s *Service) propia(ctx context.Context, cl *middleware.Claims, raw string) (Reserva, error) {
	r, porNegocio, err := s.acceso(ctx, cl, raw)
	if err != nil {
		return Reserva{}, err
	}
	if porNegocio {
		return Reserva{}, errReservaNotFound()
	}
	return r, nil
}

// Detalle muestra la reserva a la persona o al negocio.
func (s *Service) Detalle(ctx context.Context, cl *middleware.Claims, raw string) (ReservaResponse, error) {
	r, porNegocio, err := s.acceso(ctx, cl, raw)
	if err != nil {
		return ReservaResponse{}, err
	}
	return s.response(ctx, r, porNegocio), nil
}

// CalcularReembolso dice cuánto se devolvería si la persona cancela ahora.
func (s *Service) CalcularReembolso(ctx context.Context, cl *middleware.Claims, raw string) (ReembolsoResponse, error) {
	r, err := s.propia(ctx, cl, raw)
	if err != nil {
		return ReembolsoResponse{}, err
	}
	now := s.now()
	pct := Porcentaje(r.Politica, r.FechaHora, now, false)
	if r.PagoID == "" || r.Anticipo.MontoCentavos == 0 {
		pct = 0
	}
	return ReembolsoResponse{ReembolsoCentavos: Reembolso(r, now, false), Porcentaje: pct, Politica: r.Politica}, nil
}

// --- Cambios ---

// Cambiar mueve la hora o las personas si el negocio lo permite, con al menos 2 h de margen.
func (s *Service) Cambiar(ctx context.Context, cl *middleware.Claims, raw string, version int, req CambiarRequest) (ReservaResponse, error) {
	r, err := s.propia(ctx, cl, raw)
	if err != nil {
		return ReservaResponse{}, err
	}
	if req.FechaHora == nil && req.Personas == nil {
		return ReservaResponse{}, invalid("fecha_hora", "manda fecha_hora o personas")
	}
	if r.Version != version {
		return ReservaResponse{}, errStale()
	}
	now := s.now().UTC()
	b, cfg, err := s.permiteCambio(ctx, r, now)
	if err != nil {
		return ReservaResponse{}, err
	}
	hora, personas, err := s.nuevosValores(r, b, cfg, req, now)
	if err != nil {
		return ReservaResponse{}, err
	}
	var out Reserva
	err = s.conCupo(ctx, r.NegocioID, hora, cfg.DuracionTurnoMin, &r.ID, func(ocupadas int) error {
		if cfg.CapacidadPorHorario-ocupadas < personas {
			return errOcupado()
		}
		var errC error
		out, errC = s.guardarCambio(ctx, r.ID, version, hora, personas)
		return errC
	})
	if err != nil {
		return ReservaResponse{}, err
	}
	return s.response(ctx, out, false), nil
}

// guardarCambio escribe la hora y las personas si la versión no cambió.
func (s *Service) guardarCambio(ctx context.Context, id bson.ObjectID, version int, hora time.Time, personas int) (Reserva, error) {
	out, err := s.repo.CambiarSi(ctx, id, []string{EstadoConfirmada, EstadoPendiente}, version,
		bson.D{{Key: "fecha_hora", Value: hora}, {Key: "personas", Value: personas}}, nil)
	if errors.Is(err, ErrNoAplica) {
		return Reserva{}, errStale()
	}
	if err != nil {
		return Reserva{}, errStore()
	}
	return out, nil
}

// permiteCambio: `confirmada` o `pendiente`, a 2 h o más, y el negocio con `permite_cambios`.
func (s *Service) permiteCambio(ctx context.Context, r Reserva, now time.Time) (negocios.Business, negocios.ReservasConfig, error) {
	if !s.cambiable(r, now) {
		return negocios.Business{}, negocios.ReservasConfig{}, apperr.Validation("cambio_no_permitido")
	}
	b, err := s.negocios.FindByID(ctx, r.NegocioID)
	if err != nil {
		return negocios.Business{}, negocios.ReservasConfig{}, errStore()
	}
	cfg, ok := config(b)
	if !ok || !cfg.PermiteCambios {
		return negocios.Business{}, negocios.ReservasConfig{}, apperr.Validation("cambio_no_permitido")
	}
	return b, cfg, nil
}

// nuevosValores aplica el cambio pedido: con anticipo no cambian las personas (el cobro ya está
// hecho; lo resuelve nv-60) y la hora nueva debe ser un turno a 2 h o más.
func (s *Service) nuevosValores(r Reserva, b negocios.Business, cfg negocios.ReservasConfig, req CambiarRequest, now time.Time) (time.Time, int, error) {
	hora, personas := r.FechaHora, r.Personas
	if req.Personas != nil {
		if r.Anticipo.MontoCentavos > 0 && *req.Personas != r.Personas {
			return time.Time{}, 0, apperr.Validation("anticipo_cobrado")
		}
		personas = *req.Personas
	}
	if req.FechaHora != nil {
		t, err := time.Parse(time.RFC3339, *req.FechaHora)
		if err != nil {
			return time.Time{}, 0, invalid("fecha_hora", "fecha y hora inválidas")
		}
		hora = t.UTC()
		if err := s.validarHora(b, cfg, hora, now.Add(MinHorasCambio*time.Hour-time.Nanosecond)); err != nil {
			return time.Time{}, 0, err
		}
	}
	return hora, personas, nil
}

// Cancelar cancela la reserva (la persona o el negocio) y libera el cupo.
func (s *Service) Cancelar(ctx context.Context, cl *middleware.Claims, raw string, req CancelarRequest) (ReservaResponse, error) {
	r, porNegocio, err := s.acceso(ctx, cl, raw)
	if err != nil {
		return ReservaResponse{}, err
	}
	if !r.Cancelable() {
		return ReservaResponse{}, errEstado()
	}
	por := PorCliente
	if porNegocio {
		por = PorNegocio
	}
	now := s.now().UTC()
	monto := Reembolso(r, now, porNegocio)
	set := bson.D{{Key: "estado", Value: EstadoCancelada}}
	if m := strings.TrimSpace(req.Motivo); m != "" {
		set = append(set, bson.E{Key: "motivo_cancelacion", Value: m})
	}
	out, err := s.repo.CambiarSi(ctx, r.ID, activos, 0, set, &Cambio{Estado: EstadoCancelada, Fecha: now, Por: por})
	if errors.Is(err, ErrNoAplica) {
		return ReservaResponse{}, errEstado()
	}
	if err != nil {
		return ReservaResponse{}, errStore()
	}
	if monto > 0 && s.refunder != nil {
		if err := s.refunder.Reembolsar(ctx, out, monto); err != nil {
			s.log.Warn("reservas: el reembolso falló; lo reintenta nv-60", "evento", "reserva_reembolso_fallo", "error", err.Error())
		}
	}
	return s.response(ctx, out, porNegocio), nil
}

// ErrCupoPerdido: el pago llegó después de vencer el apartado y el cupo ya no está; la reserva
// queda `cancelada` con motivo `cupo_perdido` y nv-60 reembolsa completo.
var ErrCupoPerdido = errors.New("cupo perdido")

// MotivoCupoPerdido es el `motivo_cancelacion` de un pago tardío sin cupo.
const MotivoCupoPerdido = "cupo_perdido"

// ConfirmarPago es lo que llama nv-60 cuando Stripe confirma el anticipo: `bloqueada` →
// `confirmada` (o `pendiente` si el negocio confirma a mano). Si el apartado ya venció (pago
// tardío), confirma solo si el cupo sigue libre; si no, ErrCupoPerdido. Otro estado = ErrNoAplica.
func (s *Service) ConfirmarPago(ctx context.Context, id bson.ObjectID, pagoID string) (Reserva, error) {
	r, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return Reserva{}, err
	}
	b, errNegocio := s.negocios.FindByID(ctx, r.NegocioID)
	cfg, acepta := config(b)
	estado := estadoPagado(errNegocio == nil && acepta, cfg)
	now := s.now().UTC()
	set := bson.D{{Key: "estado", Value: estado}, {Key: "pago_id", Value: pagoID}}
	cambio := &Cambio{Estado: estado, Fecha: now, Por: PorSistema}
	if r.Estado == EstadoBloqueada && r.Ocupa(now) {
		return s.repo.CambiarSi(ctx, id, []string{EstadoBloqueada}, 0, set, cambio, "bloqueo_expira_en")
	}
	if r.Estado != EstadoBloqueada && r.Estado != EstadoExpirada {
		return Reserva{}, ErrNoAplica
	}
	out, err := s.confirmarTardio(ctx, r, cfg, errCupo(errNegocio, acepta), set, cambio, now)
	if err == nil || !cupoPerdido(err) {
		return out, err
	}
	return s.perderCupo(ctx, id, pagoID, now)
}

// estadoPagado: `pendiente` si el negocio confirma a mano; si no, `confirmada`.
func estadoPagado(conConfig bool, cfg negocios.ReservasConfig) string {
	if conConfig && cfg.Confirmacion == "manual" {
		return EstadoPendiente
	}
	return EstadoConfirmada
}

// perderCupo cancela la reserva pagada tarde sin cupo y devuelve ErrCupoPerdido.
func (s *Service) perderCupo(ctx context.Context, id bson.ObjectID, pagoID string, now time.Time) (Reserva, error) {
	perdida := bson.D{{Key: "estado", Value: EstadoCancelada}, {Key: "pago_id", Value: pagoID}, {Key: "motivo_cancelacion", Value: MotivoCupoPerdido}}
	out, err := s.repo.CambiarSi(ctx, id, []string{EstadoBloqueada, EstadoExpirada}, 0, perdida,
		&Cambio{Estado: EstadoCancelada, Fecha: now, Por: PorSistema}, "bloqueo_expira_en")
	if err != nil {
		return Reserva{}, err
	}
	return out, ErrCupoPerdido
}

// confirmarTardio confirma un pago que llegó con el apartado vencido si el cupo sigue libre.
func (s *Service) confirmarTardio(ctx context.Context, r Reserva, cfg negocios.ReservasConfig, previo error, set bson.D, cambio *Cambio, now time.Time) (Reserva, error) {
	if previo != nil {
		return Reserva{}, previo
	}
	var out Reserva
	err := s.conCupo(ctx, r.NegocioID, r.FechaHora, cfg.DuracionTurnoMin, &r.ID, func(ocupadas int) error {
		if !r.FechaHora.After(now) || cfg.CapacidadPorHorario-ocupadas < r.Personas {
			return ErrCupoPerdido
		}
		var errCambio error
		out, errCambio = s.repo.CambiarSi(ctx, r.ID, []string{EstadoBloqueada, EstadoExpirada}, 0, set, cambio, "bloqueo_expira_en")
		return errCambio
	})
	return out, err
}

// cupoPerdido: sin cupo, horario bloqueado u ocupado (409 del candado).
func cupoPerdido(err error) bool {
	var ae *apperr.Error
	return errors.Is(err, ErrCupoPerdido) || (errors.As(err, &ae) && ae.Kind == apperr.KindConflict)
}

// errCupo: sin negocio o sin reservas ya no hay cupo que recuperar.
func errCupo(errNegocio error, acepta bool) error {
	if errNegocio != nil || !acepta {
		return ErrCupoPerdido
	}
	return nil
}

// --- Negocio ---

// negocioPropio trae el negocio si es de la persona; ajeno o inexistente = 404.
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

// Agenda es el día del negocio, por hora; `q` busca por nombre sin acentos o inicio del código.
func (s *Service) Agenda(ctx context.Context, cl *middleware.Claims, raw string, q AgendaQuery) (AgendaResponse, error) {
	b, err := s.negocioPropio(ctx, cl, raw)
	if err != nil {
		return AgendaResponse{}, err
	}
	fecha := s.now().In(s.loc)
	if q.Fecha != "" {
		if fecha, err = s.fechaLocal(q.Fecha); err != nil {
			return AgendaResponse{}, invalid("fecha", "fecha inválida")
		}
	}
	desde, hasta := s.diaLocal(fecha)
	var estados []string
	if q.Estado != "" {
		estados = []string{q.Estado}
	}
	rs, err := s.repo.DelPeriodo(ctx, b.ID, desde, hasta, estados)
	if err != nil {
		return AgendaResponse{}, errStore()
	}
	nombres, err := s.nombres(ctx, rs)
	if err != nil {
		return AgendaResponse{}, errStore()
	}
	return AgendaResponse{Fecha: fecha.Format(validation.DateLayout), Items: filtrarAgenda(rs, nombres, q.Q)}, nil
}

// filtrarAgenda deja las filas cuyo nombre (sin acentos) contiene `q` o cuyo código empieza
// con `q`; sin `q`, todas.
func filtrarAgenda(rs []Reserva, nombres map[bson.ObjectID]string, q string) []AgendaItem {
	buscar := text.Normalize(q)
	codigo := strings.ToUpper(strings.TrimSpace(q))
	out := []AgendaItem{}
	for _, r := range rs {
		nombre := nombres[r.UsuarioID]
		if buscar != "" && !strings.Contains(text.Normalize(nombre), buscar) && !strings.HasPrefix(r.Codigo, codigo) {
			continue
		}
		out = append(out, AgendaItem{ID: r.ID.Hex(), Codigo: r.Codigo, FechaHora: r.FechaHora.UTC(), Cliente: nombre,
			Personas: r.Personas, Nota: r.Nota, Anticipo: r.Anticipo, Estado: r.Estado, Version: r.Version})
	}
	return out
}

// nombres resuelve el nombre público de cada cliente; la cuenta borrada sale «Cuenta eliminada».
func (s *Service) nombres(ctx context.Context, rs []Reserva) (map[bson.ObjectID]string, error) {
	ids := []bson.ObjectID{}
	seen := map[bson.ObjectID]bool{}
	for _, r := range rs {
		if !seen[r.UsuarioID] {
			seen[r.UsuarioID] = true
			ids = append(ids, r.UsuarioID)
		}
	}
	out := map[bson.ObjectID]string{}
	for _, id := range ids {
		out[id] = CuentaEliminada
	}
	if len(ids) == 0 {
		return out, nil
	}
	us, err := s.users.FindPublicByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, u := range us {
		out[u.ID] = u.Name
	}
	return out, nil
}

// reservaDelNegocio trae la reserva si es de ese negocio; si no, 404.
func (s *Service) reservaDelNegocio(ctx context.Context, b negocios.Business, raw string) (Reserva, error) {
	id, ok := parseID(raw)
	if !ok {
		return Reserva{}, errReservaNotFound()
	}
	r, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, ErrNotFound) || (err == nil && r.NegocioID != b.ID) {
		return Reserva{}, errReservaNotFound()
	}
	if err != nil {
		return Reserva{}, errStore()
	}
	return r, nil
}

// CambiarEstado: el negocio confirma una `pendiente` o marca asistencia de una `confirmada`.
func (s *Service) CambiarEstado(ctx context.Context, cl *middleware.Claims, rawNegocio, rawID string, req EstadoRequest) (ReservaResponse, error) {
	b, err := s.negocioPropio(ctx, cl, rawNegocio)
	if err != nil {
		return ReservaResponse{}, err
	}
	r, err := s.reservaDelNegocio(ctx, b, rawID)
	if err != nil {
		return ReservaResponse{}, err
	}
	now := s.now().UTC()
	desde := EstadoConfirmada
	switch req.Estado {
	case EstadoConfirmada:
		desde = EstadoPendiente
	case EstadoAsistio:
		if inicio, _ := s.diaLocal(r.FechaHora); now.Before(inicio) {
			return ReservaResponse{}, apperr.Validation("aun_no_es_el_dia")
		}
	case EstadoNoAsistio:
		if now.Before(r.FechaHora) {
			return ReservaResponse{}, apperr.Validation("aun_no_es_la_hora")
		}
	}
	if r.Estado != desde {
		return ReservaResponse{}, errEstado()
	}
	return s.mover(ctx, r, desde, req.Estado, now)
}

func (s *Service) mover(ctx context.Context, r Reserva, desde, hacia string, now time.Time) (ReservaResponse, error) {
	out, err := s.repo.CambiarSi(ctx, r.ID, []string{desde}, 0, bson.D{{Key: "estado", Value: hacia}}, &Cambio{Estado: hacia, Fecha: now, Por: PorNegocio})
	if errors.Is(err, ErrNoAplica) {
		return ReservaResponse{}, errEstado()
	}
	if err != nil {
		return ReservaResponse{}, errStore()
	}
	return s.response(ctx, out, true), nil
}

// Escanear marca la llegada con el código: debe ser de ese negocio y de hoy; si no, 404 igual.
func (s *Service) Escanear(ctx context.Context, cl *middleware.Claims, rawNegocio string, req EscanearRequest) (ReservaResponse, error) {
	b, err := s.negocioPropio(ctx, cl, rawNegocio)
	if err != nil {
		return ReservaResponse{}, err
	}
	r, err := s.repo.FindByCodigo(ctx, b.ID, strings.ToUpper(req.Codigo))
	if errors.Is(err, ErrNotFound) {
		return ReservaResponse{}, errReservaNotFound()
	}
	if err != nil {
		return ReservaResponse{}, errStore()
	}
	now := s.now().UTC()
	desde, hasta := s.diaLocal(now)
	if r.FechaHora.Before(desde) || !r.FechaHora.Before(hasta) {
		return ReservaResponse{}, errReservaNotFound()
	}
	if r.Estado != EstadoConfirmada {
		return ReservaResponse{}, apperr.Conflict("codigo_ya_usado")
	}
	return s.mover(ctx, r, EstadoConfirmada, EstadoAsistio, now)
}

// CrearBloqueo cierra horas del negocio (máximo 31 días).
func (s *Service) CrearBloqueo(ctx context.Context, cl *middleware.Claims, raw string, req BloqueoRequest) (BloqueoResponse, error) {
	b, err := s.negocioPropio(ctx, cl, raw)
	if err != nil {
		return BloqueoResponse{}, err
	}
	me, _ := self(cl)
	desde, err1 := time.Parse(time.RFC3339, req.Desde)
	hasta, err2 := time.Parse(time.RFC3339, req.Hasta)
	if err1 != nil || err2 != nil {
		return BloqueoResponse{}, invalid("desde", "fechas inválidas")
	}
	if !desde.Before(hasta) || hasta.Sub(desde) > MaxDiasBloqueo*24*time.Hour {
		return BloqueoResponse{}, invalid("hasta", "hasta debe ser posterior a desde y a lo más 31 días")
	}
	motivo := req.Motivo
	if motivo == "" {
		motivo = MotivoOtro
	}
	bl := Bloqueo{ID: bson.NewObjectID(), NegocioID: b.ID, Desde: desde.UTC(), Hasta: hasta.UTC(), Motivo: motivo, CreadoPor: me, CreadoEn: s.now().UTC()}
	if err := s.repo.InsertBloqueo(ctx, &bl); err != nil {
		return BloqueoResponse{}, errStore()
	}
	return BloqueoResponse{ID: bl.ID.Hex(), Desde: bl.Desde, Hasta: bl.Hasta, Motivo: bl.Motivo}, nil
}

// BorrarBloqueo abre de nuevo las horas.
func (s *Service) BorrarBloqueo(ctx context.Context, cl *middleware.Claims, raw, rawBloqueo string) error {
	b, err := s.negocioPropio(ctx, cl, raw)
	if err != nil {
		return err
	}
	id, ok := parseID(rawBloqueo)
	if !ok {
		return apperr.NotFound("bloqueo no encontrado")
	}
	err = s.repo.BorrarBloqueo(ctx, b.ID, id)
	if errors.Is(err, ErrNotFound) {
		return apperr.NotFound("bloqueo no encontrado")
	}
	if err != nil {
		return errStore()
	}
	return nil
}
