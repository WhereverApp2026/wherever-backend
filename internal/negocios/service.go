package negocios

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/cache"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Service orquesta la importación idempotente y el catálogo de categorías.
type Service struct {
	repo  *Repository
	users *usuarios.Repository
	log   *slog.Logger
	newQR func() (string, error)
	now   func() time.Time
	emb   Embeddings
	// cursor firma la paginación de GET /lugares (nv-17).
	cursor *pagination.Signer
	// cache guarda categorías y fichas en Redis (nv-18); nil = sin caché.
	cache *cache.Cache
}

// Vida en la caché (nv-18). Las fichas se invalidan al importar; las categorías también.
const (
	VidaCategorias = time.Hour
	VidaFicha      = 5 * time.Minute
)

// Llaves de la caché: `cache:categorias:todas` y `cache:negocio:{id}`.
var claveCategorias = cache.Clave("categorias", "todas")

func claveFicha(id string) string { return cache.Clave("negocio", id) }

// WithCache activa la caché de categorías y fichas; sin llamarla se consulta MongoDB siempre.
func (s *Service) WithCache(c *cache.Cache) *Service {
	s.cache = c
	return s
}

// Embeddings rehace los vectores de significado desactualizados (nv-50). Si falla, la
// importación sale igual: sin vectores responde el motor de reglas.
type Embeddings interface {
	Refrescar(ctx context.Context) error
}

// plazoEmbeddings acota cuánto espera la importación a los vectores.
const plazoEmbeddings = 30 * time.Second

// WithEmbeddings activa el recálculo de vectores al importar; sin llamarla no se calculan.
func (s *Service) WithEmbeddings(e Embeddings) *Service {
	s.emb = e
	return s
}

// NewService recibe el repositorio de negocios, el de usuarios y el logger por constructor.
func NewService(repo *Repository, users *usuarios.Repository, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, users: users, log: log, newQR: randomQR, now: time.Now}
}

// Import carga la semilla y devuelve el reporte por fila. Las filas inválidas no detienen
// el archivo; un fallo de MongoDB sí (reintentar es seguro por el slug).
func (s *Service) Import(ctx context.Context, adminSubject string, r io.Reader, format string) (ImportReport, error) {
	parsed, parseErrors, err := Parse(r, format)
	if err != nil {
		return ImportReport{}, err
	}
	valid, validationErrors := validateRows(parsed)
	report := ImportReport{Errors: append(append([]RowError{}, parseErrors...), validationErrors...)}

	admin, err := s.activeAdmin(ctx, adminSubject)
	if err != nil {
		return ImportReport{}, err
	}
	tocadas := []string{claveCategorias}
	defer func() { s.cache.Invalidar(context.WithoutCancel(ctx), tocadas...) }()
	for _, item := range valid {
		ownerID, rowErr, err := s.owner(ctx, item, admin.ID)
		if err != nil {
			return report, err
		}
		if rowErr != nil {
			report.Errors = append(report.Errors, *rowErr)
			continue
		}
		qr, err := s.newQR()
		if err != nil {
			return report, fmt.Errorf("generar qr_checkin: %w", err)
		}
		business := newBusiness(item.row, ownerID, admin.ID, qr, time.Now().UTC())
		created, err := s.repo.Upsert(ctx, &business)
		if err != nil {
			return report, apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
		}
		tocadas = append(tocadas, claveFicha(business.ID.Hex()))
		if created {
			report.Created++
		} else {
			report.Updated++
		}
	}
	// Solo conteos: ni correos de dueños ni slugs.
	s.log.Info("semilla de negocios importada", "creados", report.Created, "actualizados", report.Updated, "errores", len(report.Errors))
	s.refreshEmbeddings(ctx, report)
	return report, nil
}

// activeAdmin comprueba que el `sub` del token sea un ObjectId de una cuenta activa: es el
// revisor y, si falta `propietario_correo`, el dueño provisional.
func (s *Service) activeAdmin(ctx context.Context, subject string) (usuarios.User, error) {
	adminID, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return usuarios.User{}, apperr.Unauthorized("sesión inválida")
	}
	admin, err := s.users.FindByID(ctx, adminID)
	if errors.Is(err, usuarios.ErrNotFound) {
		return usuarios.User{}, apperr.Unauthorized("sesión inválida")
	}
	if err != nil {
		return usuarios.User{}, apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
	}
	if admin.Status != usuarios.StatusActive {
		return usuarios.User{}, apperr.Forbidden("cuenta no activa")
	}
	return admin, nil
}

