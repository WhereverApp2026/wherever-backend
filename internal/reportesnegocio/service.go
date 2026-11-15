package reportesnegocio

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Negocios es lo que los reportes leen de negocios (nv-20/nv-21).
type Negocios interface {
	FindVisible(ctx context.Context, id bson.ObjectID) (negocios.Business, bool, error)
	FindCards(ctx context.Context, ids []bson.ObjectID, now time.Time) ([]negocios.PlaceItem, error)
}

// Moderador evalúa el detalle (nv-42).
type Moderador interface {
	Evaluar(ctx context.Context, c moderacion.Contenido) moderacion.Resultado
}

// Service maneja los reportes (nv-67).
type Service struct {
	repo      *Repository
	negocios  Negocios
	cursor    *pagination.Signer
	moderador Moderador
	log       *slog.Logger
	now       func() time.Time
}

// NewService recibe sus dependencias por constructor.
func NewService(repo *Repository, n Negocios, cursor *pagination.Signer, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, negocios: n, cursor: cursor, log: log, now: time.Now}
}

// WithModeracion conecta el moderador de texto del detalle.
func (s *Service) WithModeracion(m Moderador) *Service {
	s.moderador = m
	return s
}

// CrearRequest es el cuerpo de POST /lugares/{id}/reportes.
type CrearRequest struct {
	Motivo  string `json:"motivo" validate:"required,oneof=horario precio cerrado_permanente ubicacion otro"`
	Detalle string `json:"detalle" validate:"omitempty,max=300"`
}

// ResolverRequest es el cuerpo de PATCH /admin/reportes-negocio/{id}.
type ResolverRequest struct {
	Estado string `json:"estado" validate:"required,oneof=corregido descartado"`
}

// ColaQuery es GET /admin/reportes-negocio.
type ColaQuery struct {
	Estado    string `query:"estado" validate:"omitempty,oneof=abierto corregido descartado"`
	NegocioID string `query:"negocio_id" validate:"omitempty,len=24,hexadecimal"`
	Cursor    string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limit     int    `query:"limite" validate:"min=1,max=100"`
}

// CreadoResponse es la respuesta de crear.
type CreadoResponse struct {
	ID     string `json:"id"`
	Estado string `json:"estado"`
}

// ReporteView es una fila de la cola; de quien reporta solo su `usuario_id`.
type ReporteView struct {
	ID            string     `json:"id"`
	NegocioID     string     `json:"negocio_id"`
	Negocio       string     `json:"negocio,omitempty"`
	UsuarioID     string     `json:"usuario_id"`
	Motivo        string     `json:"motivo"`
	Detalle       string     `json:"detalle,omitempty"`
	Estado        string     `json:"estado"`
	ResueltoPor   string     `json:"resuelto_por,omitempty"`
	ResueltoEn    *time.Time `json:"resuelto_en,omitempty"`
	Version       int        `json:"version"`
	CreadoEn      time.Time  `json:"creado_en"`
	ActualizadoEn time.Time  `json:"actualizado_en"`
}

func nuevaVista(r Reporte, nombre string) ReporteView {
	out := ReporteView{
		ID: r.ID.Hex(), NegocioID: r.NegocioID.Hex(), Negocio: nombre, UsuarioID: r.UsuarioID.Hex(), Motivo: r.Motivo,
		Detalle: r.Detalle, Estado: r.Estado, ResueltoEn: r.ResueltoEn, Version: r.Version, CreadoEn: r.CreadoEn, ActualizadoEn: r.ActualizadoEn,
	}
	if r.ResueltoPor != nil {
		out.ResueltoPor = r.ResueltoPor.Hex()
	}
	return out
}

func errStore() error { return apperr.Unavailable("servicio no disponible") }

func invalid(campo, msg string) error {
	return apperr.Validation("datos inválidos", apperr.FieldError{Field: campo, Message: msg})
}

func self(subject string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return id, apperr.Unauthorized("autenticación requerida")
	}
	return id, nil
}

// Crear guarda un reporte (201). Negocio no visible = 404; el dueño = 403; otro abierto en el
// mismo negocio = 409; más de 10 al día = 429; detalle rechazado por moderación = 422.
func (s *Service) Crear(ctx context.Context, subject, rawID string, req CrearRequest) (CreadoResponse, error) {
	me, err := self(subject)
	if err != nil {
		return CreadoResponse{}, err
	}
	negocio, err := s.negocioVisible(ctx, rawID, me)
	if err != nil {
		return CreadoResponse{}, err
	}
	detalle := strings.TrimSpace(req.Detalle)
	if err := s.revisarDetalle(ctx, req.Motivo, detalle); err != nil {
		return CreadoResponse{}, err
	}
	now := s.now().UTC()
	if err := s.dentroDelLimite(ctx, me, now); err != nil {
		return CreadoResponse{}, err
	}
	rep := Reporte{ID: bson.NewObjectID(), NegocioID: negocio, UsuarioID: me, Motivo: req.Motivo, Detalle: detalle,
		Estado: EstadoAbierto, Version: 1, CreadoEn: now, ActualizadoEn: now}
	err = s.repo.Insertar(ctx, &rep)
	if errors.Is(err, ErrDuplicado) {
		return CreadoResponse{}, apperr.Conflict("ya reportaste este lugar; espera a que lo revisemos")
	}
	if err != nil {
		return CreadoResponse{}, errStore()
	}
	return CreadoResponse{ID: rep.ID.Hex(), Estado: rep.Estado}, nil
}

