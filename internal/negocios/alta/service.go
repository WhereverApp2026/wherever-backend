package alta

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/expediente"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Límites del alta (spec nv-55).
const (
	MaxAltasAbiertas = 3
	KindRevisado     = notificaciones.KindBusinessReviewed
)

// Cuentas es lo que el alta usa de usuarios: leer la cuenta y agregar el rol.
type Cuentas interface {
	FindByID(ctx context.Context, id bson.ObjectID) (usuarios.User, error)
	Update(ctx context.Context, id bson.ObjectID, set bson.D) (usuarios.User, error)
}

// ErrImagenNoValida: la imagen no existe, no es de la persona o no está publicada.
var ErrImagenNoValida = errors.New("alta: imagen no publicada o ajena")

// Imagenes resuelve una imagen publicada por nv-45 (subida, moderación y publicación).
// Devuelve su URL pública; ErrImagenNoValida si no es de la persona o no está publicada.
type Imagenes interface {
	Publicada(ctx context.Context, owner bson.ObjectID, imagenID string) (string, error)
}

// Fichas invalida la ficha pública en la caché (nv-18).
type Fichas interface {
	InvalidarFicha(ctx context.Context, id string)
}

// Config son las versiones vigentes que exige el consentimiento.
type Config struct {
	// AvisoVersion es AVISO_VERSION: el consentimiento `datos_negocio` debe ser de esta versión.
	AvisoVersion string
}

// Service orquesta el alta, las fotos, el expediente y la revisión.
type Service struct {
	repo     *negocios.Repository
	exp      *expediente.Service
	cuentas  Cuentas
	imagenes Imagenes
	fichas   Fichas
	notifier notificaciones.Notifier
	cursor   *pagination.Signer
	cfg      Config
	log      *slog.Logger
	now      func() time.Time
}

// NewService recibe sus dependencias por constructor.
func NewService(repo *negocios.Repository, exp *expediente.Service, cuentas Cuentas, cursor *pagination.Signer, cfg Config, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, exp: exp, cuentas: cuentas, cursor: cursor, cfg: cfg, log: log, now: time.Now, notifier: notificaciones.Noop{}}
}

// WithImagenes conecta la subida de nv-45; sin ella las rutas de fotos responden 503.
func (s *Service) WithImagenes(i Imagenes) *Service {
	s.imagenes = i
	return s
}

// WithFichas conecta la invalidación de la ficha pública.
func (s *Service) WithFichas(f Fichas) *Service {
	s.fichas = f
	return s
}

// WithNotifier conecta el aviso al dueño (nv-40/41).
func (s *Service) WithNotifier(n notificaciones.Notifier) *Service {
	if n != nil {
		s.notifier = n
	}
	return s
}

func errStore() error        { return apperr.Unavailable("servicio no disponible") }
func errNoEncontrado() error { return apperr.NotFound("negocio no encontrado") }
func errVersion() error      { return apperr.Conflict("el negocio cambió; vuelve a cargarlo") }

func sujeto(claims *middleware.Claims) (bson.ObjectID, error) {
	if claims == nil {
		return bson.ObjectID{}, apperr.Unauthorized("autenticación requerida")
	}
	id, err := bson.ObjectIDFromHex(claims.Subject)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("sesión inválida")
	}
	return id, nil
}

// propio trae el negocio si es de la persona y no está dado de baja. Uno ajeno da 404 para no
// confirmar que existe.
func (s *Service) propio(ctx context.Context, claims *middleware.Claims, raw string) (bson.ObjectID, negocios.Business, error) {
	owner, err := sujeto(claims)
	if err != nil {
		return bson.ObjectID{}, negocios.Business{}, err
	}
	b, err := s.buscar(ctx, raw)
	if err != nil {
		return bson.ObjectID{}, negocios.Business{}, err
	}
	if b.OwnerID != owner {
		return bson.ObjectID{}, negocios.Business{}, errNoEncontrado()
	}
	return owner, b, nil
}