// owner resuelve el dueño de la fila. Un correo desconocido o inactivo es error de la fila;
// un fallo de MongoDB detiene la importación.
func (s *Service) owner(ctx context.Context, item parsedRow, adminID bson.ObjectID) (bson.ObjectID, *RowError, error) {
	if item.row.OwnerEmail == "" {
		return adminID, nil, nil
	}
	owner, err := s.users.FindByEmail(ctx, item.row.OwnerEmail)
	if errors.Is(err, usuarios.ErrNotFound) || (err == nil && owner.Status != usuarios.StatusActive) {
		return bson.ObjectID{}, &RowError{Row: item.number, Field: "propietario_correo", Message: "el propietario no existe o no está activo"}, nil
	}
	if err != nil {
		return bson.ObjectID{}, nil, apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
	}
	return owner.ID, nil, nil
}

// newBusiness arma el documento aprobado de una fila válida.
func newBusiness(row ImportRow, ownerID, reviewerID bson.ObjectID, qr string, now time.Time) Business {
	return Business{
		OwnerID:             ownerID,
		Slug:                row.Slug,
		Name:                row.Name,
		NormalizedName:      text.Normalize(row.Name),
		Category:            row.Category,
		Tags:                row.Tags,
		Description:         row.Description,
		PriceLevel:          row.PriceLevel,
		AveragePrice:        row.AveragePrice,
		Location:            row.Location,
		Address:             row.Address,
		Phone:               row.Phone,
		Schedules:           row.Schedules,
		ScheduleMin:         deriveScheduleMin(row.Schedules),
		Photos:              row.Photos,
		Services:            row.Services,
		Accessibility:       row.Accessibility,
		DietOptions:         row.DietOptions,
		Allergens:           row.Allergens,
		AcceptsReservations: row.AcceptsReservations,
		Rating:              row.Rating,
		State:               StatusApproved,
		QRCheckin:           qr,
		Step:                StepComplete,
		Revision:            Revision{EnviadoEn: &now, RevisadoPor: &reviewerID, RevisadoEn: &now},
	}
}

// Facets devuelve el catálogo público de categorías, etiquetas, servicios y filtros.
func (s *Service) Facets(ctx context.Context) (FacetsResponse, error) {
	return cache.Obtener(ctx, s.cache, claveCategorias, VidaCategorias, func(ctx context.Context) (FacetsResponse, error) {
		counts, err := s.repo.Facets(ctx)
		if err != nil {
			return FacetsResponse{}, apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
		}
		return facetsResponse(counts), nil
	})
}

// facetsResponse completa los 6 giros con cero y mapea los conteos crudos.
func facetsResponse(counts FacetCounts) FacetsResponse {
	categoryCounts := make(map[string]int, len(counts.Categories))
	for _, count := range counts.Categories {
		categoryCounts[count.ID] = count.Count
	}
	categories := make([]ItemCount, 0, len(categoryCatalog))
	for _, item := range categoryCatalog {
		categories = append(categories, ItemCount{ID: item.ID, Name: item.Name, Count: categoryCounts[item.ID]})
	}
	tags := make([]ItemCount, 0, len(counts.Tags))
	for _, count := range counts.Tags {
		tags = append(tags, ItemCount{ID: count.ID, Name: count.ID, Count: count.Count})
	}
	services := make([]ItemCount, 0, len(counts.Services))
	for _, count := range counts.Services {
		services = append(services, ItemCount{ID: count.ID, Name: count.ID, Count: count.Count})
	}
	return FacetsResponse{
		Categories: categories,
		Tags:       tags,
		Services:   services,
		Filters: Filters{
			PriceLevels:   PriceLevelCatalog(),
			DietOptions:   DietCatalog(),
			Accessibility: AccessibilityCatalog(),
		},
	}
}

// randomQR genera un identificador opaco de 128 bits para el check-in.
func randomQR() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// refreshEmbeddings rehace los vectores tras una importación con cambios. Usa su propio plazo
// y no la cancelación de la petición: el reporte ya está listo y el error solo se registra.
func (s *Service) refreshEmbeddings(ctx context.Context, report ImportReport) {
	if s.emb == nil || report.Created+report.Updated == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), plazoEmbeddings)
	defer cancel()
	if err := s.emb.Refrescar(ctx); err != nil {
		s.log.Warn("embeddings de negocios no actualizados; responde el motor de reglas", "error", err.Error())
	}
}
