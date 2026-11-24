package reservas

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Resultado de cada parada al reservar todo el plan (nv-89).
const (
	ParadaReservada    = "reservada"
	ParadaPendiente    = "pendiente"
	ParadaNoAcepta     = "no_acepta_reservas"
	ParadaSinCupo      = "sin_cupo"
	ParadaYaReservada  = "ya_reservada"
	ParadaFueraHorario = "fuera_de_horario"
	// ParadaNoReservada cubre lo demás que rechaza POST /reservas (p. ej. el tope de 3
	// apartados con anticipo); `motivo` dice cuál.
	ParadaNoReservada = "no_reservada"
)

// errYaReservada: dentro del candado, la parada ya tenía una reserva activa del plan.
var errYaReservada = errors.New("parada ya reservada")

// Planes da el plan que quien llama puede cambiar: dueño o integrante con `editar`
// (planes.Service.Editable).
type Planes interface {
	Editable(ctx context.Context, subject, rawID string) (planes.Plan, error)
}

// WithPlanes conecta reservar todo el plan (nv-89).
func (s *Service) WithPlanes(p Planes) *Service {
	s.planes = p
	return s
}

// MismaParada dice si alguna reserva que ocupa cupo en `now` es de la misma parada del mismo
// plan que r.
func MismaParada(rs []Reserva, r Reserva, now time.Time) bool {
	if r.PlanID == nil || r.ParadaID == nil {
		return false
	}
	for _, o := range rs {
		if o.Ocupa(now) && o.PlanID != nil && o.ParadaID != nil && *o.PlanID == *r.PlanID && *o.ParadaID == *r.ParadaID {
			return true
		}
	}
	return false
}

// TurnoDe es el turno que contiene la llegada: el que empieza a esa hora o el último que empezó
// antes y no ha terminado. false si la llegada cae fuera de los turnos del negocio.
func TurnoDe(turnos []time.Time, turnoMin int, llega time.Time) (time.Time, bool) {
	dur := time.Duration(turnoMin) * time.Minute
	for i := len(turnos) - 1; i >= 0; i-- {
		t := turnos[i]
		if !t.After(llega) && llega.Before(t.Add(dur)) {
			return t, true
		}
	}
	return time.Time{}, false
}

// paradaInfo es lo que se lee de una parada antes de reservar.
type paradaInfo struct {
	parada  planes.Parada
	negocio negocios.Business
	cfg     negocios.ReservasConfig
	visible bool
	acepta  bool
	vigente bool
}

// ReservarPlan reserva todas las paradas del plan en una petición (POST /planes/{id}/reservas).
// Cada parada usa el camino de POST /reservas (candado por negocio y turno, cupo, apartado y
// anticipo) con la llave `{llave}:{parada_id}`, así un reintento con la misma llave devuelve
// las reservas ya hechas. Éxito parcial: una parada sin cupo no deshace las demás. cobrar
// cuenta en la política `reservas` una reserva por parada que se va a intentar; Redis caído en
// la idempotencia o el candado = 503 y lo ya hecho queda (el reintento lo recupera).
func (s *Service) ReservarPlan(ctx context.Context, cl *middleware.Claims, rawPlan, llave string, req PlanRequest, cobrar func(n int) error) (PlanResponse, error) {
	me, p, personas, err := s.preparar(ctx, cl, rawPlan, req)
	if err != nil {
		return PlanResponse{}, err
	}
	info, err := s.paradas(ctx, p, s.now().UTC())
	if err != nil {
		return PlanResponse{}, err
	}
	if cobrar != nil {
		if err := cobrar(intentos(info)); err != nil {
			return PlanResponse{}, err
		}
	}
	out := PlanResponse{Resultados: make([]PlanResultado, 0, len(info))}
	for _, in := range info {
		res, err := s.reservarParada(ctx, me, p, in, llave, personas, req.Nota)
		if err != nil {
			return PlanResponse{}, err
		}
		out.sumar(res)
	}
	return out, nil
}

