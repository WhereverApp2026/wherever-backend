package negocios

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Panel del negocio (nv-56): campos que el dueño edita después del alta y los puntos que
// esperan otras specs (reservas nv-59, cobros nv-60, cupones nv-61, reseñas nv-62). Ningún
// módulo nuevo se importa desde aquí: llegan por las interfaces de este archivo.

// EstadoCambiosPendientes es el filtro de la cola del administrador para nombre, dirección o
// ubicación cambiados por un negocio ya aprobado (no es un `estado` del documento).
const EstadoCambiosPendientes = "cambios_pendientes"

// CobrosActivo es `cobros.estado` cuando nv-60 conectó la cuenta de Stripe.
const CobrosActivo = "activo"

// Anticipo es el cobro por adelantado de una reserva (US-078).
type Anticipo struct {
	Tipo  string `bson:"tipo" json:"tipo"`
	Valor int    `bson:"valor" json:"valor"`
}

// ReservasConfig es «Horas y cupo» de P-52: lo lee nv-59.
type ReservasConfig struct {
	AceptaReservas      bool     `bson:"acepta_reservas" json:"acepta_reservas"`
	CapacidadPorHorario int      `bson:"capacidad_por_horario" json:"capacidad_por_horario"`
	DuracionTurnoMin    int      `bson:"duracion_turno_min" json:"duracion_turno_min"`
	Anticipo            Anticipo `bson:"anticipo" json:"anticipo"`
	PoliticaCancelacion string   `bson:"politica_cancelacion" json:"politica_cancelacion"`
	Confirmacion        string   `bson:"confirmacion" json:"confirmacion"`
	PermiteCambios      bool     `bson:"permite_cambios" json:"permite_cambios"`
}

// CambiosPendientes guarda nombre, dirección o ubicación nuevos hasta que el administrador los
// aplique; el negocio sigue publicado con los de antes.
type CambiosPendientes struct {
	Campos    []string   `bson:"campos"`
	Nombre    string     `bson:"nombre,omitempty"`
	Direccion *Direccion `bson:"direccion,omitempty"`
	Ubicacion *Location  `bson:"ubicacion,omitempty"`
	EnviadoEn time.Time  `bson:"enviado_en"`
}

// Sello es la última decisión del administrador sobre el Sello de Excelencia.
type Sello struct {
	Otorgado    bool          `bson:"otorgado"`
	OtorgadoPor bson.ObjectID `bson:"otorgado_por"`
	OtorgadoEn  time.Time     `bson:"otorgado_en"`
	Motivo      string        `bson:"motivo"`
}

// Cobros es el estado de la cuenta de cobro; lo escribe nv-60.
type Cobros struct {
	Estado string `bson:"estado"`
}

// Avisos que reciben los observadores. Si uno falla, el evento queda en `avisos_pendientes`
// y el trabajo nocturno lo reintenta.
const (
	AvisoPausado    = "pausado"
	AvisoReanudado  = "reanudado"
	AvisoDadoDeBaja = "dado_de_baja"
)

// Observador recibe la pausa, la reanudación y la baja (nv-59 bloquea disponibilidad, nv-61
// pausa cupones). Un error no revierte la acción.
type Observador interface {
	Pausado(ctx context.Context, negocioID bson.ObjectID) error
	Reanudado(ctx context.Context, negocioID bson.ObjectID) error
	DadoDeBaja(ctx context.Context, negocioID bson.ObjectID) error
}

// Observadores reparte cada aviso a todos; junta los errores.
type Observadores []Observador

