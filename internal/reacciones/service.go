package reacciones

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Momentos decide si quien llama ve el momento y devuelve su autora (lo cumple
// *postales.Service); si no, 404.
type Momentos interface {
	Visible(ctx context.Context, me bson.ObjectID, raw string) (bson.ObjectID, error)
}

// Planes decide si quien llama ve el plan y devuelve su dueño (lo cumple *planes.Service);
// si no, 404.
type Planes interface {
	Visible(ctx context.Context, subject, raw string) (bson.ObjectID, error)
}

// Grupos da los integrantes del plan: «van» son los confirmados (nv-69).
type Grupos interface {
	FindByPlan(ctx context.Context, planID bson.ObjectID) (grupos.Group, error)
}

// Amigos son las relaciones de nv-36 (lo cumple *amistades.Repository).
type Amigos interface {
	List(ctx context.Context, me bson.ObjectID, kind amistades.Kind) ([]amistades.Friendship, error)
	BlockedWith(ctx context.Context, me bson.ObjectID) ([]bson.ObjectID, error)
	RelationsWith(ctx context.Context, me bson.ObjectID, others []bson.ObjectID) (map[bson.ObjectID]amistades.Friendship, error)
	CommonWith(ctx context.Context, mine, others []bson.ObjectID) (map[bson.ObjectID]int, error)
}

// Personas da nombre, foto y privacidad social de las cuentas activas.
type Personas interface {
	FindPublicByIDs(ctx context.Context, ids []bson.ObjectID) ([]usuarios.User, error)
}

// Moderador evalúa textos (nv-42).
type Moderador interface {
	Evaluar(ctx context.Context, c moderacion.Contenido) moderacion.Resultado
}

// Cola recibe lo dudoso (nv-43).
type Cola interface {
	Encolar(ctx context.Context, e moderacion.Entrada) (moderacion.Item, error)
}

// Avisos avisa agrupado por objeto (lo cumple *notificaciones.Agrupador).
type Avisos interface {
	Emit(ctx context.Context, to bson.ObjectID, kind, objeto string, data map[string]string)
}

// Deps son las dependencias obligatorias del service.
type Deps struct {
	Repo     *Repository
	Momentos Momentos
	Planes   Planes
	Grupos   Grupos
	Amigos   Amigos
	Personas Personas
	Cursor   *pagination.Signer
	Log      *slog.Logger
}

// Service maneja me gusta, comentarios y quién reaccionó (nv-83).
type Service struct {
	Deps
	moderador Moderador
	cola      Cola
	avisos    Avisos
	now       func() time.Time
}

// NewService recibe sus dependencias por constructor. Sin moderador todo comentario queda en
// moderación (fail-closed); sin avisos no avisa.
func NewService(d Deps) *Service {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Service{Deps: d, now: time.Now}
}

// WithModeracion conecta el moderador de texto (nv-42) y la cola (nv-43).
func (s *Service) WithModeracion(m Moderador, c Cola) *Service {
	s.moderador, s.cola = m, c
	return s
}

// WithAvisos conecta los avisos agrupados (`me_gusta`, `comentario`).
func (s *Service) WithAvisos(a Avisos) *Service {
	s.avisos = a
	return s
}

func (s *Service) ahora() time.Time { return s.now().UTC().Truncate(time.Millisecond) }

func errStore() error { return apperr.Unavailable("servicio no disponible") }

func errObjetoNotFound(tipo string) error {
	if tipo == ObjetoPlan {
		return apperr.NotFound("plan no encontrado")
	}
	return apperr.NotFound("momento no encontrado")
}

func errComentarioNotFound() error { return apperr.NotFound("comentario no encontrado") }

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

// Ref es un objeto visible para quien llama.
type Ref struct {
	Tipo  string
	ID    bson.ObjectID
	Dueno bson.ObjectID
}

// clave identifica el objeto en los avisos agrupados.
func (r Ref) clave() string { return r.Tipo + ":" + r.ID.Hex() }

// datos es lo que lleva el aviso: solo el id del objeto, como los demás avisos de nv-40.
func (r Ref) datos() map[string]string {
	return map[string]string{r.Tipo + "_id": r.ID.Hex()}
}

// objeto resuelve el momento o el plan si quien llama puede verlo; si no, 404.
func (s *Service) objeto(ctx context.Context, me bson.ObjectID, tipo, raw string) (Ref, error) {
	id, ok := parseID(raw)
	if !ok {
		return Ref{}, errObjetoNotFound(tipo)
	}
	var dueno bson.ObjectID
	var err error
	switch tipo {
	case ObjetoMomento:
		dueno, err = s.Momentos.Visible(ctx, me, raw)
	case ObjetoPlan:
		dueno, err = s.Planes.Visible(ctx, me.Hex(), raw)
	default:
		return Ref{}, errObjetoNotFound(tipo)
	}
	if err != nil {
		return Ref{}, err
	}
	return Ref{Tipo: tipo, ID: id, Dueno: dueno}, nil
}

// avisar manda el aviso al dueño del objeto (nunca a sí mismo).
func (s *Service) avisar(ctx context.Context, r Ref, de bson.ObjectID, kind string) {
	if s.avisos == nil || r.Dueno == de {
		return
	}
	s.avisos.Emit(ctx, r.Dueno, kind, r.clave(), r.datos())
}

// DarMeGusta es PUT /{momentos|planes}/{id}/me-gusta: idempotente; el contador sube una vez.
func (s *Service) DarMeGusta(ctx context.Context, cl *middleware.Claims, tipo, raw string) error {
	me, err := self(cl)
	if err != nil {
		return err
	}
	r, err := s.objeto(ctx, me, tipo, raw)
	if err != nil {
		return err
	}
	nuevo, err := s.Repo.DarMeGusta(ctx, MeGusta{ID: bson.NewObjectID(), ObjetoTipo: r.Tipo, ObjetoID: r.ID, UsuarioID: me, DuenoID: r.Dueno, CreadoEn: s.ahora()})
	if err != nil {
		return errStore()
	}
	if nuevo {
		s.avisar(ctx, r, me, notificaciones.KindLike)
	}
	return nil
}

// QuitarMeGusta es DELETE /{momentos|planes}/{id}/me-gusta: idempotente.
func (s *Service) QuitarMeGusta(ctx context.Context, cl *middleware.Claims, tipo, raw string) error {
	me, err := self(cl)
	if err != nil {
		return err
	}
	r, err := s.objeto(ctx, me, tipo, raw)
	if err != nil {
		return err
	}
	if err := s.Repo.QuitarMeGusta(ctx, r.Tipo, r.ID, me); err != nil {
		return errStore()
	}
	return nil
}

// mirada arma quién mira: sus bloqueos y la privacidad de las personas de la lista.
func (s *Service) mirada(ctx context.Context, me bson.ObjectID, r Ref) (Mirada, error) {
	ids, err := s.Amigos.BlockedWith(ctx, me)
	if err != nil {
		return Mirada{}, errStore()
	}
	m := Mirada{Yo: me, Dueno: r.Dueno, Bloqueados: make(map[bson.ObjectID]bool, len(ids)), Ocultos: map[bson.ObjectID]bool{}}
	for _, id := range ids {
		m.Bloqueados[id] = true
	}
	return m, nil
}

// personas trae las cuentas activas y marca en la mirada a quienes ocultan su actividad.
func (s *Service) personas(ctx context.Context, ids []bson.ObjectID, m *Mirada) (map[bson.ObjectID]usuarios.User, error) {
	out := map[bson.ObjectID]usuarios.User{}
	if len(ids) == 0 {
		return out, nil
	}
	us, err := s.Personas.FindPublicByIDs(ctx, ids)
	if err != nil {
		return nil, errStore()
	}
	for _, u := range us {
		out[u.ID] = u
		if u.Privacy.Social.HidesActivity() {
			m.Ocultos[u.ID] = true
		}
	}
	return out, nil
}

func autorView(u usuarios.User) *AutorView {
	return &AutorView{ID: u.ID.Hex(), Nombre: u.Name, NombreUsuario: u.Username, FotoURL: u.PhotoURL}
}

// textoEstado modera el texto: rechazado = 422 sin guardar; dudoso o sin moderador = en
// moderación (a la cola de nv-43).
func (s *Service) textoEstado(ctx context.Context, texto string) (string, moderacion.Resultado, error) {
	var res moderacion.Resultado
	if s.moderador == nil {
		res = moderacion.Resultado{Veredicto: moderacion.VeredictoCuarentena, Falla: moderacion.FallaSinServicio}
	} else {
		res = s.moderador.Evaluar(ctx, moderacion.Contenido{Texto: texto})
	}
	switch {
	case res.Veredicto == moderacion.VeredictoRechazado:
		return "", res, invalid("texto", "el texto no cumple las normas de la comunidad")
	case res.Publicable():
		return EstadoPublicado, res, nil
	default:
		return EstadoEnModeracion, res, nil
	}
}

// encolar manda el comentario dudoso a la cola; si falla queda en el log (sin texto) y el
// comentario sigue sin publicarse.
func (s *Service) encolar(ctx context.Context, c Comentario, res moderacion.Resultado) {
	if s.cola == nil || c.AutorID == nil {
		s.Log.WarnContext(ctx, "reacciones: sin cola de moderación", slog.String("evento", "comentario_cola_fallo"), slog.String("id", c.ID.Hex()))
		return
	}
	res.Veredicto = moderacion.VeredictoCuarentena
	_, err := s.cola.Encolar(context.WithoutCancel(ctx), moderacion.Entrada{Tipo: TipoCola, Referencia: c.ID.Hex(), AutorID: *c.AutorID, Texto: c.Texto, Resultado: res})
	if err != nil {
		s.Log.WarnContext(ctx, "reacciones: no entró a la cola", slog.String("evento", "comentario_cola_fallo"), slog.String("id", c.ID.Hex()),
			slog.String("error", err.Error()))
	}
}
