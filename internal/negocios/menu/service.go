package menu

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/busquedas"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/alta"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Negocios es lo que el menú lee de `negocios`: el dueño y el estado.
type Negocios interface {
	FindByID(ctx context.Context, id bson.ObjectID) (negocios.Business, error)
}

// Service administra el menú; la propiedad del negocio se valida aquí.
type Service struct {
	repo     *Repository
	negocios Negocios
	imagenes alta.Imagenes
	log      *slog.Logger
	now      func() time.Time
}

// NewService recibe sus dependencias por constructor.
func NewService(repo *Repository, n Negocios, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, negocios: n, log: log, now: time.Now}
}

// WithImagenes conecta las imágenes publicadas de nv-45; sin ellas, una foto responde 503.
func (s *Service) WithImagenes(i alta.Imagenes) *Service {
	s.imagenes = i
	return s
}

func errStore() error        { return apperr.Unavailable("servicio no disponible") }
func errNoEncontrado() error { return apperr.NotFound("negocio no encontrado") }
func errPlatillo() error     { return apperr.NotFound("platillo no encontrado") }
func errVersion() error      { return apperr.Conflict("el platillo cambió; vuelve a cargarlo") }

// propio trae el negocio de la persona sin baja; uno ajeno da 404.
func (s *Service) propio(ctx context.Context, claims *middleware.Claims, raw string) (negocios.Business, error) {
	if claims == nil {
		return negocios.Business{}, apperr.Unauthorized("autenticación requerida")
	}
	owner, err := bson.ObjectIDFromHex(claims.Subject)
	if err != nil {
		return negocios.Business{}, apperr.Unauthorized("sesión inválida")
	}
	b, err := s.buscar(ctx, raw)
	if err != nil {
		return negocios.Business{}, err
	}
	if b.OwnerID != owner {
		return negocios.Business{}, errNoEncontrado()
	}
	return b, nil
}

func (s *Service) buscar(ctx context.Context, raw string) (negocios.Business, error) {
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return negocios.Business{}, errNoEncontrado()
	}
	b, err := s.negocios.FindByID(ctx, id)
	if errors.Is(err, negocios.ErrNotFound) || (err == nil && b.DeletedAt != nil) {
		return negocios.Business{}, errNoEncontrado()
	}
	if err != nil {
		return negocios.Business{}, errStore()
	}
	return b, nil
}

// fotoDe resuelve la imagen publicada del dueño; vacío = sin foto.
func (s *Service) fotoDe(ctx context.Context, owner bson.ObjectID, imagenID string) (string, error) {
	if imagenID == "" {
		return "", nil
	}
	if s.imagenes == nil {
		return "", apperr.Unavailable("imágenes no disponibles")
	}
	url, err := s.imagenes.Publicada(ctx, owner, imagenID)
	if errors.Is(err, alta.ErrImagenNoValida) {
		return "", apperr.Validation("datos inválidos", apperr.FieldError{Field: "imagen_id", Message: "imagen no publicada o ajena"})
	}
	if err != nil {
		return "", errStore()
	}
	return url, nil
}

