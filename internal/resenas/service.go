package resenas

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Negocios es lo que reseñas lee y escribe de `negocios`.
type Negocios interface {
	FindByID(ctx context.Context, id bson.ObjectID) (negocios.Business, error)
	FindVisible(ctx context.Context, id bson.ObjectID) (negocios.Business, bool, error)
	FindVisibleMany(ctx context.Context, ids []bson.ObjectID) ([]negocios.Business, error)
	FindVisibleByQR(ctx context.Context, codigo string) (negocios.Business, bool, error)
	AplicarSuma(ctx context.Context, id bson.ObjectID, d negocios.Suma) error
}

// Fichas borra la ficha del negocio en la caché (nv-18) cuando cambia su calificación.
type Fichas interface {
	InvalidarFicha(ctx context.Context, id string)
}

// Moderador evalúa el texto (nv-42); la API pasa moderacion.Moderador.
type Moderador interface {
	Evaluar(ctx context.Context, c moderacion.Contenido) moderacion.Resultado
}

// Cola recibe lo que queda en cuarentena o se oculta por reportes (nv-43).
type Cola interface {
	Encolar(ctx context.Context, e moderacion.Entrada) (moderacion.Item, error)
}

// Fotos valida que una imagen sea propia y esté publicada (nv-45) y da su URL.
type Fotos interface {
	Publicada(ctx context.Context, dueno bson.ObjectID, id string) (string, error)
}

// Autores da el nombre de usuario y la foto de los autores de la lista pública.
type Autores interface {
	FindPublicByIDs(ctx context.Context, ids []bson.ObjectID) ([]usuarios.User, error)
}

// IA sugiere la respuesta del negocio; la API pasa la ia.Guardia.
type IA interface {
	GenerarValidado(ctx context.Context, pet ia.Peticion, destino any) (ia.Respuesta, error)
}

// Service maneja check-ins y reseñas (nv-62).
type Service struct {
	repo      *Repository
	negocios  Negocios
	fichas    Fichas
	moderador Moderador
	cola      Cola
	fotos     Fotos
	autores   Autores
	ia        IA
	cursor    *pagination.Signer
	loc       *time.Location
	log       *slog.Logger
	now       func() time.Time
}

// NewService recibe sus dependencias por constructor. Sin moderador todo comentario queda en
// moderación; sin fotos, una reseña con fotos da 503; sin IA, sugerir respuesta da 503.
func NewService(repo *Repository, n Negocios, fichas Fichas, autores Autores, cursor *pagination.Signer, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, negocios: n, fichas: fichas, autores: autores, cursor: cursor, loc: negocios.MexicoCity(), log: log, now: time.Now}
}

// WithModeracion conecta el moderador de texto (nv-42) y la cola del administrador (nv-43).
func (s *Service) WithModeracion(m Moderador, c Cola) *Service {
	s.moderador, s.cola = m, c
	return s
}

// WithFotos conecta las imágenes publicadas (nv-45).
func (s *Service) WithFotos(f Fotos) *Service {
	s.fotos = f
	return s
}

// WithIA conecta la IA de la respuesta sugerida (nv-26 a nv-28).
func (s *Service) WithIA(m IA) *Service {
	s.ia = m
	return s
}

// WithReloj fija el reloj (pruebas del plazo de 48 h y de la ventana de 30 días).
func (s *Service) WithReloj(now func() time.Time) *Service {
	s.now = now
	return s
}