func (s *Service) negocioVisible(ctx context.Context, rawID string, me bson.ObjectID) (bson.ObjectID, error) {
	notFound := apperr.NotFound("lugar no encontrado")
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return id, notFound
	}
	b, ok, err := s.negocios.FindVisible(ctx, id)
	if err != nil {
		return id, errStore()
	}
	if !ok {
		return id, notFound
	}
	if b.OwnerID == me {
		return id, apperr.Forbidden("no puedes reportar tu propio negocio; corrígelo desde tu panel")
	}
	return id, nil
}

// revisarDetalle: `otro` exige detalle y el detalle pasa por la moderación (rechazo = 422).
func (s *Service) revisarDetalle(ctx context.Context, motivo, detalle string) error {
	if motivo == MotivoOtro && detalle == "" {
		return invalid("detalle", "cuéntanos qué está mal")
	}
	if detalle == "" || s.moderador == nil {
		return nil
	}
	if s.moderador.Evaluar(ctx, moderacion.Contenido{Texto: detalle}).Veredicto == moderacion.VeredictoRechazado {
		return invalid("detalle", "el texto no cumple las normas de la comunidad")
	}
	return nil
}

// dentroDelLimite cuenta en MongoDB los reportes del día (hora de México); el 11.º = 429.
func (s *Service) dentroDelLimite(ctx context.Context, me bson.ObjectID, now time.Time) error {
	loc := negocios.MexicoCity()
	n := now.In(loc)
	hoy := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
	count, err := s.repo.DelDia(ctx, me, hoy)
	if err != nil {
		return errStore()
	}
	if count >= MaxPorDia {
		return apperr.RateLimited(int(hoy.AddDate(0, 0, 1).Sub(now).Seconds()) + 1)
	}
	return nil
}

// Cola lista los reportes para el administrador, del más viejo al más nuevo.
func (s *Service) Cola(ctx context.Context, q ColaQuery) (pagination.PageResult[ReporteView], error) {
	if q.Estado == "" {
		q.Estado = EstadoAbierto
	}
	var negocio *bson.ObjectID
	if q.NegocioID != "" {
		id, _ := bson.ObjectIDFromHex(q.NegocioID) // validado por el DTO
		negocio = &id
	}
	scope := s.cursor.Scope("admin_reportes_negocio", q.Estado, q.NegocioID)
	after, err := posicion(s.cursor, scope, q.Cursor)
	if err != nil {
		return pagination.PageResult[ReporteView]{}, err
	}
	rows, err := s.repo.Cola(ctx, q.Estado, negocio, after, q.Limit)
	if err != nil {
		return pagination.PageResult[ReporteView]{}, errStore()
	}
	next := ""
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		last := rows[len(rows)-1]
		if next, err = s.cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: last.CreadoEn.UTC().Format(time.RFC3339Nano), Scope: scope}); err != nil {
			return pagination.PageResult[ReporteView]{}, errStore()
		}
	}
	names, err := s.nombres(ctx, rows)
	if err != nil {
		return pagination.PageResult[ReporteView]{}, err
	}
	items := make([]ReporteView, 0, len(rows))
	for _, r := range rows {
		items = append(items, nuevaVista(r, names[r.NegocioID.Hex()]))
	}
	return pagination.NewPageResult(items, next, q.Limit), nil
}

// nombres trae el nombre de los negocios visibles de una sola vez.
func (s *Service) nombres(ctx context.Context, rows []Reporte) (map[string]string, error) {
	ids := make([]bson.ObjectID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.NegocioID)
	}
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	cards, err := s.negocios.FindCards(ctx, ids, s.now())
	if err != nil {
		return nil, errStore()
	}
	for _, c := range cards {
		out[c.ID] = c.Name
	}
	return out, nil
}

func posicion(signer *pagination.Signer, scope, raw string) (*Posicion, error) {
	c, ok, err := validation.Position(signer, scope, raw)
	if err != nil || !ok {
		return nil, err
	}
	at, errAt := time.Parse(time.RFC3339Nano, c.Sort)
	id, errID := bson.ObjectIDFromHex(c.ID)
	if errAt != nil || errID != nil {
		return nil, invalid("cursor", "cursor inválido")
	}
	return &Posicion{CreadoEn: at, ID: id}, nil
}

// Resolver marca el reporte como corregido o descartado con If-Match; solo desde abierto (409).
// No toca el negocio.
func (s *Service) Resolver(ctx context.Context, subject, rawID string, version int, req ResolverRequest) (ReporteView, error) {
	admin, err := self(subject)
	if err != nil {
		return ReporteView{}, err
	}
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return ReporteView{}, apperr.NotFound("reporte no encontrado")
	}
	rep, err := s.repo.Resolver(ctx, id, version, req.Estado, admin, s.now().UTC())
	switch {
	case errors.Is(err, ErrNoEncontrado):
		return ReporteView{}, apperr.NotFound("reporte no encontrado")
	case errors.Is(err, ErrResuelto):
		return ReporteView{}, apperr.Conflict("el reporte ya se resolvió")
	case errors.Is(err, ErrVersion):
		return ReporteView{}, apperr.Conflict("el reporte cambió; vuelve a leerlo y usa su versión en If-Match")
	case err != nil:
		return ReporteView{}, errStore()
	}
	names, err := s.nombres(ctx, []Reporte{rep})
	if err != nil {
		return ReporteView{}, err
	}
	return nuevaVista(rep, names[rep.NegocioID.Hex()]), nil
}