func (s *Service) buscar(ctx context.Context, raw string) (negocios.Business, error) {
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return negocios.Business{}, errNoEncontrado()
	}
	b, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, negocios.ErrNotFound) || (err == nil && b.DeletedAt != nil) {
		return negocios.Business{}, errNoEncontrado()
	}
	if err != nil {
		return negocios.Business{}, errStore()
	}
	return b, nil
}

// editables son los estados en que el alta se puede cambiar.
var editables = []string{negocios.StatusDraft, negocios.StatusRejected}

func editable(b negocios.Business) error {
	if !slices.Contains(editables, b.State) {
		return apperr.Conflict("alta_cerrada")
	}
	return nil
}

// consentimiento exige el consentimiento `datos_negocio` vigente (nv-19) antes de recibir
// documentos, datos fiscales o CLABE.
func (s *Service) consentimiento(ctx context.Context, owner bson.ObjectID) error {
	ok, err := s.tieneConsentimiento(ctx, owner)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.Validation("consentimiento_requerido", apperr.FieldError{Field: "consentimiento", Message: "consentimiento_requerido"})
	}
	return nil
}

func (s *Service) tieneConsentimiento(ctx context.Context, owner bson.ObjectID) (bool, error) {
	u, err := s.cuentas.FindByID(ctx, owner)
	if errors.Is(err, usuarios.ErrNotFound) {
		return false, apperr.Unauthorized("sesión inválida")
	}
	if err != nil {
		return false, errStore()
	}
	p := u.Privacy
	return p.BusinessData && (s.cfg.AvisoVersion == "" || p.NoticeVersion == s.cfg.AvisoVersion), nil
}

var noSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugDe(nombre string, id bson.ObjectID) string {
	base := strings.Trim(noSlug.ReplaceAllString(text.Normalize(nombre), "-"), "-")
	if len(base) > 60 {
		base = strings.Trim(base[:60], "-")
	}
	if base == "" {
		base = "negocio"
	}
	return base + "-" + id.Hex()[16:]
}

// Crear es el paso 1: negocio en borrador y rol `comerciante` agregado sin quitar los demás.
func (s *Service) Crear(ctx context.Context, claims *middleware.Claims, req CrearRequest) (NegocioResponse, error) {
	owner, err := sujeto(claims)
	if err != nil {
		return NegocioResponse{}, err
	}
	if !negocios.ValidCategory(req.Categoria) {
		return NegocioResponse{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "categoria", Message: "debe ser uno de los 6 giros"})
	}
	user, err := s.puedeCrear(ctx, owner)
	if err != nil {
		return NegocioResponse{}, err
	}
	qr, err := negocios.NewQR()
	if err != nil {
		return NegocioResponse{}, errStore()
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	b := negocios.Business{ID: bson.NewObjectID(), OwnerID: owner, Name: strings.TrimSpace(req.Nombre), Category: req.Categoria,
		Description: strings.TrimSpace(req.Descripcion), QRCheckin: qr, CreatedAt: now}
	b.Slug = slugDe(b.Name, b.ID)
	b.NormalizedName = text.Normalize(b.Name)
	if err := s.repo.InsertDraft(ctx, &b); err != nil {
		return NegocioResponse{}, errStore()
	}
	b.State, b.Step, b.Version, b.Plan = negocios.StatusDraft, 1, 1, negocios.PlanFree
	out := negocioResponse(b)
	out.RequiereRefresh, err = s.agregarRol(ctx, user)
	return out, err
}

// puedeCrear trae la cuenta y revisa el tope de altas abiertas.
func (s *Service) puedeCrear(ctx context.Context, owner bson.ObjectID) (usuarios.User, error) {
	user, err := s.cuentas.FindByID(ctx, owner)
	if errors.Is(err, usuarios.ErrNotFound) {
		return usuarios.User{}, apperr.Unauthorized("sesión inválida")
	}
	if err != nil {
		return usuarios.User{}, errStore()
	}
	abiertas, err := s.repo.CountOpenByOwner(ctx, owner)
	if err != nil {
		return usuarios.User{}, errStore()
	}
	if abiertas >= MaxAltasAbiertas {
		return usuarios.User{}, apperr.Conflict("limite_altas")
	}
	return user, nil
}