// Avisar manda el evento a todos los observadores.
func (o Observadores) Avisar(ctx context.Context, evento string, id bson.ObjectID) error {
	var errs []error
	for _, obs := range o {
		var err error
		switch evento {
		case AvisoPausado:
			err = obs.Pausado(ctx, id)
		case AvisoReanudado:
			err = obs.Reanudado(ctx, id)
		case AvisoDadoDeBaja:
			err = obs.DadoDeBaja(ctx, id)
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// Periodo es un intervalo [Desde, Hasta) en UTC.
type Periodo struct {
	Desde time.Time
	Hasta time.Time
}

// ProximaReserva es la siguiente reserva del día.
type ProximaReserva struct {
	Hora     time.Time
	Personas int
}

// ResumenReservas es la tarjeta de reservas del panel (nv-59).
type ResumenReservas struct {
	Hoy     int
	Proxima *ProximaReserva
}

// ResumenCupones es la tarjeta de cupones (nv-61, abre P-49).
type ResumenCupones struct {
	CanjeadosHoy int
	Activos      int
}

// ResumenResenas es la tarjeta de reseñas (nv-62, abre P-50) más los datos de la regla del
// Sello de los últimos 90 días.
type ResumenResenas struct {
	LlegadasHoy            int
	CalificacionSemana     float64
	CambioVsSemanaAnterior float64
	PorResponder           int
	Verificadas90          int
	Promedio90             float64
	RechazadasModeracion90 int
}

// FuenteReservas da el resumen de reservas del día.
type FuenteReservas interface {
	ResumenReservas(ctx context.Context, negocio bson.ObjectID, dia Periodo) (ResumenReservas, error)
}

// FuenteCupones da el resumen de cupones del día.
type FuenteCupones interface {
	ResumenCupones(ctx context.Context, negocio bson.ObjectID, dia Periodo) (ResumenCupones, error)
}

// FuenteResenas da el resumen de reseñas del día y de la semana.
type FuenteResenas interface {
	ResumenResenas(ctx context.Context, negocio bson.ObjectID, dia Periodo) (ResumenResenas, error)
}

// PanelFuente son las fuentes registradas al armar la app; una nil deja su campo en null.
type PanelFuente struct {
	Reservas FuenteReservas
	Cupones  FuenteCupones
	Resenas  FuenteResenas
}

// ReservasFuturas dice si el negocio tiene reservas por venir (nv-59); sin registrar, ninguna.
type ReservasFuturas interface {
	HayReservasFuturas(ctx context.Context, negocio bson.ObjectID, desde time.Time) (bool, error)
}

// ChangeIf aplica `$set` y `$unset` (más `actualizado_en` y `version + 1`) si el negocio cumple
// el filtro. ErrNoAplica si no coincide.
func (r *Repository) ChangeIf(ctx context.Context, filter, set bson.D, unset ...string) (Business, error) {
	set = append(set, bson.E{Key: "actualizado_en", Value: time.Now().UTC()})
	update := bson.D{{Key: "$set", Value: set}, {Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}}
	if len(unset) > 0 {
		u := bson.D{}
		for _, f := range unset {
			u = append(u, bson.E{Key: f, Value: ""})
		}
		update = append(update, bson.E{Key: "$unset", Value: u})
	}
	var b Business
	err := r.col.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&b)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Business{}, ErrNoAplica
	}
	return b, err
}

// MarcarAviso deja un aviso pendiente para el trabajo nocturno; no sube `version`.
func (r *Repository) MarcarAviso(ctx context.Context, id bson.ObjectID, evento string) error {
	_, err := r.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}}, bson.D{{Key: "$addToSet", Value: bson.D{{Key: "avisos_pendientes", Value: evento}}}})
	return err
}

// QuitarAviso borra un aviso ya entregado.
func (r *Repository) QuitarAviso(ctx context.Context, id bson.ObjectID, evento string) error {
	_, err := r.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}}, bson.D{{Key: "$pull", Value: bson.D{{Key: "avisos_pendientes", Value: evento}}}})
	return err
}

// ConAvisos trae negocios con avisos pendientes.
func (r *Repository) ConAvisos(ctx context.Context, limit int64) ([]Business, error) {
	return r.findLimit(ctx, bson.D{{Key: "avisos_pendientes.0", Value: bson.D{{Key: "$exists", Value: true}}}}, limit)
}

// PausasVencidas trae los negocios pausados cuyo `pausado.hasta` ya pasó.
func (r *Repository) PausasVencidas(ctx context.Context, now time.Time, limit int64) ([]Business, error) {
	return r.findLimit(ctx, bson.D{
		{Key: "estado", Value: StatusPaused},
		{Key: "pausado.hasta", Value: bson.D{{Key: "$lte", Value: now}}},
		{Key: "eliminado_en", Value: bson.D{{Key: "$exists", Value: false}}},
	}, limit)
}

func (r *Repository) findLimit(ctx context.Context, filter bson.D, limit int64) ([]Business, error) {
	cur, err := r.col.Find(ctx, filter, options.Find().SetLimit(limit))
	if err != nil {
		return nil, err
	}
	out := []Business{}
	return out, cur.All(ctx, &out)
}
