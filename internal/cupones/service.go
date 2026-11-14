package cupones

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Negocios es lo que cupones lee y escribe de `negocios`.
type Negocios interface {
	FindByID(ctx context.Context, id bson.ObjectID) (negocios.Business, error)
	FindVisible(ctx context.Context, id bson.ObjectID) (negocios.Business, bool, error)
	FindVisibleMany(ctx context.Context, ids []bson.ObjectID) ([]negocios.Business, error)
	FijarCuponesActivos(ctx context.Context, id bson.ObjectID, n int64) error
}

// IA genera el título sugerido; la API pasa la ia.Guardia (tope, caché y límite por usuario).
type IA interface {
	GenerarValidado(ctx context.Context, pet ia.Peticion, destino any) (ia.Respuesta, error)
}

// Service maneja los cupones (nv-61).
type Service struct {
	repo     *Repository
	negocios Negocios
	cursor   *pagination.Signer
	ia       IA
	loc      *time.Location
	log      *slog.Logger
	now      func() time.Time
	codigo   func() (string, error)
}

// NewService recibe sus dependencias por constructor. Sin IA, sugerir título responde 503.
func NewService(repo *Repository, n Negocios, cursor *pagination.Signer, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, negocios: n, cursor: cursor, loc: negocios.MexicoCity(), log: log, now: time.Now, codigo: codigoAleatorio}
}

// WithIA conecta la IA del título sugerido (nv-26 a nv-28).
func (s *Service) WithIA(m IA) *Service {
	s.ia = m
	return s
}

// Errores públicos: se crean en cada llamada porque la etapa de errores puede anotarlos.
func errCuponNotFound() error   { return apperr.NotFound("cupón no encontrado") }
func errNegocioNotFound() error { return apperr.NotFound("negocio no encontrado") }
func errStale() error {
	return apperr.Conflict("el cupón cambió; vuelve a leerlo y usa su versión en If-Match")
}
func errConCanjes() error {
	return apperr.Conflict("con_canjes: el cupón ya tiene canjes; solo cambian vigencia_fin, limite_usos y estado (páusalo en vez de borrarlo)")
}

// errStore no lleva el texto de MongoDB.
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

// negocioPropio exige que el negocio sea del comerciante y no esté eliminado; ajeno = 404.
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

// cuponPropio trae el cupón del negocio propio; otro = 404.
func (s *Service) cuponPropio(ctx context.Context, cl *middleware.Claims, rawNegocio, rawCupon string) (negocios.Business, Cupon, error) {
	b, _, err := s.negocioPropio(ctx, cl, rawNegocio)
	if err != nil {
		return b, Cupon{}, err
	}
	id, ok := parseID(rawCupon)
	if !ok {
		return b, Cupon{}, errCuponNotFound()
	}
	c, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, ErrNotFound) || (err == nil && c.NegocioID != b.ID) {
		return b, Cupon{}, errCuponNotFound()
	}
	if err != nil {
		return b, Cupon{}, errStore()
	}
	return b, c, nil
}

// recalcular escribe `negocios.cupones_activos` con un conteo; si falla lo corrige el trabajo.
func (s *Service) recalcular(ctx context.Context, negocio bson.ObjectID) {
	ctx = context.WithoutCancel(ctx)
	n, err := s.repo.ContarActivos(ctx, negocio, s.now().UTC())
	if err == nil {
		err = s.negocios.FijarCuponesActivos(ctx, negocio, n)
	}
	if err != nil {
		s.log.Warn("cupones: no se recalculó cupones_activos", "evento", "cupones_activos_fallo", "error", err.Error())
	}
}

func parseFecha(campo, raw string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, invalid(campo, "fecha RFC 3339 inválida")
	}
	return t.UTC(), nil
}

// vigencia resuelve las fechas del cupón nuevo: relámpago = ahora a ahora + 3 h.
func vigencia(req CrearRequest, now time.Time) (time.Time, time.Time, error) {
	if req.Relampago {
		return now, now.Add(duracionRelampago), nil
	}
	inicio, err := parseFecha("vigencia_inicio", req.VigenciaInicio)
	if err != nil {
		return inicio, inicio, err
	}
	fin, err := parseFecha("vigencia_fin", req.VigenciaFin)
	if err != nil {
		return inicio, fin, err
	}
	if err := ValidarVigencia(inicio, fin); err != nil {
		return inicio, fin, invalid("vigencia_fin", err.Error())
	}
	if !fin.After(now) {
		return inicio, fin, invalid("vigencia_fin", "la vigencia ya terminó")
	}
	return inicio, fin, nil
}

func validarReglas(tipo string, valor *int64, horas *Horas) error {
	if err := ValidarValor(tipo, valor); err != nil {
		return invalid("valor", err.Error())
	}
	if err := ValidarHoras(horas); err != nil {
		return invalid("horas_validas", err.Error())
	}
	return nil
}