// agregarRol suma `comerciante` sin quitar los demás. El access vigente conserva sus roles
// hasta el siguiente refresh: devuelve true para que la app refresque.
func (s *Service) agregarRol(ctx context.Context, user usuarios.User) (bool, error) {
	if slices.Contains(user.Roles, middleware.RoleMerchant) {
		return false, nil
	}
	roles := append(slices.Clone(user.Roles), middleware.RoleMerchant)
	if _, err := s.cuentas.Update(ctx, user.ID, bson.D{{Key: "roles", Value: roles}}); err != nil {
		return false, errStore()
	}
	return true, nil
}

// Mios lista los negocios de la persona.
func (s *Service) Mios(ctx context.Context, claims *middleware.Claims) ([]NegocioResponse, error) {
	owner, err := sujeto(claims)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListByOwner(ctx, owner)
	if err != nil {
		return nil, errStore()
	}
	out := make([]NegocioResponse, 0, len(rows))
	for _, b := range rows {
		out = append(out, negocioResponse(b))
	}
	return out, nil
}

// Paso es un paso del alta del 2 al 6.
type Paso struct {
	Numero int
	Dos    *Paso2Request
	Tres   *Paso3Request
	Cinco  *Paso5Request
}

// GuardarPaso valida y guarda un paso con If-Match. Solo en borrador o rechazado.
func (s *Service) GuardarPaso(ctx context.Context, claims *middleware.Claims, id string, version int, p Paso) (NegocioResponse, error) {
	_, b, err := s.propio(ctx, claims, id)
	if err != nil {
		return NegocioResponse{}, err
	}
	if err := editable(b); err != nil {
		return NegocioResponse{}, err
	}
	if b.Version != version {
		return NegocioResponse{}, errVersion()
	}
	set, fields, err := s.setDePaso(ctx, b, p)
	if err != nil {
		return NegocioResponse{}, err
	}
	if len(fields) > 0 {
		return NegocioResponse{}, apperr.Validation("datos inválidos", fields...)
	}
	set = append(set, bson.E{Key: "paso_alta", Value: max(b.Step, p.Numero)})
	nb, err := s.repo.ApplyIf(ctx, bson.D{
		{Key: "_id", Value: b.ID}, {Key: "propietario_id", Value: b.OwnerID}, {Key: "version", Value: version},
		{Key: "estado", Value: bson.D{{Key: "$in", Value: editables}}},
	}, set)
	if errors.Is(err, negocios.ErrNoAplica) {
		return NegocioResponse{}, errVersion()
	}
	if err != nil {
		return NegocioResponse{}, errStore()
	}
	return negocioResponse(nb), nil
}

// datos arma el `$set` de los pasos que traen cuerpo (2, 3 y 5).
func (p Paso) datos() (bson.D, []apperr.FieldError, bool) {
	switch {
	case p.Numero == 2 && p.Dos != nil:
		loc := negocios.Location{Type: p.Dos.Ubicacion.Type, Coordinates: p.Dos.Ubicacion.Coordinates}
		fields := append(negocios.ValidateLocation(loc), negocios.ValidateAddress(p.Dos.Direccion)...)
		return bson.D{{Key: "ubicacion", Value: loc}, {Key: "direccion", Value: p.Dos.Direccion}}, fields, true
	case p.Numero == 3 && p.Tres != nil:
		return bson.D{{Key: "horarios", Value: p.Tres.Horarios}, {Key: "horario_min", Value: negocios.DeriveScheduleMin(p.Tres.Horarios)}},
			negocios.ValidateSchedules(p.Tres.Horarios), true
	case p.Numero == 5 && p.Cinco != nil:
		c := p.Cinco
		fields := append(negocios.ValidatePricing(c.NivelPrecio, c.PrecioPromedio), negocios.ValidateOffer(c.Servicios, c.OpcionesDieta, c.Alergenos)...)
		return bson.D{
			{Key: "nivel_precio", Value: c.NivelPrecio}, {Key: "precio_promedio", Value: c.PrecioPromedio},
			{Key: "servicios", Value: nonNil(c.Servicios)}, {Key: "opciones_dieta", Value: nonNil(c.OpcionesDieta)},
			{Key: "alergenos", Value: nonNil(c.Alergenos)}, {Key: "accesibilidad", Value: c.Accesibilidad},
		}, fields, true
	}
	return nil, nil, false
}