// preparar valida a quien llama, el plan que puede cambiar, las personas y que el plan no haya
// pasado.
func (s *Service) preparar(ctx context.Context, cl *middleware.Claims, rawPlan string, req PlanRequest) (bson.ObjectID, planes.Plan, int, error) {
	me, err := s.verificada(ctx, cl)
	if err != nil {
		return me, planes.Plan{}, 0, err
	}
	if s.planes == nil {
		return me, planes.Plan{}, 0, errStore()
	}
	p, err := s.planes.Editable(ctx, cl.Subject, rawPlan)
	if err != nil {
		return me, planes.Plan{}, 0, err
	}
	personas := req.Personas
	if personas == 0 {
		personas = p.Personas
	}
	if personas < 1 || personas > MaxPersonas {
		return me, planes.Plan{}, 0, invalid("personas", "las personas van de 1 a 20")
	}
	if !pendiente(p, s.now().UTC()) {
		return me, planes.Plan{}, 0, apperr.Validation("el plan ya pasó", apperr.FieldError{Field: "plan", Message: "plan_pasado"})
	}
	return me, p, personas, nil
}

// intentos cuenta las paradas que se van a intentar reservar: aceptan reservas y no tienen
// una reserva activa del plan (un reintento ya no las cuenta).
func intentos(info []paradaInfo) int {
	n := 0
	for _, in := range info {
		if in.acepta && !in.vigente {
			n++
		}
	}
	return n
}

// sumar agrega el resultado y, si espera el pago del anticipo, lo suma al total.
func (out *PlanResponse) sumar(res PlanResultado) {
	if res.Estado == ParadaPendiente && res.Reserva != nil {
		out.AnticipoTotal += res.Reserva.Anticipo.MontoCentavos
		out.RequierenPago++
	}
	out.Resultados = append(out.Resultados, res)
}

// verificada es quien llama, con el correo verificado (igual que POST /reservas).
func (s *Service) verificada(ctx context.Context, cl *middleware.Claims) (bson.ObjectID, error) {
	me, err := self(cl)
	if err != nil {
		return me, err
	}
	u, err := s.users.FindByID(ctx, me)
	if err != nil {
		return me, errStore()
	}
	if !u.EmailVerified {
		return me, apperr.Forbidden("correo_no_verificado")
	}
	return me, nil
}

// pendiente dice si alguna parada todavía no llega.
func pendiente(p planes.Plan, now time.Time) bool {
	for _, st := range p.Paradas {
		if st.Llega.After(now) {
			return true
		}
	}
	return false
}

// paradas lee el negocio de cada parada y si ya tiene una reserva activa del plan.
func (s *Service) paradas(ctx context.Context, p planes.Plan, now time.Time) ([]paradaInfo, error) {
	info := make([]paradaInfo, 0, len(p.Paradas))
	ids := make([]bson.ObjectID, 0, len(p.Paradas))
	for _, st := range p.Paradas {
		b, found, err := s.negocios.FindVisible(ctx, st.NegocioID)
		if err != nil {
			return nil, errStore()
		}
		cfg, acepta := config(b)
		info = append(info, paradaInfo{parada: st, negocio: b, cfg: cfg, visible: found, acepta: found && acepta})
		ids = append(ids, st.NegocioID)
	}
	desde, hasta := rango(p)
	rs, err := s.repo.DelPlanEn(ctx, p.ID, ids, desde, hasta)
	if err != nil {
		return nil, errStore()
	}
	for i := range info {
		info[i].vigente = conReservaActiva(rs, info[i].parada.ID, now)
	}
	return info, nil
}

// rango va de la primera a la última llegada con un día a cada lado: la reserva de la parada
// es la del turno que contiene la llegada.
func rango(p planes.Plan) (time.Time, time.Time) {
	var desde, hasta time.Time
	for i, st := range p.Paradas {
		if i == 0 || st.Llega.Before(desde) {
			desde = st.Llega
		}
		if i == 0 || st.Llega.After(hasta) {
			hasta = st.Llega
		}
	}
	return desde.Add(-24 * time.Hour), hasta.Add(24 * time.Hour)
}

// conReservaActiva dice si la parada tiene una reserva que ocupa cupo en `now`.
func conReservaActiva(rs []Reserva, parada bson.ObjectID, now time.Time) bool {
	for _, r := range rs {
		if r.ParadaID != nil && *r.ParadaID == parada && r.Ocupa(now) {
			return true
		}
	}
	return false
}

