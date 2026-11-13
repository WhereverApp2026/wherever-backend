package imagenes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Valores por defecto de la spec nv-45.
const (
	DefaultVence       = 5 * time.Minute
	DefaultMaxBytes    = 5 << 20
	DefaultSimultaneas = 1
	DefaultAbandono    = 2 * time.Minute
	vidaPendiente      = 24 * time.Hour
	largoID            = 16 // bytes aleatorios → 32 hex
)

// Moderador es lo que el service usa de internal/moderacion (nv-42).
type Moderador interface {
	Evaluar(ctx context.Context, c moderacion.Contenido) moderacion.Resultado
}

// Config son los límites del service. URLPublica es la del CloudFront de imágenes.
type Config struct {
	URLPublica string
	Vence      time.Duration
	MaxBytes   int64
	// Simultaneas es cuántas decodificaciones corren a la vez por instancia (memoria).
	Simultaneas int
	// Abandono es cuánto espera una imagen `procesando` antes de que otra petición la retome.
	Abandono time.Duration
}

func (c Config) conDefectos() Config {
	if c.Vence <= 0 {
		c.Vence = DefaultVence
	}
	if c.MaxBytes <= 0 {
		c.MaxBytes = DefaultMaxBytes
	}
	if c.Simultaneas <= 0 {
		c.Simultaneas = DefaultSimultaneas
	}
	if c.Abandono <= 0 {
		c.Abandono = DefaultAbandono
	}
	return c
}

// Service sube, confirma y borra imágenes.
type Service struct {
	repo      *Repository
	almacen   Almacen
	moderador Moderador
	cfg       Config
	turnos    chan struct{}
	log       *slog.Logger
	ahora     func() time.Time
}

// NewService recibe todo por constructor; sin almacén, moderador o URL no hay service (las
// rutas responden 503).
func NewService(repo *Repository, almacen Almacen, moderador Moderador, cfg Config, log *slog.Logger) (*Service, error) {
	if repo == nil || almacen == nil || moderador == nil || cfg.URLPublica == "" {
		return nil, errors.New("imagenes: faltan el repositorio, el almacén, el moderador o la URL pública")
	}
	if log == nil {
		log = slog.Default()
	}
	cfg = cfg.conDefectos()
	return &Service{repo: repo, almacen: almacen, moderador: moderador, cfg: cfg,
		turnos: make(chan struct{}, cfg.Simultaneas), log: log, ahora: time.Now}, nil
}

// Subir emite el formulario firmado y deja la imagen pendiente.
func (s *Service) Subir(ctx context.Context, sujeto string, req SubidaRequest) (*SubidaResponse, error) {
	dueno, err := bson.ObjectIDFromHex(sujeto)
	if err != nil {
		return nil, apperr.Unauthorized("")
	}
	id, err := nuevoID()
	if err != nil {
		return nil, err
	}
	subida, err := s.almacen.FirmarSubida(ctx, claveOriginal(id), req.Tipo, s.cfg.MaxBytes, s.cfg.Vence)
	if err != nil {
		s.log.ErrorContext(ctx, "imagenes: no se pudo firmar la subida", slog.String("error", err.Error()))
		return nil, apperr.Unavailable("no se pudo preparar la subida")
	}
	ahora := s.ahora().UTC()
	expira := ahora.Add(vidaPendiente)
	img := &Imagen{ID: id, DuenoID: dueno, Tipo: req.Tipo, Estado: EstadoPendiente, CreadaEn: ahora, ActualizadaEn: ahora, ExpiraEn: &expira}
	if err := s.repo.Insert(ctx, img); err != nil {
		return nil, err
	}
	return &SubidaResponse{ID: id, URL: subida.URL, Campos: subida.Campos, VenceEn: ahora.Add(s.cfg.Vence), MaxBytes: s.cfg.MaxBytes}, nil
}

