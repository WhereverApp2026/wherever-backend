package reservas

import (
	"context"
	"math"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// MensajesCollection guarda los mensajes del negocio al cliente de una reserva (nv-73).
const MensajesCollection = "mensajes_reserva"

// Topes de los mensajes y del recordatorio (nv-73).
const (
	MaxMensajeTexto    = 500
	MaxMensajesVentana = 5
	VentanaMensajes    = 24 * time.Hour
	VidaMensaje        = 90 * 24 * time.Hour
	MaxMensajesDetalle = 20
	// VentanaContacto: tras `asistio` o `no_asistio` el negocio aún puede escribir 24 h.
	VentanaContacto = 24 * time.Hour
	// AntesRecordatorio es cuánto antes de `fecha_hora` sale el aviso.
	AntesRecordatorio = 2 * time.Hour
	// MaxRecordatorios por corrida del barrido; el resto sale en la siguiente.
	MaxRecordatorios = 500
)

// Tipos de aviso de nv-73 (canal `reservas`, siempre activo). `datos` solo lleva ids.
const (
	KindMensaje      = notificaciones.KindReservaMensaje
	KindRecordatorio = notificaciones.KindReservaRecordatorio
)

// Mensaje es un documento de `mensajes_reserva`. `usuario_id` es el cliente (destinatario).
type Mensaje struct {
	ID        bson.ObjectID `bson:"_id"`
	ReservaID bson.ObjectID `bson:"reserva_id"`
	NegocioID bson.ObjectID `bson:"negocio_id"`
	UsuarioID bson.ObjectID `bson:"usuario_id"`
	Texto     string        `bson:"texto"`
	AutorID   bson.ObjectID `bson:"autor_id"`
	CreadoEn  time.Time     `bson:"creado_en"`
	ExpiraEn  time.Time     `bson:"expira_en"`
}

// Contactable dice si el negocio puede escribir al cliente: `confirmada`, o `asistio` /
// `no_asistio` con la hora de hace menos de 24 h.
func (r Reserva) Contactable(now time.Time) bool {
	switch r.Estado {
	case EstadoConfirmada:
		return true
	case EstadoAsistio, EstadoNoAsistio:
		return now.Sub(r.FechaHora) < VentanaContacto
	}
	return false
}

// EnVentanaRecordatorio dice si la reserva toca recordatorio en `now`: `confirmada`, sin aviso
// previo y con la hora en (now, now + 2 h].
func (r Reserva) EnVentanaRecordatorio(now time.Time) bool {
	return r.Estado == EstadoConfirmada && r.RecordatorioEn == nil &&
		r.FechaHora.After(now) && !r.FechaHora.After(now.Add(AntesRecordatorio))
}

// MensajeRequest es el cuerpo de POST /negocios/{id}/reservas/{rid}/mensajes. `excludesall`
// rechaza `<` y `>`: el texto va sin HTML.
type MensajeRequest struct {
	Texto string `json:"texto" validate:"required,max=500,excludesall=<>"`
}

// Normalizar recorta los espacios antes de validar (solo espacios = vacío = 422).
func (m *MensajeRequest) Normalizar() { m.Texto = strings.TrimSpace(m.Texto) }

// MensajeResponse es el 201 del mensaje.
type MensajeResponse struct {
	ID        string    `json:"id"`
	ReservaID string    `json:"reserva_id"`
	Texto     string    `json:"texto"`
	CreadoEn  time.Time `json:"creado_en"`
}

// MensajeItem es un mensaje en el detalle de la reserva que ve el cliente.
type MensajeItem struct {
	ID       string    `json:"id"`
	Texto    string    `json:"texto"`
	CreadoEn time.Time `json:"creado_en"`
}

func errNoContactable() error { return apperr.Conflict("reserva_no_contactable") }

// EnviarMensaje deja un mensaje del negocio al cliente de la reserva y le avisa. Reserva de
// otro negocio o inexistente = 404; estado sin contacto = 409; el 6.º en 24 h = 429.
func (s *Service) EnviarMensaje(ctx context.Context, cl *middleware.Claims, rawNegocio, rawID string, req MensajeRequest) (MensajeResponse, error) {
	b, err := s.negocioPropio(ctx, cl, rawNegocio)
	if err != nil {
		return MensajeResponse{}, err
	}
	r, err := s.reservaDelNegocio(ctx, b, rawID)
	if err != nil {
		return MensajeResponse{}, err
	}
	now := s.now().UTC()
	if !r.Contactable(now) {
		return MensajeResponse{}, errNoContactable()
	}
	recientes, err := s.repo.MensajesDesde(ctx, r.ID, now.Add(-VentanaMensajes), MaxMensajesVentana)
	if err != nil {
		return MensajeResponse{}, errStore()
	}
	if len(recientes) >= MaxMensajesVentana {
		return MensajeResponse{}, apperr.RateLimited(esperaMensaje(recientes[0], now))
	}
	m := Mensaje{ID: bson.NewObjectID(), ReservaID: r.ID, NegocioID: b.ID, UsuarioID: r.UsuarioID, Texto: req.Texto,
		AutorID: b.OwnerID, CreadoEn: now, ExpiraEn: now.Add(VidaMensaje)}
	if err := s.repo.InsertMensaje(ctx, &m); err != nil {
		return MensajeResponse{}, errStore()
	}
	s.notifier.Emit(ctx, r.UsuarioID, KindMensaje, map[string]string{
		"reserva_id": r.ID.Hex(), "negocio_id": b.ID.Hex(), "mensaje_id": m.ID.Hex(),
	})
	s.log.Info("reservas: mensaje al cliente", "evento", "reserva_mensaje", "reserva_id", r.ID.Hex())
	return MensajeResponse{ID: m.ID.Hex(), ReservaID: r.ID.Hex(), Texto: m.Texto, CreadoEn: m.CreadoEn}, nil
}

// esperaMensaje son los segundos hasta que el más viejo de la ventana sale de ella (≥ 1).
func esperaMensaje(masViejo, now time.Time) int {
	falta := masViejo.Add(VentanaMensajes).Sub(now)
	return max(1, int(math.Ceil(falta.Seconds())))
}

// mensajesDe arma la lista del detalle: los 20 más nuevos, de más nuevo a más viejo.
func (s *Service) mensajesDe(ctx context.Context, reserva bson.ObjectID) ([]MensajeItem, error) {
	ms, err := s.repo.MensajesDe(ctx, reserva, MaxMensajesDetalle)
	if err != nil {
		return nil, err
	}
	out := make([]MensajeItem, 0, len(ms))
	for _, m := range ms {
		out = append(out, MensajeItem{ID: m.ID.Hex(), Texto: m.Texto, CreadoEn: m.CreadoEn.UTC()})
	}
	return out, nil
}

// Recordar avisa una sola vez a cada reserva `confirmada` que empieza en las próximas 2 h. El
// `UpdateOne` condicional sobre `recordatorio_en` hace que dos instancias no dupliquen el aviso.
func (s *Service) Recordar(ctx context.Context) (int64, error) {
	now := s.now().UTC()
	rs, err := s.repo.PorRecordar(ctx, now, now.Add(AntesRecordatorio), MaxRecordatorios)
	if err != nil {
		return 0, err
	}
	var n int64
	for _, r := range rs {
		if !r.EnVentanaRecordatorio(now) {
			continue
		}
		marcada, err := s.repo.MarcarRecordatorio(ctx, r, now)
		if err != nil {
			return n, err
		}
		if !marcada {
			continue
		}
		s.notifier.Emit(ctx, r.UsuarioID, KindRecordatorio, map[string]string{"reserva_id": r.ID.Hex(), "negocio_id": r.NegocioID.Hex()})
		n++
	}
	return n, nil
}

// --- Repositorio ---

// InsertMensaje guarda un mensaje.
func (r *Repository) InsertMensaje(ctx context.Context, m *Mensaje) error {
	_, err := r.mensajes.InsertOne(ctx, m)
	return err
}

// MensajesDesde trae las fechas de hasta `limit` mensajes de la reserva desde `desde`, del más
// viejo al más nuevo (ix_mensajes_reserva).
func (r *Repository) MensajesDesde(ctx context.Context, reserva bson.ObjectID, desde time.Time, limit int) ([]time.Time, error) {
	ms, err := findAll[Mensaje](ctx, r.mensajes, bson.D{
		{Key: "reserva_id", Value: reserva}, {Key: "creado_en", Value: bson.D{{Key: "$gte", Value: desde}}},
	}, options.Find().SetSort(bson.D{{Key: "creado_en", Value: 1}}).SetLimit(int64(limit)).SetProjection(bson.D{{Key: "creado_en", Value: 1}}))
	if err != nil {
		return nil, err
	}
	out := make([]time.Time, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.CreadoEn)
	}
	return out, nil
}

