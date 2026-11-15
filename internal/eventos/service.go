package eventos

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
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Negocios trae el negocio organizador (nv-20).
type Negocios interface {
	FindByID(ctx context.Context, id bson.ObjectID) (negocios.Business, error)
}

// Moderador evalúa el texto (nv-42).
type Moderador interface {
	Evaluar(ctx context.Context, c moderacion.Contenido) moderacion.Resultado
}

// Cola guarda lo dudoso para que lo revise un administrador (nv-43).
type Cola interface {
	Encolar(ctx context.Context, e moderacion.Entrada) (moderacion.Item, error)
}

// Service maneja los eventos (nv-65).
type Service struct {
	repo      *Repository
	negocios  Negocios
	cursor    *pagination.Signer
	moderador Moderador
	cola      Cola
	fotos     string
	log       *slog.Logger
	now       func() time.Time
}

// NewService recibe sus dependencias por constructor. `fotos` es la URL pública de las
// imágenes de nv-45; sin ella no se aceptan fotos.
func NewService(repo *Repository, n Negocios, cursor *pagination.Signer, fotos string, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, negocios: n, cursor: cursor, fotos: strings.TrimRight(fotos, "/"), log: log, now: time.Now}
}

// WithModeracion conecta el moderador de texto y la cola.
func (s *Service) WithModeracion(m Moderador, c Cola) *Service {
	s.moderador, s.cola = m, c
	return s
}

func errNoEncontrado() error { return apperr.NotFound("evento no encontrado") }
func errStore() error        { return apperr.Unavailable("servicio no disponible") }
func errVersion() error {
	return apperr.Conflict("el evento cambió; vuelve a leerlo y usa su versión en If-Match")
}

func invalid(campo, msg string) error {
	return apperr.Validation("datos inválidos", apperr.FieldError{Field: campo, Message: msg})
}

func quien(cl *middleware.Claims) (bson.ObjectID, error) {
	if cl == nil {
		return bson.ObjectID{}, apperr.Unauthorized("autenticación requerida")
	}
	id, err := bson.ObjectIDFromHex(cl.Subject)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("autenticación requerida")
	}
	return id, nil
}

// Lista devuelve los eventos visibles cercanos que empiezan entre hoy y hoy + `dias` (día de
// México) y no han terminado, por inicio y distancia.
func (s *Service) Lista(ctx context.Context, q ListaQuery) (pagination.PageResult[EventoView], error) {
	now := s.now()
	loc := negocios.MexicoCity()
	n := now.In(loc)
	hoy := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
	f := Filtro{Lat: *q.Lat, Lng: *q.Lng, RadioM: float64(q.RadioM), Desde: hoy, Hasta: hoy.AddDate(0, 0, q.Dias+1), Ahora: now, Tipo: q.Tipo}
	scope := s.cursor.Scope("eventos", strconv.FormatFloat(f.Lat, 'f', -1, 64), strconv.FormatFloat(f.Lng, 'f', -1, 64), strconv.Itoa(q.RadioM), strconv.Itoa(q.Dias), q.Tipo)
	after, err := posicion(s.cursor, scope, q.Cursor)
	if err != nil {
		return pagination.PageResult[EventoView]{}, err
	}
	filas, err := s.repo.Cercanos(ctx, f, after, q.Limit)
	if err != nil {
		return pagination.PageResult[EventoView]{}, errStore()
	}
	next := ""
	if len(filas) > q.Limit {
		filas = filas[:q.Limit]
		last := filas[len(filas)-1]
		sort := last.Inicio.UTC().Format(time.RFC3339Nano) + "|" + strconv.FormatFloat(last.DistanciaM, 'g', -1, 64)
		if next, err = s.cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: sort, Scope: scope}); err != nil {
			return pagination.PageResult[EventoView]{}, errStore()
		}
	}
	items := make([]EventoView, 0, len(filas))
	for _, fila := range filas {
		v := nuevaVista(fila.Evento)
		d := fila.DistanciaM
		v.DistanciaM = &d
		items = append(items, v)
	}
	return pagination.NewPageResult(items, next, q.Limit), nil
}