// Confirmar procesa el original subido: lo valida, lo recodifica, lo modera una sola vez y lo
// publica, lo rechaza o lo deja en revisión.
func (s *Service) Confirmar(ctx context.Context, sujeto, id string) (*ImagenResponse, error) {
	dueno, err := s.dueno(sujeto, id)
	if err != nil {
		return nil, err
	}
	img, err := s.tomar(ctx, id, dueno)
	if err != nil {
		return nil, err
	}
	datos, err := s.almacen.Leer(ctx, claveOriginal(id), s.cfg.MaxBytes)
	if err != nil {
		return s.fallaLectura(ctx, img, err)
	}
	proc, err := s.procesar(ctx, datos, img.Tipo)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		s.soltar(ctx, img)
		return nil, apperr.Unavailable("servidor ocupado; intenta de nuevo")
	}
	if err != nil {
		return s.rechazarArchivo(ctx, img)
	}
	if !img.Moderacion.Concluyente() {
		res := s.moderador.Evaluar(ctx, moderacion.Contenido{Imagen: proc.Grande})
		img.Moderacion = &Moderacion{Veredicto: string(res.Veredicto), Puntajes: res.Puntajes, Falla: res.Falla}
	}
	return s.decidir(ctx, img, proc)
}

// tomar marca la imagen como en proceso; 404 si no es del dueño, 409 si ya se procesó o se
// está procesando.
func (s *Service) tomar(ctx context.Context, id string, dueno bson.ObjectID) (*Imagen, error) {
	ahora := s.ahora().UTC()
	img, err := s.repo.Tomar(ctx, id, dueno, ahora, ahora.Add(-s.cfg.Abandono))
	if !errors.Is(err, ErrNoTomable) {
		return img, err
	}
	if _, err := s.repo.Buscar(ctx, id, dueno); errors.Is(err, ErrNotFound) {
		return nil, apperr.NotFound("imagen no encontrada")
	} else if err != nil {
		return nil, err
	}
	return nil, apperr.Conflict("la imagen ya se procesó o se está procesando")
}

// fallaLectura responde cuando no se pudo leer el original: 409 si no se subió, 422 si es
// demasiado grande y 503 si el almacén falla.
func (s *Service) fallaLectura(ctx context.Context, img *Imagen, err error) (*ImagenResponse, error) {
	switch {
	case errors.Is(err, ErrSinObjeto):
		s.soltar(ctx, img)
		return nil, apperr.Conflict("la imagen no se ha subido o ya venció")
	case errors.Is(err, ErrObjetoGrande):
		return s.rechazarArchivo(ctx, img)
	}
	s.soltar(ctx, img)
	s.log.ErrorContext(ctx, "imagenes: no se pudo leer el original", slog.String("error", err.Error()))
	return nil, apperr.Unavailable("almacén de imágenes no disponible")
}

// procesar decodifica con turno: una decodificación a la vez por instancia (memoria).
func (s *Service) procesar(ctx context.Context, datos []byte, tipo string) (Procesada, error) {
	select {
	case s.turnos <- struct{}{}:
	case <-ctx.Done():
		return Procesada{}, ctx.Err()
	}
	defer func() { <-s.turnos }()
	return Procesar(datos, tipo)
}

func (s *Service) decidir(ctx context.Context, img *Imagen, proc Procesada) (*ImagenResponse, error) {
	switch moderacion.Veredicto(img.Moderacion.Veredicto) {
	case moderacion.VeredictoAprobado:
		if err := s.publicar(ctx, img.ID, proc); err != nil {
			s.soltar(ctx, img) // el veredicto queda guardado: no se vuelve a moderar
			s.log.ErrorContext(ctx, "imagenes: no se pudo publicar", slog.String("error", err.Error()))
			return nil, apperr.Unavailable("almacén de imágenes no disponible")
		}
		s.borrar(ctx, claveOriginal(img.ID))
		img.Estado, img.ExpiraEn = EstadoPublicada, nil
	case moderacion.VeredictoRechazado:
		s.borrar(ctx, claveOriginal(img.ID))
		img.Estado, img.Motivo, img.ExpiraEn = EstadoRechazada, MotivoModeracion, nil
	default:
		// Dudosa o moderación caída: el original sigue en cuarentena/ para la cola de nv-43.
		img.Estado, img.ExpiraEn = EstadoEnRevision, nil
	}
	if err := s.terminar(ctx, img); err != nil {
		return nil, err
	}
	s.log.InfoContext(ctx, "imagen procesada", slog.String("evento", "imagen_procesada"), slog.String("imagen", img.ID),
		slog.String("estado", img.Estado), slog.String("veredicto", img.Moderacion.Veredicto), slog.String("falla", img.Moderacion.Falla))
	return s.respuesta(img), nil
}

