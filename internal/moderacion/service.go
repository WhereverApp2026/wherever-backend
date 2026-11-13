package moderacion

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// ErrEntradaInvalida es una Entrada que no cabe en la cola (veredicto que no es cuarentena,
// sin tipo, referencia, autor o contenido, o campos demasiado largos).
var ErrEntradaInvalida = errors.New("moderacion: entrada inválida para la cola")

// Service atiende la cola de revisión y las reglas. Cumple FuenteReglas: el Moderador lee las
// reglas guardadas en cada evaluación, así se ajustan sin recompilar ni reiniciar.
type Service struct {
	repo     *Repository
	cursor   *pagination.Signer
	destinos map[string]Destino
	log      *slog.Logger
	now      func() time.Time
}

// NewService recibe sus dependencias por constructor; sin cursor la lista falla cerrada.
func NewService(repo *Repository, cursor *pagination.Signer, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, cursor: cursor, destinos: map[string]Destino{}, log: log, now: time.Now}
}

// WithDestino registra quién publica o descarta los recursos de un tipo al decidirse. Se llama
// al armar la app, antes de atender peticiones.
func (s *Service) WithDestino(tipo string, d Destino) *Service {
	s.destinos[tipo] = d
	return s
}

func errStore() error { return apperr.Unavailable("servicio no disponible") }

func errNoEncontrado() error { return apperr.NotFound("elemento de moderación no encontrado") }

func revisor(subject string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("autenticación requerida")
	}
	return id, nil
}

// Reglas cumple FuenteReglas: las guardadas o, si nunca se guardaron, las por defecto. Si la
// base falla devuelve el error y el Moderador deja el contenido en cuarentena.
func (s *Service) Reglas(ctx context.Context) (Reglas, error) {
	d, err := s.repo.LeerReglas(ctx)
	if errors.Is(err, ErrNoEncontrado) {
		return ReglasPorDefecto(), nil
	}
	if err != nil {
		return Reglas{}, err
	}
	return d.reglas(), nil
}

// Encolar guarda lo que quedó en cuarentena para que lo revise un administrador. Si el recurso
// ya espera revisión devuelve el elemento existente.
func (s *Service) Encolar(ctx context.Context, e Entrada) (Item, error) {
	if err := validarEntrada(e); err != nil {
		return Item{}, err
	}
	puntajes := e.Resultado.Puntajes
	if puntajes == nil {
		puntajes = map[string]float64{}
	}
	it := Item{
		ID: bson.NewObjectID(), Tipo: e.Tipo, Referencia: e.Referencia, AutorID: e.AutorID,
		Texto: e.Texto, ImagenClave: e.ImagenClave, Veredicto: e.Resultado.Veredicto,
		Puntajes: puntajes, Falla: e.Resultado.Falla, Estado: EstadoPendiente,
		CreadoEn: s.now().UTC().Truncate(time.Millisecond), Version: 1,
	}
	return s.repo.Insertar(ctx, it)
}

func validarEntrada(e Entrada) error {
	switch {
	case e.Resultado.Veredicto != VeredictoCuarentena,
		e.Tipo == "" || utf8.RuneCountInString(e.Tipo) > MaxRunasTipo,
		e.Referencia == "" || utf8.RuneCountInString(e.Referencia) > MaxRunasRefOrig,
		e.AutorID.IsZero(),
		e.Texto == "" && e.ImagenClave == "",
		utf8.RuneCountInString(e.Texto) > MaxRunasTexto,
		utf8.RuneCountInString(e.ImagenClave) > MaxRunasImgClave:
		return ErrEntradaInvalida
	}
	return nil
}

// Listar devuelve una página de la cola en orden de llegada.
func (s *Service) Listar(ctx context.Context, q ListaQuery) (pagination.PageResult[Resumen], error) {
	if q.Estado == "" {
		q.Estado = EstadoPendiente
	}
	scope := s.cursor.Scope("moderacion", q.Estado, q.Tipo)
	after, err := posicion(s.cursor, scope, q.Cursor)
	if err != nil {
		return pagination.PageResult[Resumen]{}, err
	}
	rows, err := s.repo.Listar(ctx, q.Estado, q.Tipo, after, q.Limit)
	if err != nil {
		return pagination.PageResult[Resumen]{}, errStore()
	}
	next := ""
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		last := rows[len(rows)-1]
		next, err = s.cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: last.CreadoEn.UTC().Format(time.RFC3339Nano), Scope: scope})
		if err != nil {
			return pagination.PageResult[Resumen]{}, errStore()
		}
	}
	items := make([]Resumen, 0, len(rows))
	for _, it := range rows {
		items = append(items, nuevoResumen(it))
	}
	return pagination.NewPageResult(items, next, q.Limit), nil
}

func posicion(signer *pagination.Signer, scope, raw string) (*Posicion, error) {
	c, ok, err := validation.Position(signer, scope, raw)
	if err != nil || !ok {
		return nil, err
	}
	bad := apperr.Validation("cursor inválido", apperr.FieldError{Field: "cursor", Message: "cursor inválido"})
	at, err := time.Parse(time.RFC3339Nano, c.Sort)
	if err != nil {
		return nil, bad
	}
	id, err := bson.ObjectIDFromHex(c.ID)
	if err != nil {
		return nil, bad
	}
	return &Posicion{CreadoEn: at, ID: id}, nil
}

