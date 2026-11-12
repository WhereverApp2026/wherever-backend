package busquedas

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
)

// Service resuelve el autocompletar, la búsqueda y las búsquedas recientes.
type Service struct {
	repo *Repository
	log  *slog.Logger
	now  func() time.Time
}

// NewService recibe el repositorio y el logger por constructor.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log, now: time.Now}
}

// WithClock fija el reloj (pruebas de `abierto_ahora` y de orden de recientes).
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

func pocasLetras() error {
	return apperr.Validation("datos inválidos", apperr.FieldError{Field: "q", Message: "escribe al menos 2 letras"})
}

// noDisponible no lleva el texto de MongoDB: podría repetir lo que la persona escribió.
func noDisponible() error { return apperr.Unavailable("servicio no disponible") }

// Autocompletar sugiere negocios aprobados con una palabra del nombre que empieza con `q`
// (sin acentos ni mayúsculas): primero los que empiezan así, luego por calificación.
func (s *Service) Autocompletar(ctx context.Context, q string, limite int) (AutocompletarRespuesta, error) {
	prefijo := text.Normalize(q)
	if len([]rune(prefijo)) < minLetras {
		return AutocompletarRespuesta{}, pocasLetras()
	}
	rows, err := s.repo.Prefijo(ctx, prefijo)
	if err != nil {
		return AutocompletarRespuesta{}, noDisponible()
	}
	// Estable: dentro de cada grupo queda el orden por calificación de MongoDB.
	slices.SortStableFunc(rows, func(a, b negocios.Business) int {
		return boolRank(strings.HasPrefix(a.NormalizedName, prefijo)) - boolRank(strings.HasPrefix(b.NormalizedName, prefijo))
	})
	items := make([]Sugerencia, 0, min(len(rows), limite))
	for _, b := range rows[:min(len(rows), limite)] {
		items = append(items, Sugerencia{ID: b.ID.Hex(), Slug: b.Slug, Nombre: b.Name, Categoria: b.Category})
	}
	return AutocompletarRespuesta{Items: items}, nil
}

func boolRank(first bool) int {
	if first {
		return 0
	}
	return 1
}

// Buscar usa primero el índice de texto en español; si no llena la página, completa con los
// negocios que coinciden con errores menores (Aproximados). Con sesión, guarda la búsqueda.
func (s *Service) Buscar(ctx context.Context, userID, q string, limite int) (BuscarRespuesta, error) {
	words := Palabras(q)
	if len([]rune(strings.Join(words, ""))) < minLetras {
		return BuscarRespuesta{}, pocasLetras()
	}
	found, err := s.repo.Texto(ctx, strings.Join(words, " "), limite)
	if err != nil {
		return BuscarRespuesta{}, noDisponible()
	}
	aproximada := false
	if len(found) < limite {
		visibles, err := s.repo.Visibles(ctx)
		if err != nil {
			return BuscarRespuesta{}, noDisponible()
		}
		ya := make(map[bson.ObjectID]bool, len(found))
		for _, b := range found {
			ya[b.ID] = true
		}
		extra := Aproximados(words, visibles, ya, limite-len(found))
		aproximada = len(extra) > 0
		found = append(found, extra...)
	}
	s.guardar(ctx, userID, q, words)
	now := s.now()
	items := make([]negocios.PlaceItem, 0, len(found))
	for _, b := range found {
		items = append(items, negocios.CardOf(b, now))
	}
	return BuscarRespuesta{Items: items, Aproximada: aproximada}, nil
}

// guardar registra la búsqueda de quien tiene sesión. Si falla, la búsqueda responde igual.
func (s *Service) guardar(ctx context.Context, userID, q string, words []string) {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return // sin sesión
	}
	if err := s.repo.Guardar(ctx, id, strings.TrimSpace(q), strings.Join(words, " "), s.now().UTC()); err != nil {
		s.log.WarnContext(ctx, "busquedas: no se guardó la búsqueda reciente")
	}
}

func usuario(userID string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("autenticación requerida")
	}
	return id, nil
}

// Recientes devuelve las búsquedas del usuario, de la más nueva a la más vieja.
func (s *Service) Recientes(ctx context.Context, userID string) (RecientesRespuesta, error) {
	id, err := usuario(userID)
	if err != nil {
		return RecientesRespuesta{}, err
	}
	rows, err := s.repo.Recientes(ctx, id)
	if err != nil {
		return RecientesRespuesta{}, noDisponible()
	}
	items := make([]RecienteItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, RecienteItem{Texto: r.Text, BuscadoEn: r.SearchedAt.UTC()})
	}
	return RecientesRespuesta{Items: items}, nil
}

// BorrarRecientes elimina todas las búsquedas del usuario; sin búsquedas también es éxito.
func (s *Service) BorrarRecientes(ctx context.Context, userID string) error {
	id, err := usuario(userID)
	if err != nil {
		return err
	}
	if err := s.repo.Borrar(ctx, id); err != nil {
		return noDisponible()
	}
	return nil
}
