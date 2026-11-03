package ciudades

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
)

// Límites del autocompletado (spec nv-05: 8 por defecto, máximo 50).
const (
	DefaultLimit = 8
	MaxLimit     = 50
)

// Service resuelve el autocompletado y la importación.
type Service struct {
	repo *Repository
}

// NewService recibe el repositorio por constructor.
func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// Suggestion es un elemento del autocompletado.
type Suggestion struct {
	ID    string `json:"id"`
	Name  string `json:"nombre"`
	State string `json:"estado"`
}

// Autocomplete sugiere ciudades cuyo nombre empieza con `q` (sin acentos ni mayúsculas);
// primero las que tienen cobertura. Sin coincidencias, lista vacía.
func (s *Service) Autocomplete(ctx context.Context, q string, limit int) ([]Suggestion, error) {
	prefix := text.Normalize(q)
	if len([]rune(prefix)) < 2 {
		return nil, apperr.Validation("datos inválidos", apperr.FieldError{Field: "q", Message: "escribe al menos 2 letras"})
	}
	cities, err := s.repo.Autocomplete(ctx, prefix, int64(limit))
	if err != nil {
		return nil, apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
	}
	out := make([]Suggestion, 0, len(cities))
	for _, c := range cities {
		out = append(out, Suggestion{ID: c.ID.Hex(), Name: c.Name, State: c.State})
	}
	return out, nil
}

// Find busca una ciudad por id (lo usa el perfil al guardar `ciudad`).
func (s *Service) Find(ctx context.Context, id bson.ObjectID) (City, error) {
	return s.repo.FindByID(ctx, id)
}

// ImportRow es un renglón de la importación (CSV con encabezado o arreglo JSON).
type ImportRow struct {
	Name      string  `json:"nombre"`
	State     string  `json:"estado"`
	Latitude  float64 `json:"latitud"`
	Longitude float64 `json:"longitud"`
	Covered   bool    `json:"con_cobertura"`
}

// Import carga ciudades desde CSV (`nombre,estado,latitud,longitud,con_cobertura`) o JSON.
// Es idempotente: repetir el archivo actualiza en vez de duplicar. Devuelve cuántas cargó.
func (s *Service) Import(ctx context.Context, r io.Reader, format string) (int, error) {
	rows, err := parseRows(r, format)
	if err != nil {
		return 0, err
	}
	for i, row := range rows {
		if err := validateRow(row); err != nil {
			return i, fmt.Errorf("renglón %d: %w", i+1, err)
		}
		city := City{Name: strings.TrimSpace(row.Name), State: strings.TrimSpace(row.State), Covered: row.Covered,
			Center: Point{Type: "Point", Coordinates: []float64{row.Longitude, row.Latitude}}}
		if err := s.repo.Upsert(ctx, city); err != nil {
			return i, fmt.Errorf("renglón %d: %w", i+1, err)
		}
	}
	return len(rows), nil
}

func validateRow(row ImportRow) error {
	switch {
	case strings.TrimSpace(row.Name) == "" || strings.TrimSpace(row.State) == "":
		return errors.New("nombre y estado son obligatorios")
	case row.Latitude < -90 || row.Latitude > 90 || row.Longitude < -180 || row.Longitude > 180:
		return errors.New("coordenadas fuera de rango")
	}
	return nil
}

func parseRows(r io.Reader, format string) ([]ImportRow, error) {
	switch format {
	case "json":
		var rows []ImportRow
		return rows, json.NewDecoder(r).Decode(&rows)
	case "csv":
		return parseCSV(r)
	default:
		return nil, fmt.Errorf("formato %q no soportado (csv o json)", format)
	}
}

func parseCSV(r io.Reader) ([]ImportRow, error) {
	records, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	want := []string{"nombre", "estado", "latitud", "longitud", "con_cobertura"}
	if strings.Join(records[0], ",") != strings.Join(want, ",") {
		return nil, fmt.Errorf("encabezado esperado: %s", strings.Join(want, ","))
	}
	rows := make([]ImportRow, 0, len(records)-1)
	for i, rec := range records[1:] {
		lat, errLat := strconv.ParseFloat(rec[2], 64)
		lon, errLon := strconv.ParseFloat(rec[3], 64)
		covered, errCov := strconv.ParseBool(rec[4])
		if err := errors.Join(errLat, errLon, errCov); err != nil {
			return nil, fmt.Errorf("renglón %d: %w", i+1, err)
		}
		rows = append(rows, ImportRow{Name: rec[0], State: rec[1], Latitude: lat, Longitude: lon, Covered: covered})
	}
	return rows, nil
}
