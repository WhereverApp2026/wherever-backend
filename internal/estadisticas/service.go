package estadisticas

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Negocios es lo que estadisticas lee de nv-56: el negocio para validar propiedad, plan,
// categoría y alta.
type Negocios interface {
	FindByID(ctx context.Context, id bson.ObjectID) (negocios.Business, error)
}

// Service arma las estadísticas, el reporte y los trabajos que las llenan.
type Service struct {
	repo     *Repository
	negocios Negocios
	zona     *time.Location
	log      *slog.Logger
	now      func() time.Time
}

// NewService recibe sus dependencias por constructor.
func NewService(repo *Repository, n Negocios, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	zona, err := time.LoadLocation(Zona)
	if err != nil {
		zona = time.UTC
	}
	return &Service{repo: repo, negocios: n, zona: zona, log: log, now: time.Now}
}

// WithReloj cambia la hora de referencia (pruebas de días cerrados y de meses).
func (s *Service) WithReloj(now func() time.Time) *Service {
	s.now = now
	return s
}

func errStore() error        { return apperr.Unavailable("servicio no disponible") }
func errNoEncontrado() error { return apperr.NotFound("negocio no encontrado") }

func invalido(campo, msg string) error {
	return apperr.Validation("datos inválidos", apperr.FieldError{Field: campo, Message: msg})
}

// propio trae el negocio de la persona sin baja. Uno ajeno da 404 para no confirmar que existe.
func (s *Service) propio(ctx context.Context, claims *middleware.Claims, raw string) (negocios.Business, error) {
	if claims == nil {
		return negocios.Business{}, apperr.Unauthorized("autenticación requerida")
	}
	owner, err := bson.ObjectIDFromHex(claims.Subject)
	if err != nil {
		return negocios.Business{}, apperr.Unauthorized("sesión inválida")
	}
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return negocios.Business{}, errNoEncontrado()
	}
	b, err := s.negocios.FindByID(ctx, id)
	if errors.Is(err, negocios.ErrNotFound) || (err == nil && (b.DeletedAt != nil || b.OwnerID != owner)) {
		return negocios.Business{}, errNoEncontrado()
	}
	if err != nil {
		return negocios.Business{}, errStore()
	}
	return b, nil
}

// --- días locales ---

// fecha es el día calendario de t en la zona, como medianoche UTC (para sumar días sin
// cambios de horario).
func fecha(t time.Time, zona *time.Location) time.Time {
	t = t.In(zona)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// instante es el inicio del día calendario d en la zona.
func instante(d time.Time, zona *time.Location) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, zona)
}

func dia(d time.Time) string { return d.Format(FormatoDia) }

func dias(desde, hasta time.Time) int { return int(hasta.Sub(desde).Hours()/24) + 1 }

func (s *Service) hoy() time.Time { return fecha(s.now(), s.zona) }

// --- contador de vistas (vistas.Contador) ---

// Contar suma la vista contada por nv-66 al día local del negocio. Sin Redis.
func (s *Service) Contar(ctx context.Context, negocio bson.ObjectID, origen string, now time.Time) error {
	return s.repo.Contar(ctx, negocio, dia(fecha(now, s.zona)), origen, now.UTC())
}

// --- trabajos ---

// escribir recalcula los días [desde, hasta] del negocio. Con `ceros`, los días sin actividad
// se escriben en cero (así baja una cifra que cambió); sin él solo se escriben los días con algo.
func (s *Service) escribir(ctx context.Context, id bson.ObjectID, desde, hasta time.Time, ceros bool) error {
	b, err := s.negocios.FindByID(ctx, id)
	if errors.Is(err, negocios.ErrNotFound) || (err == nil && b.DeletedAt != nil) {
		return nil
	}
	if err != nil {
		return err
	}
	act, err := s.repo.Actividad(ctx, id, instante(desde, s.zona), instante(hasta.AddDate(0, 0, 1), s.zona))
	if err != nil {
		return err
	}
	if ceros {
		for d := desde; !d.After(hasta); d = d.AddDate(0, 0, 1) {
			if _, ok := act[dia(d)]; !ok {
				act[dia(d)] = Actividad{}
			}
		}
	}
	return s.repo.Escribir(ctx, id, b.Category, act, s.now().UTC())
}

// Recalcular rehace hoy y ayer de cada negocio con actividad desde ayer. Los días anteriores
// quedan congelados. Devuelve cuántos negocios tocó.
func (s *Service) Recalcular(ctx context.Context) (int, error) {
	hoy := s.hoy()
	ayer := hoy.AddDate(0, 0, -1)
	ids, err := s.repo.ConActividad(ctx, instante(ayer, s.zona))
	if err != nil {
		return 0, err
	}
	var errs []error
	for _, id := range ids {
		if err := s.escribir(ctx, id, ayer, hoy, true); err != nil {
			errs = append(errs, err)
		}
	}
	return len(ids), errors.Join(errs...)
}

// Rellenar llena los últimos 90 días de hasta 50 negocios aprobados que aún no los tienen. Las
// vistas anteriores a nv-74 quedan en 0.
func (s *Service) Rellenar(ctx context.Context) (int, error) {
	ids, err := s.repo.PendientesRelleno(ctx, LoteRelleno)
	if err != nil {
		return 0, err
	}
	hoy := s.hoy()
	n := 0
	for _, id := range ids {
		if err := s.escribir(ctx, id, hoy.AddDate(0, 0, -DiasRelleno), hoy, false); err != nil {
			return n, err
		}
		if err := s.repo.MarcarRelleno(ctx, id, s.now().UTC()); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// Nocturno resume las categorías del mes de ayer y del anterior y borra los días de más de
// 400 días. Devuelve cuántos días borró.
func (s *Service) Nocturno(ctx context.Context) (int, error) {
	hoy := s.hoy()
	ayer := hoy.AddDate(0, 0, -1)
	mes := time.Date(ayer.Year(), ayer.Month(), 1, 0, 0, 0, 0, time.UTC)
	now := s.now().UTC()
	for _, m := range []time.Time{mes, mes.AddDate(0, -1, 0)} {
		if _, err := s.repo.CalcularCategorias(ctx, m.Format(FormatoMes), now); err != nil {
			return 0, err
		}
	}
	borrados, err := s.repo.Limpiar(ctx, dia(hoy.AddDate(0, 0, -DiasRetencion)))
	return int(borrados), err
}

// BorrarNegocio borra las estadísticas del negocio dado de baja.
func (s *Service) BorrarNegocio(ctx context.Context, negocio bson.ObjectID) error {
	return s.repo.BorrarNegocio(ctx, negocio)
}