// posicion lee el cursor firmado (`inicio|distancia`); uno manipulado o de otra consulta es 422.
func posicion(signer *pagination.Signer, scope, raw string) (*Posicion, error) {
	c, ok, err := validation.Position(signer, scope, raw)
	if err != nil || !ok {
		return nil, err
	}
	bad := invalid("cursor", "cursor inválido")
	inicio, dist, found := strings.Cut(c.Sort, "|")
	at, errAt := time.Parse(time.RFC3339Nano, inicio)
	d, errD := strconv.ParseFloat(dist, 64)
	id, errID := bson.ObjectIDFromHex(c.ID)
	if !found || errAt != nil || errD != nil || errID != nil {
		return nil, bad
	}
	return &Posicion{Inicio: at, DistanciaM: d, ID: id}, nil
}

// Detalle da un evento visible; borrador, cancelado o en revisión solo para quien puede
// editarlo (los demás, 404).
func (s *Service) Detalle(ctx context.Context, cl *middleware.Claims, raw string) (EventoView, error) {
	e, err := s.buscar(ctx, raw)
	if err != nil {
		return EventoView{}, err
	}
	if !e.Visible() {
		ok, err := s.puedeEditar(ctx, cl, e)
		if err != nil {
			return EventoView{}, err
		}
		if !ok {
			return EventoView{}, errNoEncontrado()
		}
	}
	return nuevaVista(e), nil
}

func (s *Service) buscar(ctx context.Context, raw string) (Evento, error) {
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return Evento{}, errNoEncontrado()
	}
	e, err := s.repo.Buscar(ctx, id)
	if errors.Is(err, ErrNoEncontrado) {
		return Evento{}, errNoEncontrado()
	}
	if err != nil {
		return Evento{}, errStore()
	}
	return e, nil
}

// puedeEditar: administrador, quien lo creó o el dueño del negocio organizador.
func (s *Service) puedeEditar(ctx context.Context, cl *middleware.Claims, e Evento) (bool, error) {
	if cl == nil {
		return false, nil
	}
	if esAdmin(cl) {
		return true, nil
	}
	me, err := quien(cl)
	if err != nil {
		return false, nil
	}
	if e.CreadoPor != nil && *e.CreadoPor == me {
		return true, nil
	}
	if e.NegocioID == nil {
		return false, nil
	}
	b, err := s.negocios.FindByID(ctx, *e.NegocioID)
	if errors.Is(err, negocios.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, errStore()
	}
	return b.OwnerID == me && b.DeletedAt == nil, nil
}

// negocioPublicable exige un negocio propio, aprobado, sin pausa ni baja (comerciante); para
// un administrador basta que exista. Si no, 403.
func (s *Service) negocioPublicable(ctx context.Context, cl *middleware.Claims, me bson.ObjectID, raw string) (*bson.ObjectID, error) {
	admin := esAdmin(cl)
	forbidden := apperr.Forbidden("solo publicas eventos de tu negocio aprobado")
	if raw == "" {
		if admin {
			return nil, nil
		}
		return nil, invalid("negocio_id", "el comerciante publica para uno de sus negocios")
	}
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return nil, forbidden
	}
	b, err := s.negocios.FindByID(ctx, id)
	if errors.Is(err, negocios.ErrNotFound) {
		return nil, forbidden
	}
	if err != nil {
		return nil, errStore()
	}
	if !admin && !publicable(b, me) {
		return nil, forbidden
	}
	return &id, nil
}

// publicable: el negocio es de me, está aprobado y no está en pausa ni dado de baja.
func publicable(b negocios.Business, me bson.ObjectID) bool {
	return b.OwnerID == me && b.State == negocios.StatusApproved && b.Paused == nil && b.DeletedAt == nil
}

// fechas valida inicio y fin: fin > inicio, inicio desde hace 1 h hasta 180 días y duración de
// hasta 30 días.
func (s *Service) fechas(inicio, fin time.Time) error {
	now := s.now()
	switch {
	case !fin.After(inicio):
		return invalid("fin", "fin va después de inicio")
	case inicio.Before(now.Add(-toleranciaPasado)) || inicio.After(now.Add(maxAdelanto)):
		return invalid("inicio", "inicio va desde hace una hora hasta 180 días adelante")
	case fin.Sub(inicio) > maxDuracion:
		return invalid("fin", "un evento dura hasta 30 días")
	}
	return nil
}

func parseHora(campo, raw string) (time.Time, error) {
	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, invalid(campo, "usa una fecha y hora RFC 3339")
	}
	return at.UTC(), nil
}

