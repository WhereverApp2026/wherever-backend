package postales

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Imagenes consulta las imágenes propias de nv-45.
type Imagenes interface {
	Consultar(ctx context.Context, dueno bson.ObjectID, id string) (imagenes.Vista, error)
}

// Amigos son las relaciones de nv-36.
type Amigos interface {
	Relation(ctx context.Context, me, other bson.ObjectID) (status string, mine bool, err error)
	List(ctx context.Context, me bson.ObjectID, kind amistades.Kind) ([]amistades.Friendship, error)
	BlockedWith(ctx context.Context, me bson.ObjectID) ([]bson.ObjectID, error)
}

// Planes lee los planes activos (nv-30).
type Planes interface {
	FindActive(ctx context.Context, id bson.ObjectID) (planes.Plan, error)
}

// Grupos dice quién está en el grupo de un plan (nv-37).
type Grupos interface {
	FindByPlan(ctx context.Context, planID bson.ObjectID) (grupos.Group, error)
}

// Negocios lee los negocios visibles y su propietario.
type Negocios interface {
	FindByID(ctx context.Context, id bson.ObjectID) (negocios.Business, error)
	FindVisible(ctx context.Context, id bson.ObjectID) (negocios.Business, bool, error)
	// FindFavoriteCards trae nombre y fotos sin filtrar por estado: el recuerdo copia la parada
	// aunque el lugar haya cerrado después.
	FindFavoriteCards(ctx context.Context, ids []bson.ObjectID) ([]negocios.Business, error)
}

// Autores da nombre de usuario y foto de las cuentas activas.
type Autores interface {
	FindPublicByIDs(ctx context.Context, ids []bson.ObjectID) ([]usuarios.User, error)
}

// Moderador evalúa textos (nv-42); la API pasa moderacion.Moderador.
type Moderador interface {
	Evaluar(ctx context.Context, c moderacion.Contenido) moderacion.Resultado
}

// Cola recibe lo dudoso (nv-43).
type Cola interface {
	Encolar(ctx context.Context, e moderacion.Entrada) (moderacion.Item, error)
}

// Deps son las dependencias obligatorias del service.
type Deps struct {
	Repo     *Repository
	Amigos   Amigos
	Planes   Planes
	Grupos   Grupos
	Negocios Negocios
	Autores  Autores
	Cursor   *pagination.Signer
	Log      *slog.Logger
}

// Service maneja postales, recuerdos y momentos (nv-63).
type Service struct {
	Deps
	imagenes  Imagenes
	moderador Moderador
	cola      Cola
	notifier  notificaciones.Notifier
	loc       *time.Location
	now       func() time.Time
}

// NewService recibe sus dependencias por constructor. Sin imágenes (sin IMAGENES_*) crear una
// postal o fijar la tarjeta responde 503; sin moderador todo texto queda en revisión.
func NewService(d Deps) *Service {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Service{Deps: d, notifier: notificaciones.Noop{}, loc: negocios.MexicoCity(), now: time.Now}
}

// WithImagenes conecta las imágenes de nv-45.
func (s *Service) WithImagenes(i Imagenes) *Service {
	s.imagenes = i
	return s
}

// WithModeracion conecta el moderador de texto (nv-42) y la cola (nv-43).
func (s *Service) WithModeracion(m Moderador, c Cola) *Service {
	s.moderador, s.cola = m, c
	return s
}

// WithNotifier conecta los avisos (nv-40 y nv-41).
func (s *Service) WithNotifier(n notificaciones.Notifier) *Service {
	if n != nil {
		s.notifier = n
	}
	return s
}

// WithReloj fija el reloj (pruebas de la fecha del plan).
func (s *Service) WithReloj(now func() time.Time) *Service {
	s.now = now
	return s
}

func (s *Service) ahora() time.Time { return s.now().UTC().Truncate(time.Millisecond) }

// Errores públicos.
func errPostalNotFound() error   { return apperr.NotFound("postal no encontrada") }
func errMomentoNotFound() error  { return apperr.NotFound("momento no encontrado") }
func errRecuerdoNotFound() error { return apperr.NotFound("recuerdo no encontrado") }
func errNegocioNotFound() error  { return apperr.NotFound("negocio no encontrado") }
func errPlanNotFound() error     { return apperr.NotFound("plan no encontrado") }
func errStore() error            { return apperr.Unavailable("servicio no disponible") }
func errSinImagenes() error      { return apperr.Unavailable("imagenes_no_disponibles") }
func errStale() error {
	return apperr.Conflict("cambió desde que lo leíste; vuelve a leerlo y usa su versión en If-Match")
}

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

// quien es la persona que llama si trae token; sin token, nil (rutas con sesión opcional).
func quien(cl *middleware.Claims) *bson.ObjectID {
	id, err := self(cl)
	if err != nil {
		return nil
	}
	return &id
}

func parseID(raw string) (bson.ObjectID, bool) {
	id, err := bson.ObjectIDFromHex(raw)
	return id, err == nil
}

func hexPtr(id *bson.ObjectID) *string {
	if id == nil {
		return nil
	}
	h := id.Hex()
	return &h
}

// relacion es lo que hay entre quien mira (nil = sin sesión) y el dueño.
func (s *Service) relacion(ctx context.Context, viewer *bson.ObjectID, dueno bson.ObjectID) (Relacion, error) {
	if viewer == nil {
		return Relacion{}, nil
	}
	if *viewer == dueno {
		return Relacion{Dueno: true}, nil
	}
	status, _, err := s.Amigos.Relation(ctx, *viewer, dueno)
	if err != nil {
		return Relacion{}, errStore()
	}
	return Relacion{Amigos: status == amistades.StatusAccepted, Bloqueado: status == amistades.StatusBlocked}, nil
}