// reservarParada intenta una parada. Solo devuelve error para cortar toda la petición (Redis o
// MongoDB caídos, la misma llave en curso); lo demás queda en el resultado de la parada.
func (s *Service) reservarParada(ctx context.Context, me bson.ObjectID, p planes.Plan, in paradaInfo, llave string, personas int, nota string) (PlanResultado, error) {
	res := PlanResultado{ParadaID: in.parada.ID.Hex(), NegocioID: in.parada.NegocioID.Hex()}
	switch {
	case !in.visible:
		res.Estado, res.Motivo = ParadaNoAcepta, "negocio_no_disponible"
		return res, nil
	case !in.acepta:
		res.Estado = ParadaNoAcepta
		return res, nil
	}
	clave := me.Hex() + ":" + llave + ":" + in.parada.ID.Hex()
	prev, err := s.idem.Tomar(ctx, clave)
	if err != nil {
		return res, errStore()
	}
	if prev != "" {
		r, _, err := s.repetida(ctx, me, prev)
		if err != nil {
			return res, err
		}
		return conReserva(res, r), nil
	}
	if in.vigente {
		s.idem.Soltar(ctx, clave)
		res.Estado = ParadaYaReservada
		return res, nil
	}
	hora, ok := TurnoDe(Turnos(in.negocio.Schedules, in.cfg.DuracionTurnoMin, in.parada.Llega.In(s.loc), s.loc), in.cfg.DuracionTurnoMin, in.parada.Llega)
	if !ok {
		s.idem.Soltar(ctx, clave)
		res.Estado = ParadaFueraHorario
		return res, nil
	}
	r, err := s.crearReserva(ctx, me, CrearRequest{
		NegocioID: in.parada.NegocioID.Hex(), FechaHora: hora.UTC().Format(time.RFC3339), Personas: personas, Nota: nota,
		PlanID: p.ID.Hex(), ParadaID: in.parada.ID.Hex(),
	}, true)
	if err != nil {
		s.idem.Soltar(ctx, clave)
		return motivo(res, err)
	}
	if err := s.idem.Guardar(ctx, clave, r.ID.Hex()); err != nil {
		s.log.Warn("reservas: no se guardó la llave de idempotencia", "evento", "reserva_idem_fallo", "error", err.Error())
	}
	return conReserva(res, s.response(ctx, r, false)), nil
}

// conReserva pone la reserva de la parada: `pendiente` si espera el pago del anticipo,
// `reservada` si ya aparta el lugar. Si la llave repetida trae una reserva que ya no aparta
// (cancelada o vencida), `no_reservada` con su estado de motivo.
func conReserva(res PlanResultado, r ReservaResponse) PlanResultado {
	switch r.Estado {
	case EstadoBloqueada:
		res.Estado = ParadaPendiente
	case EstadoPendiente, EstadoConfirmada:
		res.Estado = ParadaReservada
	default:
		res.Estado, res.Motivo = ParadaNoReservada, r.Estado
	}
	res.Reserva = &r
	return res
}

// motivo traduce el rechazo de una parada; un error del servicio corta la petición.
func motivo(res PlanResultado, err error) (PlanResultado, error) {
	if errors.Is(err, errYaReservada) {
		res.Estado = ParadaYaReservada
		return res, nil
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		return res, errStore()
	}
	estado, why, ok := rechazo(appErr)
	if !ok {
		return res, err
	}
	res.Estado, res.Motivo = estado, why
	return res, nil
}

// rechazo da el estado y el motivo de la parada para lo que rechaza POST /reservas; false si
// es un error del servicio.
func rechazo(e *apperr.Error) (string, string, bool) {
	switch e.Kind {
	case apperr.KindNotFound:
		return ParadaNoAcepta, "negocio_no_disponible", true
	case apperr.KindConflict:
		if e.Message == "horario_ocupado" {
			return ParadaSinCupo, "", true
		}
		return ParadaNoReservada, e.Message, true
	case apperr.KindValidation:
		switch {
		case len(e.Fields) > 0 && e.Fields[0].Field == "fecha_hora":
			return ParadaFueraHorario, "", true
		case e.Message == "no_acepta_reservas":
			return ParadaNoAcepta, "", true
		}
		return ParadaNoReservada, e.Message, true
	}
	return "", "", false
}