func (s *Service) publicar(ctx context.Context, id string, proc Procesada) error {
	if err := s.almacen.Guardar(ctx, clavePublicada(id, LadoGrande), "image/jpeg", proc.Grande); err != nil {
		return err
	}
	return s.almacen.Guardar(ctx, clavePublicada(id, LadoChico), "image/jpeg", proc.Chica)
}

// rechazarArchivo borra un original que no es una foto válida y responde 422.
func (s *Service) rechazarArchivo(ctx context.Context, img *Imagen) (*ImagenResponse, error) {
	s.borrar(ctx, claveOriginal(img.ID))
	img.Estado, img.Motivo, img.ExpiraEn = EstadoRechazada, MotivoArchivo, nil
	if err := s.terminar(ctx, img); err != nil {
		return nil, err
	}
	return nil, apperr.Validation("el archivo no es una foto JPEG, PNG o WebP válida o pasa de " +
		"6000 px por lado")
}

// terminar guarda el resultado; si mientras tanto la borraron, borra lo publicado y da 404.
func (s *Service) terminar(ctx context.Context, img *Imagen) error {
	img.ActualizadaEn = s.ahora().UTC()
	ok, err := s.repo.Terminar(context.WithoutCancel(ctx), img)
	if err != nil {
		return err
	}
	if !ok {
		s.borrar(ctx, claves(img.ID)...)
		return apperr.NotFound("imagen no encontrada")
	}
	return nil
}

// soltar devuelve la imagen a su estado confirmable para reintentar.
func (s *Service) soltar(ctx context.Context, img *Imagen) {
	img.Estado = EstadoPendiente
	if img.Moderacion != nil && !img.Moderacion.Concluyente() {
		img.Estado = EstadoEnRevision
	}
	img.ActualizadaEn = s.ahora().UTC()
	if _, err := s.repo.Terminar(context.WithoutCancel(ctx), img); err != nil {
		s.log.ErrorContext(ctx, "imagenes: no se pudo soltar", slog.String("error", err.Error()))
	}
}

// borrar limpia claves en S3 sin fallar la petición: lo que quede en cuarentena/ vence a 1 día.
func (s *Service) borrar(ctx context.Context, claves ...string) {
	if err := s.almacen.Borrar(context.WithoutCancel(ctx), claves...); err != nil {
		s.log.WarnContext(ctx, "imagenes: no se pudo borrar en S3", slog.String("error", err.Error()))
	}
}

// Eliminar borra la imagen del dueño en S3 y en la base (404 si no existe o es ajena).
func (s *Service) Eliminar(ctx context.Context, sujeto, id string) error {
	dueno, err := s.dueno(sujeto, id)
	if err != nil {
		return err
	}
	if _, err := s.repo.Buscar(ctx, id, dueno); errors.Is(err, ErrNotFound) {
		return apperr.NotFound("imagen no encontrada")
	} else if err != nil {
		return err
	}
	if err := s.almacen.Borrar(ctx, claves(id)...); err != nil {
		s.log.ErrorContext(ctx, "imagenes: no se pudo borrar", slog.String("error", err.Error()))
		return apperr.Unavailable("almacén de imágenes no disponible")
	}
	if _, err := s.repo.Eliminar(ctx, id, dueno); err != nil {
		return err
	}
	return nil
}

// dueno valida el token y el id: un id mal formado es 404, igual que uno ajeno.
func (s *Service) dueno(sujeto, id string) (bson.ObjectID, error) {
	dueno, err := bson.ObjectIDFromHex(sujeto)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("")
	}
	if !idValido(id) {
		return bson.ObjectID{}, apperr.NotFound("imagen no encontrada")
	}
	return dueno, nil
}

func (s *Service) respuesta(img *Imagen) *ImagenResponse {
	return nuevaRespuesta(img, s.cfg.URLPublica)
}

func nuevoID() (string, error) {
	b := make([]byte, largoID)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func idValido(id string) bool {
	if len(id) != 2*largoID {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