func (s *Service) setDePaso(ctx context.Context, b negocios.Business, p Paso) (bson.D, []apperr.FieldError, error) {
	if set, fields, ok := p.datos(); ok {
		return set, fields, nil
	}
	switch p.Numero {
	case 4:
		if len(b.Photos) < negocios.MinPhotos {
			return nil, []apperr.FieldError{{Field: "fotos", Message: "sube al menos 3 fotos"}}, nil
		}
		return bson.D{}, nil, nil
	case 6:
		vigentes, err := s.exp.Vigentes(ctx, b.ID)
		if err != nil {
			return nil, nil, err
		}
		return bson.D{}, faltanDocumentos(vigentes), nil
	default:
		return nil, []apperr.FieldError{{Field: "paso", Message: "debe ser del 2 al 6"}}, nil
	}
}

func faltanDocumentos(vigentes map[string]bool) []apperr.FieldError {
	var out []apperr.FieldError
	for _, tipo := range expediente.TiposArchivo {
		if !vigentes[tipo] {
			out = append(out, apperr.FieldError{Field: tipo, Message: "falta"})
		}
	}
	return out
}

// Faltantes es lo que le falta al negocio para enviarse a revisión: pasos 1 a 5, 3 fotos,
// INE (frente y reverso), comprobante y consentimiento. Fiscales y CLABE no frenan (A5).
func Faltantes(b negocios.Business, vigentes map[string]bool, consentimiento bool) []apperr.FieldError {
	var out []apperr.FieldError
	add := func(campo string) { out = append(out, apperr.FieldError{Field: campo, Message: "falta"}) }
	if b.Location.Type != "Point" {
		add("ubicacion")
	}
	if len(b.ScheduleMin) == 0 {
		add("horarios")
	}
	if len(b.Photos) < negocios.MinPhotos {
		add("fotos")
	}
	if b.PriceLevel < 1 {
		add("precios")
	}
	out = append(out, faltanDocumentos(vigentes)...)
	if !consentimiento {
		add("consentimiento")
	}
	return out
}

