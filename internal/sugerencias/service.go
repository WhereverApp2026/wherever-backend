// Package sugerencias lee el grafo de Neo4j (nv-52) para «personas que quizá conoces» y
// «tus amigos han ido» (nv-53). Mongo manda en quién es amigo, quién tiene una relación y
// quién se deja encontrar; Neo4j solo recorre el grafo.
package sugerencias

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/personas"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

const (
	// MaxSuggestions es el tope de GET /amistades/sugerencias.
	MaxSuggestions = 20
	// candidates es cuántas pide al grafo: quien apagó buscable_por_usuario o no está activo
	// se descarta después en Mongo y aun así deben quedar hasta 20.
	candidates = 100
	// MaxAvatars es cuántos amigos muestra GET /lugares/{id}/amigos.
	MaxAvatars = 3
	// graphTimeout corta la espera de Neo4j (Aura en pausa): después la respuesta es degradada.
	graphTimeout = 3 * time.Second
)

// Graph son las consultas de Neo4j; las cumple *neo4j.Driver.
type Graph interface {
	MutualFriends(ctx context.Context, id string, exclude []string, limit int) ([]neo4j.Mutual, error)
	FriendsAt(ctx context.Context, id, business string, friends []string) ([]string, error)
}

// Relations son las amistades en Mongo; las cumple *amistades.Repository.
type Relations interface {
	ListByUser(ctx context.Context, me bson.ObjectID) ([]amistades.Friendship, error)
}

// Users es lo que hace falta de las cuentas; lo cumple *usuarios.Repository.
type Users interface {
	FindPublicByIDs(ctx context.Context, ids []bson.ObjectID) ([]usuarios.User, error)
	FindSuggestableByIDs(ctx context.Context, ids []bson.ObjectID) ([]usuarios.User, error)
}

// Places dice si un negocio es público; lo cumple *negocios.Repository.
type Places interface {
	FindVisible(ctx context.Context, id bson.ObjectID) (negocios.Business, bool, error)
}

// Service arma las dos respuestas. Sin grafo (o con Neo4j caído) responde vacío y degradado.
type Service struct {
	relations Relations
	users     Users
	places    Places
	graph     Graph
	log       *slog.Logger
}

// NewService recibe los repositorios por constructor; el grafo se agrega con WithGraph.
func NewService(relations Relations, users Users, places Places, log *slog.Logger) (*Service, error) {
	if relations == nil || users == nil || places == nil {
		return nil, errors.New("sugerencias: faltan los repositorios")
	}
	return &Service{relations: relations, users: users, places: places, log: log}, nil
}

// WithGraph conecta Neo4j. Un *neo4j.Driver nulo deja el servicio degradado.
func (s *Service) WithGraph(g *neo4j.Driver) *Service {
	if g != nil {
		s.graph = g
	}
	return s
}

func unavailable(err error) error {
	return apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
}

func self(subject string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("credenciales inválidas")
	}
	return id, nil
}

// degraded deja constancia de que Neo4j no respondió; sin ids ni datos personales.
func (s *Service) degraded(op string, err error) {
	if s.log != nil {
		s.log.Warn("grafo sin respuesta; respuesta degradada", "dependencia", "neo4j", "operacion", op, "error", err.Error())
	}
}

// ask corre una consulta del grafo con plazo; sin grafo o con error dice que la respuesta
// va degradada.
func (s *Service) ask(ctx context.Context, op string, query func(ctx context.Context, g Graph) error) bool {
	if s.graph == nil {
		return true
	}
	gctx, cancel := context.WithTimeout(ctx, graphTimeout)
	defer cancel()
	if err := query(gctx, s.graph); err != nil {
		s.degraded(op, err)
		return true
	}
	return false
}

// hexIDs convierte los ids del grafo; uno mal formado no es de Mongo y se ignora.
func hexIDs(raw []string) []bson.ObjectID {
	out := make([]bson.ObjectID, 0, len(raw))
	for _, r := range raw {
		if id, err := bson.ObjectIDFromHex(r); err == nil {
			out = append(out, id)
		}
	}
	return out
}