// Detalle trae un elemento con sus puntajes y su decisión.
func (s *Service) Detalle(ctx context.Context, id string) (Detalle, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return Detalle{}, errNoEncontrado()
	}
	it, err := s.repo.Buscar(ctx, oid)
	if errors.Is(err, ErrNoEncontrado) {
		return Detalle{}, errNoEncontrado()
	}
	if err != nil {
		return Detalle{}, errStore()
	}
	return nuevoDetalle(it), nil
}

// Decidir aprueba o rechaza un elemento pendiente y registra quién y cuándo. Una segunda
// decisión, o una versión vieja, es 409. Después avisa al destino del tipo; si el destino
// falla, la decisión queda guardada y el recurso sigue sin publicarse (cierra ante fallos).
func (s *Service) Decidir(ctx context.Context, subject, id string, version int, req DecisionRequest) (Detalle, error) {
	who, err := revisor(subject)
	if err != nil {
		return Detalle{}, err
	}
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return Detalle{}, errNoEncontrado()
	}
	motivo := strings.TrimSpace(req.Motivo)
	if req.Decision == EstadoRechazado && motivo == "" {
		return Detalle{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "motivo", Message: "obligatorio al rechazar"})
	}
	it, err := s.repo.Decidir(ctx, oid, version, req.Decision, motivo, who, s.now().UTC().Truncate(time.Millisecond))
	switch {
	case errors.Is(err, ErrNoEncontrado):
		return Detalle{}, errNoEncontrado()
	case errors.Is(err, ErrYaDecidido):
		return Detalle{}, apperr.Conflict("el elemento ya se decidió o cambió; vuelve a cargarlo")
	case err != nil:
		return Detalle{}, errStore()
	}
	if d, ok := s.destinos[it.Tipo]; ok {
		if err := d.AlDecidir(ctx, it); err != nil {
			s.log.WarnContext(ctx, "moderación: el destino no aplicó la decisión",
				slog.String("evento", "moderacion_destino_fallo"), slog.String("tipo", it.Tipo), slog.String("id", it.ID.Hex()), slog.String("error", err.Error()))
		}
	}
	return nuevoDetalle(it), nil
}

// VerReglas devuelve las reglas vigentes (las por defecto con version 0 si nunca se guardaron).
func (s *Service) VerReglas(ctx context.Context) (ReglasView, error) {
	d, err := s.repo.LeerReglas(ctx)
	if errors.Is(err, ErrNoEncontrado) {
		return reglasPorDefectoView(), nil
	}
	if err != nil {
		return ReglasView{}, errStore()
	}
	return nuevaReglasView(d), nil
}

// CrearReglas guarda las primeras reglas; 409 si ya existen (se ajustan con PATCH).
func (s *Service) CrearReglas(ctx context.Context, subject string, req ReglasRequest) (ReglasView, error) {
	who, err := revisor(subject)
	if err != nil {
		return ReglasView{}, err
	}
	d, err := s.armarReglas(who, Reglas(req))
	if err != nil {
		return ReglasView{}, err
	}
	d.Version = 1
	switch err := s.repo.CrearReglas(ctx, d); {
	case errors.Is(err, ErrYaExisten):
		return ReglasView{}, apperr.Conflict("las reglas ya existen; ajústalas con PATCH")
	case err != nil:
		return ReglasView{}, errStore()
	}
	return nuevaReglasView(d), nil
}

// AjustarReglas cambia las reglas guardadas si siguen en la versión dada.
func (s *Service) AjustarReglas(ctx context.Context, subject string, version int, req ReglasPatch) (ReglasView, error) {
	who, err := revisor(subject)
	if err != nil {
		return ReglasView{}, err
	}
	actual, err := s.repo.LeerReglas(ctx)
	if errors.Is(err, ErrNoEncontrado) {
		return ReglasView{}, apperr.NotFound("no hay reglas guardadas; créalas con POST")
	}
	if err != nil {
		return ReglasView{}, errStore()
	}
	d, err := s.armarReglas(who, req.aplicar(actual.reglas()))
	if err != nil {
		return ReglasView{}, err
	}
	out, err := s.repo.ActualizarReglas(ctx, version, d)
	switch {
	case errors.Is(err, ErrNoEncontrado):
		return ReglasView{}, apperr.NotFound("no hay reglas guardadas; créalas con POST")
	case errors.Is(err, ErrYaDecidido):
		return ReglasView{}, apperr.Conflict("las reglas cambiaron; vuelve a cargarlas")
	case err != nil:
		return ReglasView{}, errStore()
	}
	return nuevaReglasView(out), nil
}

// armarReglas valida y normaliza: palabras sin espacios sobrantes, sin repetidas (sin acentos ni
// mayúsculas) y con el orden en que llegaron.
func (s *Service) armarReglas(who bson.ObjectID, r Reglas) (reglasDoc, error) {
	if err := r.Validar(); err != nil {
		return reglasDoc{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "cuarentena",
			Message: fmt.Sprintf("se pide %v <= cuarentena < rechazo <= 100", CuarentenaMinima)})
	}
	palabras := make([]string, 0, len(r.Palabras))
	vistas := map[string]bool{}
	for _, p := range r.Palabras {
		p = strings.Join(strings.Fields(p), " ")
		k := plano(p)
		if k == "" {
			return reglasDoc{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "palabras", Message: "cada palabra necesita letras o números"})
		}
		if !vistas[k] {
			vistas[k] = true
			palabras = append(palabras, p)
		}
	}
	return reglasDoc{Cuarentena: r.Cuarentena, Rechazo: r.Rechazo, Palabras: palabras, ActualizadoPor: who, ActualizadoEn: s.now().UTC().Truncate(time.Millisecond)}, nil
}