// bloqueados son las personas con bloqueo con quien mira (vacío sin sesión).
func (s *Service) bloqueados(ctx context.Context, viewer *bson.ObjectID) (map[bson.ObjectID]bool, error) {
	out := map[bson.ObjectID]bool{}
	if viewer == nil {
		return out, nil
	}
	ids, err := s.Amigos.BlockedWith(ctx, *viewer)
	if err != nil {
		return nil, errStore()
	}
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

// autores resuelve nombre de usuario y foto; las cuentas no activas no aparecen.
func (s *Service) autores(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID]AutorView, error) {
	out := map[bson.ObjectID]AutorView{}
	if len(ids) == 0 {
		return out, nil
	}
	us, err := s.Autores.FindPublicByIDs(ctx, ids)
	if err != nil {
		return nil, errStore()
	}
	for _, u := range us {
		out[u.ID] = AutorView{ID: u.ID.Hex(), NombreUsuario: u.Username, FotoURL: u.PhotoURL}
	}
	return out, nil
}

// planVisible trae el plan si quien llama es su dueño o integrante confirmado; si no, 404.
func (s *Service) planVisible(ctx context.Context, me, id bson.ObjectID) (planes.Plan, error) {
	p, err := s.Planes.FindActive(ctx, id)
	if errors.Is(err, planes.ErrNotFound) {
		return planes.Plan{}, errPlanNotFound()
	}
	if err != nil {
		return planes.Plan{}, errStore()
	}
	if p.OwnerID == me {
		return p, nil
	}
	g, err := s.Grupos.FindByPlan(ctx, id)
	if errors.Is(err, grupos.ErrNotFound) {
		return planes.Plan{}, errPlanNotFound()
	}
	if err != nil {
		return planes.Plan{}, errStore()
	}
	for _, m := range g.Members {
		if m.UserID == me && m.Status == grupos.StatusConfirmed {
			return p, nil
		}
	}
	return planes.Plan{}, errPlanNotFound()
}

// evaluar modera un texto. Vacío = aprobado; sin moderador queda en revisión (fail-closed).
func (s *Service) evaluar(ctx context.Context, texto string) moderacion.Resultado {
	if strings.TrimSpace(texto) == "" {
		return moderacion.Resultado{Veredicto: moderacion.VeredictoAprobado}
	}
	if s.moderador == nil {
		return moderacion.Resultado{Veredicto: moderacion.VeredictoCuarentena, Falla: moderacion.FallaSinServicio}
	}
	return s.moderador.Evaluar(ctx, moderacion.Contenido{Texto: texto})
}

// textoEstado traduce el veredicto: rechazado = 422 sin guardar.
func (s *Service) textoEstado(ctx context.Context, campo, texto string) (string, moderacion.Resultado, error) {
	res := s.evaluar(ctx, texto)
	switch {
	case res.Veredicto == moderacion.VeredictoRechazado:
		return "", res, invalid(campo, "el texto no cumple las normas de la comunidad")
	case res.Publicable():
		return TextoAprobado, res, nil
	default:
		return TextoEnRevision, res, nil
	}
}

// encolar manda el texto dudoso a la cola de nv-43; si falla queda en el log (sin texto) y el
// contenido sigue sin publicarse.
func (s *Service) encolar(ctx context.Context, tipo string, ref, autor bson.ObjectID, texto string, res moderacion.Resultado) {
	if s.cola == nil {
		s.Log.WarnContext(ctx, "postales: sin cola de moderación", slog.String("evento", "postal_cola_fallo"), slog.String("tipo", tipo), slog.String("id", ref.Hex()))
		return
	}
	res.Veredicto = moderacion.VeredictoCuarentena
	if _, err := s.cola.Encolar(context.WithoutCancel(ctx), moderacion.Entrada{Tipo: tipo, Referencia: ref.Hex(), AutorID: autor, Texto: texto, Resultado: res}); err != nil {
		s.Log.WarnContext(ctx, "postales: no entró a la cola", slog.String("evento", "postal_cola_fallo"), slog.String("tipo", tipo),
			slog.String("id", ref.Hex()), slog.String("error", err.Error()))
	}
}

// posicion lee el cursor firmado; uno de otra lista o manipulado = 422.
func (s *Service) posicion(scope, raw string) (*Pos, error) {
	c, ok, err := validation.Position(s.Cursor, scope, raw)
	if err != nil || !ok {
		return nil, err
	}
	id, okID := parseID(c.ID)
	t, errT := time.Parse(time.RFC3339Nano, c.Sort)
	if !okID || errT != nil {
		return nil, invalid("cursor", "cursor inválido")
	}
	return &Pos{Fecha: t, ID: id}, nil
}

func (s *Service) siguiente(scope string, id bson.ObjectID, t time.Time) (string, error) {
	next, err := s.Cursor.Encode(pagination.Cursor{ID: id.Hex(), Sort: t.UTC().Format(time.RFC3339Nano), Scope: scope})
	if err != nil {
		return "", errStore()
	}
	return next, nil
}

// pagina recorta a `limite` y arma el cursor del último.
func pagina[T any](s *Service, rows []T, limite int, scope string, clave func(T) (bson.ObjectID, time.Time)) ([]T, string, error) {
	if len(rows) <= limite {
		return rows, "", nil
	}
	rows = rows[:limite]
	id, t := clave(rows[len(rows)-1])
	next, err := s.siguiente(scope, id, t)
	return rows, next, err
}
