package recomendaciones

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendador"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Users lee al usuario que pide; la implementa *usuarios.Repository.
type Users interface {
	FindByID(ctx context.Context, id bson.ObjectID) (usuarios.User, error)
}

// Places da candidatos y comprueba que un negocio sea público; la implementa
// *negocios.Repository.
type Places interface {
	recomendador.Fuente
	FindVisible(ctx context.Context, id bson.ObjectID) (negocios.Business, bool, error)
}

// Service arma «Para ti ahora» y guarda los descartes.
type Service struct {
	repo   *Repository
	users  Users
	places Places
	pesos  recomendador.Pesos
	log    *slog.Logger
	now    func() time.Time
}

// NewService valida los pesos al arrancar: con pesos inválidos la API no levanta.
func NewService(repo *Repository, users Users, places Places, pesos recomendador.Pesos, log *slog.Logger) (*Service, error) {
	if err := pesos.Validate(); err != nil {
		return nil, errors.Join(recomendador.ErrPesos, err)
	}
	return &Service{repo: repo, users: users, places: places, pesos: pesos, log: log, now: time.Now}, nil
}

// WithClock fija el reloj (pruebas).
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

func userID(subject string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("sesión inválida")
	}
	return id, nil
}

// businessID: un id mal formado es un negocio que no existe.
func businessID(raw string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return bson.ObjectID{}, apperr.NotFound("negocio no encontrado")
	}
	return id, nil
}

// Recommend devuelve «Para ti ahora» del usuario.
func (s *Service) Recommend(ctx context.Context, subject string, q Query) (Response, error) {
	uid, err := userID(subject)
	if err != nil {
		return Response{}, err
	}
	user, err := s.users.FindByID(ctx, uid)
	if errors.Is(err, usuarios.ErrNotFound) {
		return Response{}, apperr.Unauthorized("sesión inválida")
	}
	if err != nil {
		return Response{}, err
	}
	excluded, err := s.repo.Excluded(ctx, uid)
	if err != nil {
		return Response{}, err
	}
	now := s.now()
	c := Contexto(user, q, now, excluded)
	recs, err := recomendador.Recomendar(ctx, s.places, s.pesos, c, q.Limit)
	if err != nil {
		// Sin el texto de MongoDB: podría repetir la ubicación o las alergias.
		s.log.ErrorContext(ctx, "recomendaciones: consulta fallida", slog.String("evento", "recomendaciones_error"))
		return Response{}, apperr.Unavailable("servicio no disponible")
	}
	out := Response{Items: make([]Card, 0, len(recs)), Company: q.Company, Moment: c.Momento}
	for _, r := range recs {
		out.Items = append(out.Items, toCard(r, c.Momento))
	}
	return out, nil
}

func toCard(r recomendador.Recomendacion, at time.Time) Card {
	reasons := r.Razones
	if reasons == nil {
		reasons = []string{}
	}
	return Card{
		PlaceItem: negocios.ItemOf(negocios.Candidate{Business: r.Negocio, DistanceM: r.DistanciaM}, at),
		Score:     r.Puntaje, Reasons: reasons, Why: r.PorQue,
	}
}

// NotInterested marca un negocio público con «No me interesa». Repetirlo da lo mismo.
func (s *Service) NotInterested(ctx context.Context, subject, rawBusiness string) error {
	uid, err := userID(subject)
	if err != nil {
		return err
	}
	bid, err := businessID(rawBusiness)
	if err != nil {
		return err
	}
	if _, ok, err := s.places.FindVisible(ctx, bid); err != nil {
		return err
	} else if !ok {
		return apperr.NotFound("negocio no encontrado")
	}
	return s.repo.Mark(ctx, uid, bid, s.now().UTC())
}

// Undo deshace «No me interesa». Solo ve los descartes propios: el de otro usuario es 404.
func (s *Service) Undo(ctx context.Context, subject, rawBusiness string) error {
	uid, err := userID(subject)
	if err != nil {
		return err
	}
	bid, err := businessID(rawBusiness)
	if err != nil {
		return err
	}
	removed, err := s.repo.Unmark(ctx, uid, bid)
	if err != nil {
		return err
	}
	if !removed {
		return apperr.NotFound("el negocio no estaba descartado")
	}
	return nil
}
