package insignias

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Progreso es el avance hacia la meta de una insignia.
type Progreso struct {
	Actual int `json:"actual"`
	Meta   int `json:"meta"`
}

// Insignia es una insignia del catálogo vista por su dueña: ganada o bloqueada.
type Insignia struct {
	ID          string     `json:"id"`
	Nombre      string     `json:"nombre"`
	Descripcion string     `json:"descripcion"`
	Ganada      bool       `json:"ganada"`
	GanadaEn    *time.Time `json:"ganada_en"`
	Progreso    Progreso   `json:"progreso"`
}

// Ganada es una insignia que otra persona puede ver: solo las ganadas, sin progreso.
type Ganada struct {
	ID       string    `json:"id"`
	Nombre   string    `json:"nombre"`
	GanadaEn time.Time `json:"ganada_en"`
}

// MiasResponse es GET /usuarios/yo/insignias.
type MiasResponse struct {
	Insignias []Insignia `json:"insignias"`
	Ganadas   int        `json:"ganadas"`
	Total     int        `json:"total"`
}

// Service calcula las insignias.
type Service struct {
	repo *Repository
	now  func() time.Time
	log  *slog.Logger
}

// NewService recibe el repositorio por constructor.
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, now: time.Now, log: log}
}

func errStore() error { return apperr.Unavailable("servicio no disponible") }

// estado cuenta, guarda las recién ganadas y devuelve conteos y fechas.
func (s *Service) estado(ctx context.Context, userID bson.ObjectID) (map[string]int, map[string]time.Time, error) {
	conteos, err := s.repo.Conteos(ctx, userID)
	if err != nil {
		return nil, nil, errStore()
	}
	fechas, err := s.repo.Ganadas(ctx, userID)
	if err != nil {
		return nil, nil, errStore()
	}
	ahora := s.now().UTC().Truncate(time.Millisecond)
	for _, d := range Catalogo {
		if _, ya := fechas[d.ID]; ya || !d.Ganada(conteos) {
			continue
		}
		if err := s.repo.Guardar(ctx, userID, d.ID, ahora); err != nil {
			return nil, nil, errStore()
		}
		fechas[d.ID] = ahora
		s.log.InfoContext(ctx, "insignia ganada", "evento", "insignia", "insignia", d.ID)
	}
	return conteos, fechas, nil
}

// Mias da el catálogo completo para la dueña: ganadas con su fecha y bloqueadas con su
// avance. Una insignia ya ganada se conserva aunque el conteo baje (borró un plan).
func (s *Service) Mias(ctx context.Context, subject string) (MiasResponse, error) {
	userID, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return MiasResponse{}, apperr.Unauthorized("autenticación requerida")
	}
	conteos, fechas, err := s.estado(ctx, userID)
	if err != nil {
		return MiasResponse{}, err
	}
	out := MiasResponse{Insignias: make([]Insignia, 0, len(Catalogo)), Total: len(Catalogo)}
	for _, d := range Catalogo {
		it := Insignia{ID: d.ID, Nombre: d.Nombre, Descripcion: d.Descripcion, Progreso: Progreso{Actual: d.Progreso(conteos), Meta: d.Meta}}
		if f, ok := fechas[d.ID]; ok {
			f := f.UTC()
			it.Ganada, it.GanadaEn, it.Progreso.Actual = true, &f, d.Meta
			out.Ganadas++
		}
		out.Insignias = append(out.Insignias, it)
	}
	return out, nil
}

// GanadasDe da solo las insignias ganadas de otra persona, en el orden del catálogo. Quien
// llama ya comprobó que puede ver su perfil (personas, nv-70).
func (s *Service) GanadasDe(ctx context.Context, userID bson.ObjectID) ([]Ganada, error) {
	_, fechas, err := s.estado(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := []Ganada{}
	for _, d := range Catalogo {
		if f, ok := fechas[d.ID]; ok {
			out = append(out, Ganada{ID: d.ID, Nombre: d.Nombre, GanadaEn: f.UTC()})
		}
	}
	return out, nil
}