// Errores públicos: se crean en cada llamada porque la etapa de errores puede anotarlos.
func errResenaNotFound() error  { return apperr.NotFound("reseña no encontrada") }
func errNegocioNotFound() error { return apperr.NotFound("negocio no encontrado") }
func errStore() error           { return apperr.Unavailable("servicio no disponible") }
func errStale() error {
	return apperr.Conflict("la reseña cambió; vuelve a leerla y usa su versión en If-Match")
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

func parseID(raw string) (bson.ObjectID, bool) {
	id, err := bson.ObjectIDFromHex(raw)
	return id, err == nil
}

func (s *Service) ahora() time.Time { return s.now().UTC().Truncate(time.Millisecond) }

// negocioPropio trae el negocio del comerciante; ajeno, eliminado o inexistente = 404.
func (s *Service) negocioPropio(ctx context.Context, cl *middleware.Claims, raw string) (negocios.Business, bson.ObjectID, error) {
	me, err := self(cl)
	if err != nil {
		return negocios.Business{}, me, err
	}
	id, ok := parseID(raw)
	if !ok {
		return negocios.Business{}, me, errNegocioNotFound()
	}
	b, err := s.negocios.FindByID(ctx, id)
	if errors.Is(err, negocios.ErrNotFound) {
		return negocios.Business{}, me, errNegocioNotFound()
	}
	if err != nil {
		return negocios.Business{}, me, errStore()
	}
	if b.OwnerID != me || b.DeletedAt != nil {
		return negocios.Business{}, me, errNegocioNotFound()
	}
	return b, me, nil
}

// evaluar modera un texto. Sin texto no hay nada que moderar (aprobado); sin moderador queda en
// cuarentena (fail-closed).
func (s *Service) evaluar(ctx context.Context, texto string) moderacion.Resultado {
	if strings.TrimSpace(texto) == "" {
		return moderacion.Resultado{Veredicto: moderacion.VeredictoAprobado}
	}
	if s.moderador == nil {
		return moderacion.Resultado{Veredicto: moderacion.VeredictoCuarentena, Falla: moderacion.FallaSinServicio}
	}
	return s.moderador.Evaluar(ctx, moderacion.Contenido{Texto: texto})
}

func errRechazado() error {
	return invalid("comentario", "el texto no cumple las normas de la comunidad")
}

// textoCola es lo que ve el administrador: el comentario o, si no hay, un resumen.
func textoCola(r Resena) string {
	if r.Comentario != "" {
		return r.Comentario
	}
	return fmt.Sprintf("Reseña sin comentario: %d estrellas; chips: %s", r.Calificacion, strings.Join(r.Chips, ", "))
}

// encolar manda la reseña (o su respuesta) a la cola de nv-43. Si falla, queda en el log: la
// reseña sigue sin publicarse.
func (s *Service) encolar(ctx context.Context, tipo string, ref bson.ObjectID, autor bson.ObjectID, texto string, res moderacion.Resultado) {
	if s.cola == nil {
		s.log.WarnContext(ctx, "reseñas: sin cola de moderación", slog.String("evento", "resena_cola_fallo"), slog.String("id", ref.Hex()))
		return
	}
	res.Veredicto = moderacion.VeredictoCuarentena
	if _, err := s.cola.Encolar(context.WithoutCancel(ctx), moderacion.Entrada{Tipo: tipo, Referencia: ref.Hex(), AutorID: autor, Texto: texto, Resultado: res}); err != nil {
		s.log.WarnContext(ctx, "reseñas: no entró a la cola", slog.String("evento", "resena_cola_fallo"), slog.String("id", ref.Hex()), slog.String("error", err.Error()))
	}
}

// aplicar mueve la calificación del negocio por la diferencia entre antes y después. Si el
// negocio no se actualiza, la reseña vuelve a como estaba (la bandera `contada` nunca queda
// desalineada) y responde 503.
func (s *Service) aplicar(ctx context.Context, antes, despues Resena) error {
	d := despues.Aporte().Menos(antes.Aporte())
	if d.Cero() {
		return nil
	}
	if err := s.negocios.AplicarSuma(ctx, antes.NegocioID, d); err != nil {
		if errRev := s.repo.Revertir(context.WithoutCancel(ctx), antes); errRev != nil {
			s.log.ErrorContext(ctx, "reseñas: calificación y bandera desalineadas", slog.String("evento", "resena_recalculo_fallo"),
				slog.String("id", antes.ID.Hex()), slog.String("error", errRev.Error()))
		}
		return errStore()
	}
	if s.fichas != nil {
		s.fichas.InvalidarFicha(ctx, antes.NegocioID.Hex())
	}
	return nil
}

// publicar pasa la reseña a `publicada` y la cuenta, si estaba en uno de `desde` sin contar.
// ErrNoAplica si ya no estaba ahí.
func (s *Service) publicar(ctx context.Context, id bson.ObjectID, desde ...string) error {
	now := s.ahora()
	antes, err := s.repo.Transicion(ctx, bson.D{
		{Key: "_id", Value: id}, {Key: "estado", Value: bson.D{{Key: "$in", Value: desde}}}, {Key: "contada", Value: false},
	}, bson.D{{Key: "estado", Value: EstadoPublicada}, {Key: "contada", Value: true}, {Key: "moderada_en", Value: now}}, now)
	if err != nil {
		return err
	}
	despues := antes
	despues.Estado, despues.Contada = EstadoPublicada, true
	return s.aplicar(ctx, antes, despues)
}

// retirar pasa la reseña de uno de `desde` a `estado` y la descuenta si estaba contada.
func (s *Service) retirar(ctx context.Context, filter bson.D, estado string) (Resena, error) {
	now := s.ahora()
	set := bson.D{{Key: "estado", Value: estado}, {Key: "contada", Value: false}}
	if estado == EstadoRechazada || estado == EstadoOculta {
		set = append(set, bson.E{Key: "moderada_en", Value: now})
	}
	antes, err := s.repo.Transicion(ctx, filter, set, now)
	if err != nil {
		return Resena{}, err
	}
	despues := antes
	despues.Estado, despues.Contada = estado, false
	return antes, s.aplicar(ctx, antes, despues)
}

func (s *Service) respuesta(r Resena, now time.Time) ResenaResponse {
	out := ResenaResponse{
		ID: r.ID.Hex(), NegocioID: r.NegocioID.Hex(), CheckinID: r.CheckinID.Hex(), Calificacion: r.Calificacion,
		Aspectos: r.Aspectos, Chips: noNil(r.Chips), Comentario: r.Comentario, Fotos: fotosNoNil(r.Fotos), Estado: r.Estado,
		Util: r.Util, Editable: r.Estado == EstadoPublicada && Editable(r.CreadoEn, now), Version: r.Version,
		CreadoEn: r.CreadoEn.UTC(), ActualizadoEn: r.ActualizadoEn.UTC(),
	}
	if rn := r.RespuestaNegocio; rn != nil && rn.Estado == EstadoPublicada {
		out.RespuestaNegocio = &RespuestaView{Texto: rn.Texto, Fecha: rn.Fecha.UTC()}
	}
	return out
}

func noNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func fotosNoNil(v []Foto) []Foto {
	if v == nil {
		return []Foto{}
	}
	return v
}

// fotosDe valida las fotos: imágenes publicadas propias de nv-45.
func (s *Service) fotosDe(ctx context.Context, me bson.ObjectID, ids []string) ([]Foto, error) {
	out := make([]Foto, 0, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	if s.fotos == nil {
		return nil, apperr.Unavailable("imagenes_no_disponibles")
	}
	for _, id := range ids {
		url, err := s.fotos.Publicada(ctx, me, id)
		if errors.Is(err, imagenes.ErrNoPublicada) {
			return nil, invalid("fotos", "cada foto debe ser una imagen propia ya publicada")
		}
		if err != nil {
			return nil, errStore()
		}
		out = append(out, Foto{ID: id, URL: url})
	}
	return out, nil
}