// EnviarRevision pasa el negocio a `en_revision` si tiene lo mínimo.
func (s *Service) EnviarRevision(ctx context.Context, claims *middleware.Claims, id string) (NegocioResponse, error) {
	owner, b, err := s.propio(ctx, claims, id)
	if err != nil {
		return NegocioResponse{}, err
	}
	if err := editable(b); err != nil {
		return NegocioResponse{}, err
	}
	vigentes, err := s.exp.Vigentes(ctx, b.ID)
	if err != nil {
		return NegocioResponse{}, err
	}
	consent, err := s.tieneConsentimiento(ctx, owner)
	if err != nil {
		return NegocioResponse{}, err
	}
	if fields := Faltantes(b, vigentes, consent); len(fields) > 0 {
		return NegocioResponse{}, apperr.Validation("faltan datos para enviar a revisión", fields...)
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	nb, err := s.repo.ApplyIf(ctx, bson.D{
		{Key: "_id", Value: b.ID}, {Key: "propietario_id", Value: owner}, {Key: "version", Value: b.Version},
		{Key: "estado", Value: bson.D{{Key: "$in", Value: editables}}},
	}, bson.D{{Key: "estado", Value: negocios.StatusReview}, {Key: "revision", Value: negocios.Revision{EnviadoEn: &now}}})
	if errors.Is(err, negocios.ErrNoAplica) {
		return NegocioResponse{}, errVersion()
	}
	if err != nil {
		return NegocioResponse{}, errStore()
	}
	return negocioResponse(nb), nil
}

// Cola lista los negocios en revisión (o, con `estado=cambios_pendientes`, los aprobados con
// nombre o dirección por revisar, nv-56), del envío más viejo al más nuevo.
func (s *Service) Cola(ctx context.Context, q ColaQuery) (pagination.PageResult[ColaItem], error) {
	estado := q.Estado
	if estado == "" {
		estado = negocios.StatusReview
	}
	scope := s.cursor.Scope("negocios_revision", estado)
	var after *negocios.QueuePosition
	c, ok, err := validation.Position(s.cursor, scope, q.Cursor)
	if err != nil {
		return pagination.PageResult[ColaItem]{}, err
	}
	if ok {
		bad := apperr.Validation("cursor inválido", apperr.FieldError{Field: "cursor", Message: "cursor inválido"})
		at, errT := time.Parse(time.RFC3339Nano, c.Sort)
		oid, errID := bson.ObjectIDFromHex(c.ID)
		if errT != nil || errID != nil {
			return pagination.PageResult[ColaItem]{}, bad
		}
		after = &negocios.QueuePosition{SentAt: at, ID: oid}
	}
	rows, err := s.repo.Queue(ctx, estado, after, q.Limit)
	if err != nil {
		return pagination.PageResult[ColaItem]{}, errStore()
	}
	next := ""
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		last := rows[len(rows)-1]
		next, err = s.cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: enviadoEn(last, estado).UTC().Format(time.RFC3339Nano), Scope: scope})
		if err != nil {
			return pagination.PageResult[ColaItem]{}, errStore()
		}
	}
	now := s.now().UTC()
	items := make([]ColaItem, 0, len(rows))
	for _, b := range rows {
		items = append(items, colaItem(b, now))
	}
	return pagination.NewPageResult(items, next, q.Limit), nil
}

// enviadoEn es la posición del negocio en su cola: el envío a revisión o el de los cambios.
func enviadoEn(b negocios.Business, estado string) time.Time {
	if estado == negocios.EstadoCambiosPendientes && b.CambiosPendientes != nil {
		return b.CambiosPendientes.EnviadoEn
	}
	if b.Revision.EnviadoEn != nil {
		return *b.Revision.EnviadoEn
	}
	return time.Time{}
}

// Decidir aprueba o rechaza un negocio en revisión (If-Match). Solo un administrador lleva un
// negocio a `aprobado`. Notifica al dueño y programa el borrado de la INE a 90 días.
func (s *Service) Decidir(ctx context.Context, claims *middleware.Claims, id string, version int, req DecisionRequest) (NegocioResponse, error) {
	admin, err := sujeto(claims)
	if err != nil {
		return NegocioResponse{}, err
	}
	motivo := strings.TrimSpace(req.Motivo)
	if n := len([]rune(motivo)); req.Decision == DecisionRechazar && (n < 10 || n > 500) {
		return NegocioResponse{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "motivo", Message: "al rechazar, el motivo es obligatorio (10 a 500 caracteres)"})
	}
	b, err := s.buscar(ctx, id)
	if err != nil {
		return NegocioResponse{}, err
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	if b.State != negocios.StatusReview && b.CambiosPendientes != nil {
		return s.decidirCambios(ctx, b, version, req.Decision, motivo)
	}
	set, resultado := cambiosDeDecision(req.Decision, motivo, negocios.Revision{EnviadoEn: b.Revision.EnviadoEn, RevisadoPor: &admin, RevisadoEn: &now})
	nb, err := s.repo.ApplyIf(ctx, bson.D{{Key: "_id", Value: b.ID}, {Key: "estado", Value: negocios.StatusReview}, {Key: "version", Value: version}}, set)
	if errors.Is(err, negocios.ErrNoAplica) {
		return NegocioResponse{}, apperr.Conflict("el negocio ya se decidió o cambió")
	}
	if err != nil {
		return NegocioResponse{}, errStore()
	}
	s.despuesDeDecidir(ctx, nb, resultado, now)
	return negocioResponse(nb), nil
}

