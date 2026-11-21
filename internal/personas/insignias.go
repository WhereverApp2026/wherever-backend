package personas

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/insignias"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Insignias son las ganadas de una persona (nv-78).
type Insignias interface {
	GanadasDe(ctx context.Context, userID bson.ObjectID) ([]insignias.Ganada, error)
}

// InsigniasResponse es GET /personas/{id}/insignias.
type InsigniasResponse struct {
	Insignias []insignias.Ganada `json:"insignias"`
}

// WithInsignias conecta las insignias; sin ellas la ruta responde 503.
func (s *Service) WithInsignias(i Insignias) *Service {
	s.insignias = i
	return s
}

// InsigniasDe da las insignias ganadas de otra persona con las mismas reglas que su perfil:
// inexistente, inactiva, propia o con bloqueo en cualquier sentido = el mismo 404; con
// `actividad: nadie`, la lista vacía.
func (s *Service) InsigniasDe(ctx context.Context, subject, rawID string) (InsigniasResponse, error) {
	if s.insignias == nil {
		return InsigniasResponse{}, apperr.Unavailable("servicio no disponible")
	}
	me, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return InsigniasResponse{}, apperr.Unauthorized("credenciales inválidas")
	}
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return InsigniasResponse{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "id", Message: "id inválido"})
	}
	u, _, err := s.visiblePerson(ctx, me, id)
	if err != nil {
		return InsigniasResponse{}, err
	}
	// Con `actividad: nadie` no se ve ninguna (nv-82); la forma de la respuesta no cambia.
	if u.Privacy.Social.HidesActivity() {
		return InsigniasResponse{Insignias: []insignias.Ganada{}}, nil
	}
	ganadas, err := s.insignias.GanadasDe(ctx, u.ID)
	if err != nil {
		return InsigniasResponse{}, err
	}
	return InsigniasResponse{Insignias: ganadas}, nil
}