// foto solo acepta URLs de las imágenes de nv-45.
func (s *Service) foto(raw string) error {
	if raw == "" || (s.fotos != "" && strings.HasPrefix(raw, s.fotos+"/")) {
		return nil
	}
	return invalid("foto_url", "la foto debe subirse con la API de imágenes")
}

// Crear publica un evento (201). Texto rechazado por la moderación = 422; dudoso = guardado en
// revisión y en la cola.
func (s *Service) Crear(ctx context.Context, cl *middleware.Claims, req CrearRequest) (EventoView, error) {
	me, err := quien(cl)
	if err != nil {
		return EventoView{}, err
	}
	negocio, err := s.negocioPublicable(ctx, cl, me, req.NegocioID)
	if err != nil {
		return EventoView{}, err
	}
	e, err := s.nuevo(req)
	if err != nil {
		return EventoView{}, err
	}
	now := s.now().UTC()
	e.ID, e.NegocioID, e.CreadoPor, e.Version, e.CreadoEn, e.ActualizadoEn = bson.NewObjectID(), negocio, &me, 1, now, now
	res, err := s.moderar(ctx, e)
	if err != nil {
		return EventoView{}, err
	}
	e.EnRevision = res.Veredicto == moderacion.VeredictoCuarentena
	if err := s.repo.Insertar(ctx, &e); err != nil {
		return EventoView{}, errStore()
	}
	s.encolar(ctx, e, me, res)
	return nuevaVista(e), nil
}

func (s *Service) nuevo(req CrearRequest) (Evento, error) {
	inicio, err := parseHora("inicio", req.Inicio)
	if err != nil {
		return Evento{}, err
	}
	fin, err := parseHora("fin", req.Fin)
	if err != nil {
		return Evento{}, err
	}
	if err := s.fechas(inicio, fin); err != nil {
		return Evento{}, err
	}
	if err := s.foto(req.FotoURL); err != nil {
		return Evento{}, err
	}
	estado := req.Estado
	if estado == "" {
		estado = EstadoPublicado
	}
	e := Evento{
		Titulo: strings.TrimSpace(req.Titulo), Tipo: req.Tipo, Descripcion: strings.TrimSpace(req.Descripcion),
		Ubicacion: nuevoPunto(*req.Ubicacion.Lat, *req.Ubicacion.Lng), Direccion: strings.TrimSpace(req.Direccion),
		Inicio: inicio, Fin: fin, Precio: *req.Precio, FotoURL: req.FotoURL, Estado: estado,
	}
	if e.Titulo == "" || e.Direccion == "" {
		return Evento{}, invalid("titulo", "titulo y direccion no pueden ir vacíos")
	}
	return e, nil
}

// moderar evalúa título y descripción (nv-42): rechazado = 422, dudoso = en revisión.
func (s *Service) moderar(ctx context.Context, e Evento) (moderacion.Resultado, error) {
	texto := strings.TrimSpace(e.Titulo + "\n" + e.Descripcion)
	res := moderacion.Resultado{Veredicto: moderacion.VeredictoCuarentena, Falla: moderacion.FallaSinServicio}
	if s.moderador != nil {
		res = s.moderador.Evaluar(ctx, moderacion.Contenido{Texto: texto})
	}
	if res.Veredicto == moderacion.VeredictoRechazado {
		return res, invalid("titulo", "el texto no cumple las normas de la comunidad")
	}
	return res, nil
}

// encolar deja el evento dudoso en la cola de moderación; si la cola falla, queda en revisión
// y se registra (sin texto).
func (s *Service) encolar(ctx context.Context, e Evento, autor bson.ObjectID, res moderacion.Resultado) {
	if !e.EnRevision || s.cola == nil {
		return
	}
	texto := strings.TrimSpace(e.Titulo + "\n" + e.Descripcion)
	if _, err := s.cola.Encolar(ctx, moderacion.Entrada{Tipo: TipoCola, Referencia: e.ID.Hex(), AutorID: autor, Texto: texto, Resultado: res}); err != nil {
		s.log.WarnContext(ctx, "eventos: no se encoló para moderación", slog.String("evento", "evento_cola_fallida"), slog.String("id", e.ID.Hex()))
	}
}

func esAdmin(cl *middleware.Claims) bool {
	return cl != nil && slices.Contains(cl.Roles, middleware.RoleAdmin)
}