// others son las otras personas de mis relaciones; con only, solo las de ese estado.
func (s *Service) others(ctx context.Context, me bson.ObjectID, only string) ([]string, error) {
	relations, err := s.relations.ListByUser(ctx, me)
	if err != nil {
		return nil, unavailable(err)
	}
	out := make([]string, 0, len(relations))
	for _, f := range relations {
		if only == "" || f.Status == only {
			out = append(out, f.Other(me).Hex())
		}
	}
	return out, nil
}

// Suggestions devuelve hasta 20 personas con amigos en común, sin amigos, solicitudes ni
// bloqueos de por medio y sin quien apagó buscable_por_usuario.
func (s *Service) Suggestions(ctx context.Context, subject string) (SuggestionsResponse, error) {
	me, err := self(subject)
	if err != nil {
		return SuggestionsResponse{}, err
	}
	out := SuggestionsResponse{Results: []Suggestion{}}
	exclude, err := s.others(ctx, me, "")
	if err != nil {
		return out, err
	}
	var found []neo4j.Mutual
	out.Degraded = s.ask(ctx, "sugerencias", func(ctx context.Context, g Graph) error {
		found, err = g.MutualFriends(ctx, me.Hex(), exclude, candidates)
		return err
	})
	if out.Degraded {
		return out, nil
	}
	ids := make([]string, 0, len(found))
	for _, m := range found {
		ids = append(ids, m.ID)
	}
	people, err := s.users.FindSuggestableByIDs(ctx, hexIDs(ids))
	if err != nil {
		return out, unavailable(err)
	}
	out.Results = rank(found, people)
	return out, nil
}

// rank sigue el orden del grafo con las cuentas que Mongo dejó pasar, hasta 20.
func rank(found []neo4j.Mutual, people []usuarios.User) []Suggestion {
	byID := make(map[string]usuarios.User, len(people))
	for _, p := range people {
		byID[p.ID.Hex()] = p
	}
	out := []Suggestion{}
	for _, m := range found {
		if p, ok := byID[m.ID]; ok && len(out) < MaxSuggestions {
			out = append(out, Suggestion{Person: person(p), Common: m.Common})
		}
	}
	return out
}

// visiblePlace lee `{id}`: mal formado o no público es 404.
func (s *Service) visiblePlace(ctx context.Context, raw string) (bson.ObjectID, error) {
	place, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return place, apperr.NotFound("lugar no encontrado")
	}
	_, ok, err := s.places.FindVisible(ctx, place)
	if err != nil {
		return place, unavailable(err)
	}
	if !ok {
		return place, apperr.NotFound("lugar no encontrado")
	}
	return place, nil
}

// FriendsAt dice cuántos amigos tienen un plan activo con el negocio y muestra hasta 3.
// Un id mal formado o un negocio no público es 404.
func (s *Service) FriendsAt(ctx context.Context, subject, rawPlace string) (FriendsAtResponse, error) {
	me, err := self(subject)
	if err != nil {
		return FriendsAtResponse{}, err
	}
	place, err := s.visiblePlace(ctx, rawPlace)
	if err != nil {
		return FriendsAtResponse{}, err
	}
	out := FriendsAtResponse{Friends: []personas.Person{}}
	friends, err := s.others(ctx, me, amistades.StatusAccepted)
	if err != nil || len(friends) == 0 {
		return out, err
	}
	var went []string
	out.Degraded = s.ask(ctx, "amigos_lugar", func(ctx context.Context, g Graph) error {
		went, err = g.FriendsAt(ctx, me.Hex(), place.Hex(), friends)
		return err
	})
	if out.Degraded {
		return out, nil
	}
	people, err := s.users.FindPublicByIDs(ctx, hexIDs(went))
	if err != nil {
		return out, unavailable(err)
	}
	out.Total = len(people)
	for _, p := range people[:min(len(people), MaxAvatars)] {
		out.Friends = append(out.Friends, person(p))
	}
	return out, nil
}

func person(u usuarios.User) personas.Person {
	return personas.Person{ID: u.ID.Hex(), Name: u.Name, Username: u.Username, PhotoURL: u.PhotoURL}
}