// MensajesDe trae los `limit` mensajes más nuevos de la reserva.
func (r *Repository) MensajesDe(ctx context.Context, reserva bson.ObjectID, limit int) ([]Mensaje, error) {
	return findAll[Mensaje](ctx, r.mensajes, bson.D{{Key: "reserva_id", Value: reserva}},
		options.Find().SetSort(bson.D{{Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(int64(limit)))
}

// MensajesDeUsuario trae todos los mensajes que recibió la persona (exportación ARCO).
func (r *Repository) MensajesDeUsuario(ctx context.Context, usuario bson.ObjectID) ([]Mensaje, error) {
	return findAll[Mensaje](ctx, r.mensajes, bson.D{{Key: "usuario_id", Value: usuario}},
		options.Find().SetSort(bson.D{{Key: "creado_en", Value: -1}}))
}

// BorrarMensajesDeUsuario borra los mensajes que recibió la persona (ARCO).
func (r *Repository) BorrarMensajesDeUsuario(ctx context.Context, usuario bson.ObjectID) error {
	_, err := r.mensajes.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: usuario}})
	return err
}

// PorRecordar trae las `confirmada` sin recordatorio con la hora en (desde, hasta] (ix_recordatorios).
func (r *Repository) PorRecordar(ctx context.Context, desde, hasta time.Time, limit int) ([]Reserva, error) {
	return findAll[Reserva](ctx, r.col, bson.D{
		{Key: "estado", Value: EstadoConfirmada},
		{Key: "fecha_hora", Value: bson.D{{Key: "$gt", Value: desde}, {Key: "$lte", Value: hasta}}},
		{Key: "recordatorio_en", Value: bson.D{{Key: "$exists", Value: false}}},
	}, options.Find().SetSort(bson.D{{Key: "fecha_hora", Value: 1}}).SetLimit(int64(limit)).
		SetProjection(bson.D{{Key: "usuario_id", Value: 1}, {Key: "negocio_id", Value: 1}, {Key: "fecha_hora", Value: 1}, {Key: "estado", Value: 1}}))
}

// MarcarRecordatorio pone `recordatorio_en` solo si la reserva sigue `confirmada`, a la misma
// hora y sin aviso; true si esta llamada la marcó. No sube la versión: no es un cambio del
// cliente ni del negocio.
func (r *Repository) MarcarRecordatorio(ctx context.Context, res Reserva, now time.Time) (bool, error) {
	out, err := r.col.UpdateOne(ctx, bson.D{
		{Key: "_id", Value: res.ID}, {Key: "estado", Value: EstadoConfirmada}, {Key: "fecha_hora", Value: res.FechaHora},
		{Key: "recordatorio_en", Value: bson.D{{Key: "$exists", Value: false}}},
	}, bson.D{{Key: "$set", Value: bson.D{{Key: "recordatorio_en", Value: now}}}})
	if err != nil {
		return false, err
	}
	return out.ModifiedCount == 1, nil
}