// Crear agrega un platillo; 409 `limite_menu` con 200.
func (s *Service) Crear(ctx context.Context, claims *middleware.Claims, negocioID string, req CrearRequest) (ItemResponse, error) {
	b, err := s.propio(ctx, claims, negocioID)
	if err != nil {
		return ItemResponse{}, err
	}
	n, err := s.repo.Count(ctx, b.ID)
	if err != nil {
		return ItemResponse{}, errStore()
	}
	if n >= MaxItems {
		return ItemResponse{}, apperr.Conflict("limite_menu")
	}
	url, err := s.fotoDe(ctx, b.OwnerID, req.ImagenID)
	if err != nil {
		return ItemResponse{}, err
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	it := Item{ID: bson.NewObjectID(), NegocioID: b.ID, Seccion: strings.TrimSpace(req.Seccion), Nombre: strings.TrimSpace(req.Nombre),
		Descripcion: strings.TrimSpace(req.Descripcion), Precio: req.Precio, Etiquetas: etiquetas(req.Etiquetas), ImagenID: req.ImagenID,
		FotoURL: url, Disponible: req.Disponible == nil || *req.Disponible, Orden: req.Orden, Version: 1, CreadoEn: now, ActualizadoEn: now}
	if err := s.repo.Insert(ctx, &it); err != nil {
		return ItemResponse{}, errStore()
	}
	return itemResponse(it), nil
}

// Editar cambia los campos que vienen (If-Match).
func (s *Service) Editar(ctx context.Context, claims *middleware.Claims, negocioID, itemID string, version int, req EditarRequest) (ItemResponse, error) {
	b, err := s.propio(ctx, claims, negocioID)
	if err != nil {
		return ItemResponse{}, err
	}
	id, err := s.existe(ctx, b.ID, itemID)
	if err != nil {
		return ItemResponse{}, err
	}
	set, err := s.cambios(ctx, b.OwnerID, req)
	if err != nil {
		return ItemResponse{}, err
	}
	it, err := s.repo.UpdateIf(ctx, b.ID, id, version, set)
	if errors.Is(err, ErrVersion) {
		return ItemResponse{}, errVersion()
	}
	if err != nil {
		return ItemResponse{}, errStore()
	}
	return itemResponse(it), nil
}

func (s *Service) cambios(ctx context.Context, owner bson.ObjectID, req EditarRequest) (bson.D, error) {
	set := bson.D{}
	add := func(k string, v any) { set = append(set, bson.E{Key: k, Value: v}) }
	if req.Seccion != nil {
		add("seccion", strings.TrimSpace(*req.Seccion))
	}
	if req.Nombre != nil {
		add("nombre", strings.TrimSpace(*req.Nombre))
	}
	if req.Descripcion != nil {
		add("descripcion", strings.TrimSpace(*req.Descripcion))
	}
	if req.Precio != nil {
		add("precio", *req.Precio)
	}
	if req.Etiquetas != nil {
		add("etiquetas", etiquetas(*req.Etiquetas))
	}
	if req.Disponible != nil {
		add("disponible", *req.Disponible)
	}
	if req.Orden != nil {
		add("orden", *req.Orden)
	}
	if req.ImagenID != nil {
		url, err := s.fotoDe(ctx, owner, *req.ImagenID)
		if err != nil {
			return nil, err
		}
		add("imagen_id", *req.ImagenID)
		add("foto_url", url)
	}
	return set, nil
}

func (s *Service) existe(ctx context.Context, negocio bson.ObjectID, raw string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return bson.ObjectID{}, errPlatillo()
	}
	if _, err := s.repo.Find(ctx, negocio, id); errors.Is(err, ErrNotFound) {
		return bson.ObjectID{}, errPlatillo()
	} else if err != nil {
		return bson.ObjectID{}, errStore()
	}
	return id, nil
}

// Borrar quita un platillo (If-Match). La imagen sigue siendo de la persona en nv-45.
func (s *Service) Borrar(ctx context.Context, claims *middleware.Claims, negocioID, itemID string, version int) error {
	b, err := s.propio(ctx, claims, negocioID)
	if err != nil {
		return err
	}
	id, err := s.existe(ctx, b.ID, itemID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteIf(ctx, b.ID, id, version); errors.Is(err, ErrVersion) {
		return errVersion()
	} else if err != nil {
		return errStore()
	}
	return nil
}

// Publico es el menú de un negocio aprobado, agrupado por sección; oculta lo no disponible.
func (s *Service) Publico(ctx context.Context, negocioID string, q MenuQuery) (MenuResponse, error) {
	b, err := s.buscar(ctx, negocioID)
	if err != nil {
		return MenuResponse{}, err
	}
	if b.State != negocios.StatusApproved {
		return MenuResponse{}, errNoEncontrado()
	}
	consulta := ""
	if q.Q != "" {
		consulta = strings.Join(busquedas.Palabras(q.Q), " ")
		if consulta == "" {
			return MenuResponse{Secciones: []Seccion{}}, nil
		}
	}
	items, err := s.repo.Publicos(ctx, b.ID, strings.TrimSpace(q.Seccion), consulta)
	if err != nil {
		return MenuResponse{}, errStore()
	}
	return agrupar(items), nil
}