// cambiosDeDecision arma el `$set` de aprobar o rechazar y el estado resultante.
func cambiosDeDecision(decision, motivo string, rev negocios.Revision) (bson.D, string) {
	if decision == DecisionRechazar {
		rev.MotivoRechazo = motivo
		return bson.D{{Key: "estado", Value: negocios.StatusRejected}, {Key: "revision", Value: rev}}, negocios.StatusRejected
	}
	return bson.D{{Key: "estado", Value: negocios.StatusApproved}, {Key: "verificado", Value: true},
		{Key: "paso_alta", Value: negocios.StepComplete}, {Key: "revision", Value: rev}}, negocios.StatusApproved
}

// despuesDeDecidir programa el borrado de la INE a 90 días, invalida la ficha si se aprobó y
// avisa al dueño. Nada de esto cambia la respuesta: la decisión ya quedó.
func (s *Service) despuesDeDecidir(ctx context.Context, b negocios.Business, resultado string, revisado time.Time) {
	if err := s.exp.ProgramarBorrado(ctx, b.ID, revisado); err != nil {
		s.log.ErrorContext(ctx, "alta: borrado de la INE sin programar", "evento", "retencion_sin_programar")
	}
	if resultado == negocios.StatusApproved && s.fichas != nil {
		s.fichas.InvalidarFicha(ctx, b.ID.Hex())
	}
	s.notifier.Emit(ctx, b.OwnerID, KindRevisado, map[string]string{"negocio_id": b.ID.Hex(), "resultado": resultado})
}

// decidirCambios aplica o descarta nombre, dirección o ubicación de un negocio aprobado
// (nv-56). El negocio nunca se despublica; descartar exige motivo y avisa al dueño.
func (s *Service) decidirCambios(ctx context.Context, b negocios.Business, version int, decision, motivo string) (NegocioResponse, error) {
	c := b.CambiosPendientes
	set := bson.D{}
	resultado := "cambios_descartados"
	if decision == DecisionAprobar {
		resultado = "cambios_aplicados"
		if c.Nombre != "" {
			set = append(set, bson.E{Key: "nombre", Value: c.Nombre}, bson.E{Key: "nombre_normalizado", Value: text.Normalize(c.Nombre)})
		}
		if c.Direccion != nil {
			set = append(set, bson.E{Key: "direccion", Value: *c.Direccion})
		}
		if c.Ubicacion != nil {
			set = append(set, bson.E{Key: "ubicacion", Value: *c.Ubicacion})
		}
	}
	nb, err := s.repo.ChangeIf(ctx, bson.D{{Key: "_id", Value: b.ID}, {Key: "version", Value: version},
		{Key: "cambios_pendientes", Value: bson.D{{Key: "$exists", Value: true}}}}, set, "cambios_pendientes")
	if errors.Is(err, negocios.ErrNoAplica) {
		return NegocioResponse{}, apperr.Conflict("el negocio ya se decidió o cambió")
	}
	if err != nil {
		return NegocioResponse{}, errStore()
	}
	if s.fichas != nil {
		s.fichas.InvalidarFicha(ctx, b.ID.Hex())
	}
	data := map[string]string{"negocio_id": b.ID.Hex(), "resultado": resultado}
	if decision == DecisionRechazar {
		data["motivo"] = motivo
	}
	s.notifier.Emit(ctx, b.OwnerID, KindRevisado, data)
	return negocioResponse(nb), nil
}