func valorGuardado(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

// Crear guarda un cupón del negocio propio. Programado si empieza después; pausado si el
// negocio está en pausa (vuelve al reanudar).
func (s *Service) Crear(ctx context.Context, cl *middleware.Claims, rawNegocio string, req CrearRequest) (CuponResponse, error) {
	b, _, err := s.negocioPropio(ctx, cl, rawNegocio)
	if err != nil {
		return CuponResponse{}, err
	}
	if err := validarReglas(req.Tipo, req.Valor, req.HorasValidas); err != nil {
		return CuponResponse{}, err
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	inicio, fin, err := vigencia(req, now)
	if err != nil {
		return CuponResponse{}, err
	}
	c := Cupon{
		ID: bson.NewObjectID(), NegocioID: b.ID, Titulo: req.Titulo, Tipo: req.Tipo, Valor: valorGuardado(req.Valor), Condiciones: req.Condiciones,
		VigenciaInicio: inicio, VigenciaFin: fin, Relampago: req.Relampago, DiasValidos: req.DiasValidos, HorasValidas: req.HorasValidas,
		LimiteUsos: req.LimiteUsos, UsosPorUsuario: 1, Ubicacion: Location{Type: b.Location.Type, Coordinates: b.Location.Coordinates},
		Categoria: b.Category, NegocioNombre: b.Name, Estado: EstadoActivo, Version: 1, CreadoEn: now, ActualizadoEn: now,
	}
	if c.DiasValidos == nil {
		c.DiasValidos = []int{}
	}
	if inicio.After(now) {
		c.Estado = EstadoProgramado
	}
	if b.State == negocios.StatusPaused {
		c.Estado, c.PausadoPorNegocio = EstadoPausado, true
	}
	if err := s.repo.Insert(ctx, &c); err != nil {
		return CuponResponse{}, errStore()
	}
	s.recalcular(ctx, b.ID)
	return duenoResponse(c), nil
}

// Lista son los cupones del negocio propio, del más nuevo al más viejo.
func (s *Service) Lista(ctx context.Context, cl *middleware.Claims, rawNegocio string, q ListaNegocioQuery) (pagination.PageResult[CuponResponse], error) {
	b, _, err := s.negocioPropio(ctx, cl, rawNegocio)
	if err != nil {
		return pagination.PageResult[CuponResponse]{}, err
	}
	if s.cursor == nil {
		return pagination.PageResult[CuponResponse]{}, errStore()
	}
	scope := s.cursor.Scope("cupones_negocio", b.ID.Hex(), q.Estado)
	var after *PosLista
	if pos, ok, err := s.posicion(scope, q.Cursor); err != nil {
		return pagination.PageResult[CuponResponse]{}, err
	} else if ok {
		after = &PosLista{Fecha: pos.Fecha, ID: pos.ID}
	}
	cs, err := s.repo.DelNegocio(ctx, b.ID, q.Estado, after, q.Limite)
	if err != nil {
		return pagination.PageResult[CuponResponse]{}, errStore()
	}
	next := ""
	if len(cs) > q.Limite {
		cs = cs[:q.Limite]
		last := cs[len(cs)-1]
		if next, err = s.cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: last.CreadoEn.UTC().Format(time.RFC3339Nano), Scope: scope}); err != nil {
			return pagination.PageResult[CuponResponse]{}, errStore()
		}
	}
	items := make([]CuponResponse, 0, len(cs))
	for _, c := range cs {
		items = append(items, duenoResponse(c))
	}
	return pagination.NewPageResult(items, next, q.Limite), nil
}

// posicion lee un cursor firmado de fecha + id; ajeno o alterado = 422.
func (s *Service) posicion(scope, raw string) (PosCartera, bool, error) {
	c, ok, err := validation.Position(s.cursor, scope, raw)
	if err != nil || !ok {
		return PosCartera{}, false, err
	}
	id, okID := parseID(c.ID)
	fecha, errT := time.Parse(time.RFC3339Nano, c.Sort)
	if !okID || errT != nil {
		return PosCartera{}, false, invalid("cursor", "cursor inválido")
	}
	return PosCartera{Fecha: fecha, ID: id}, true, nil
}

// Borrar elimina un cupón sin canjes; con canjes = 409 (se pausa en su lugar).
func (s *Service) Borrar(ctx context.Context, cl *middleware.Claims, rawNegocio, rawCupon string, version int) error {
	b, c, err := s.cuponPropio(ctx, cl, rawNegocio, rawCupon)
	if err != nil {
		return err
	}
	if c.Version != version {
		return errStale()
	}
	if c.UsosActuales > 0 {
		return errConCanjes()
	}
	if err := s.repo.BorrarSi(ctx, c.ID, b.ID, version); errors.Is(err, ErrNoAplica) {
		return errStale()
	} else if err != nil {
		return errStore()
	}
	s.recalcular(ctx, b.ID)
	return nil
}
