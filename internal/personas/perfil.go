package personas

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// graphTimeout corta la espera de Neo4j (Aura en pausa); después va degradado (nv-53).
const graphTimeout = 3 * time.Second

// MaxContacts es el tope de teléfonos por petición de POST /personas/contactos (nv-70).
const MaxContacts = 500

// Profiles es lo que el perfil y los contactos leen de las cuentas (nv-70).
type Profiles interface {
	FindByID(ctx context.Context, id bson.ObjectID) (usuarios.User, error)
	FindByPhonesSearchable(ctx context.Context, hashes []string, exclude []bson.ObjectID) ([]usuarios.User, error)
}

// Mutes dice si me silenció a other (amistades, nv-70).
type Mutes interface {
	Muted(ctx context.Context, me, other bson.ObjectID) (bool, error)
}

// CommonGroups cuenta los grupos vivos donde están los dos (grupos, nv-37).
type CommonGroups interface {
	CommonGroups(ctx context.Context, a, b bson.ObjectID) (int64, error)
}

// Graph cuenta los amigos en común en Neo4j; lo cumple *neo4j.Driver.
type Graph interface {
	CommonFriends(ctx context.Context, a, b string) (int64, error)
}

type profileDeps struct {
	profiles Profiles
	mutes    Mutes
	groups   CommonGroups
	graph    Graph
}

// WithProfile conecta el perfil de persona y los contactos (nv-70). Sin grafo, los amigos en
// común van degradados.
func (s *Service) WithProfile(p Profiles, m Mutes, g CommonGroups, graph Graph) *Service {
	s.profile = profileDeps{profiles: p, mutes: m, groups: g, graph: graph}
	return s
}

// WithGraph conecta Neo4j para los amigos en común; un *neo4j.Driver nulo no se conecta.
func (s *Service) WithGraph(g Graph) *Service {
	s.profile.graph = g
	return s
}

// FriendExtras es lo que solo ve un amigo.
type FriendExtras struct {
	Level         string `json:"nivel"`
	CommonFriends *int64 `json:"amigos_en_comun"`
	CommonGroups  int64  `json:"grupos_en_comun"`
	Muted         bool   `json:"silenciado"`
	Degraded      bool   `json:"degradado"`
}

// ProfileResponse es GET /personas/{id}: lo público para cualquiera y, solo entre amigos, los
// extras.
type ProfileResponse struct {
	Person
	Relation string `json:"relacion"`
	*FriendExtras
}

func errUnavailable(err error) error {
	return apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
}

// Profile devuelve el perfil de otra persona. Inexistente, inactiva, propia o con bloqueo en
// cualquier sentido = el mismo 404.
func (s *Service) Profile(ctx context.Context, subject, rawID string) (ProfileResponse, error) {
	me, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return ProfileResponse{}, apperr.Unauthorized("credenciales inválidas")
	}
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return ProfileResponse{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "id", Message: "id inválido"})
	}
	u, rel, err := s.visiblePerson(ctx, me, id)
	if err != nil {
		return ProfileResponse{}, err
	}
	out := ProfileResponse{Person: newPerson(u), Relation: rel}
	if rel == RelationFriend {
		if out.FriendExtras, err = s.friendExtras(ctx, me, u); err != nil {
			return ProfileResponse{}, err
		}
	}
	return out, nil
}

// visiblePerson trae a la persona y su relación conmigo. Inexistente, inactiva, propia o con
// bloqueo en cualquier sentido = el mismo 404.
func (s *Service) visiblePerson(ctx context.Context, me, id bson.ObjectID) (usuarios.User, string, error) {
	notFound := apperr.NotFound("persona no encontrada")
	u, err := s.profile.profiles.FindByID(ctx, id)
	if errors.Is(err, usuarios.ErrNotFound) || (err == nil && (u.ID == me || u.Status != usuarios.StatusActive)) {
		return usuarios.User{}, "", notFound
	}
	if err != nil {
		return usuarios.User{}, "", errUnavailable(err)
	}
	status, mine, err := s.blocks.Relation(ctx, me, id)
	if err != nil {
		return usuarios.User{}, "", errUnavailable(err)
	}
	rel, visible := relationOf(status, mine)
	if !visible {
		return usuarios.User{}, "", notFound
	}
	return u, rel, nil
}

func (s *Service) friendExtras(ctx context.Context, me bson.ObjectID, u usuarios.User) (*FriendExtras, error) {
	out := &FriendExtras{Level: u.Level}
	var err error
	if out.Muted, err = s.profile.mutes.Muted(ctx, me, u.ID); err != nil {
		return nil, errUnavailable(err)
	}
	if out.CommonGroups, err = s.profile.groups.CommonGroups(ctx, me, u.ID); err != nil {
		return nil, errUnavailable(err)
	}
	out.CommonFriends, out.Degraded = s.commonFriends(ctx, me, u.ID)
	return out, nil
}

// commonFriends pregunta a Neo4j con plazo; sin grafo, caído o lento = nil y degradado.
func (s *Service) commonFriends(ctx context.Context, me, other bson.ObjectID) (*int64, bool) {
	if s.profile.graph == nil {
		return nil, true
	}
	gctx, cancel := context.WithTimeout(ctx, graphTimeout)
	defer cancel()
	n, err := s.profile.graph.CommonFriends(gctx, me.Hex(), other.Hex())
	if err != nil {
		if s.log != nil {
			s.log.Warn("grafo sin respuesta; respuesta degradada", "dependencia", "neo4j", "operacion", "perfil", "error", err.Error())
		}
		return nil, true
	}
	return &n, false
}

// ContactsRequest es el cuerpo de POST /personas/contactos; los teléfonos van en claro por TLS
// y nunca se guardan ni se registran.
type ContactsRequest struct {
	Phones []string `json:"telefonos" validate:"required,min=1,max=500,dive,max=40"`
}

// ContactMatch es un contacto con cuenta: `indice` es su posición en la lista enviada.
type ContactMatch struct {
	Index    int    `json:"indice"`
	Person   Person `json:"persona"`
	Relation string `json:"relacion"`
}

// ContactsResponse es POST /personas/contactos; nunca lleva teléfonos ni hashes.
type ContactsResponse struct {
	Results []ContactMatch `json:"resultados"`
}

// normalizeContacts normaliza cada teléfono a E.164 y quita inválidos y repetidos; devuelve el
// HMAC de cada uno con el primer índice donde apareció.
func normalizeContacts(key []byte, phones []string) ([]string, map[string]int) {
	hashes := []string{}
	first := map[string]int{}
	for i, raw := range phones {
		phone, ok, err := parsePhone(trimPhone(raw))
		if !ok || err != nil {
			continue
		}
		h := usuarios.HashPhone(key, phone)
		if _, seen := first[h]; !seen {
			first[h] = i
			hashes = append(hashes, h)
		}
	}
	return hashes, first
}

// Contacts dice qué teléfonos de la agenda tienen cuenta activa y se dejan encontrar por
// teléfono; sin la propia ni bloqueos. No crea documentos ni registra números.
func (s *Service) Contacts(ctx context.Context, subject string, req ContactsRequest) (ContactsResponse, error) {
	me, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return ContactsResponse{}, apperr.Unauthorized("credenciales inválidas")
	}
	hashes, first := normalizeContacts(s.phoneKey, req.Phones)
	if len(hashes) == 0 {
		return ContactsResponse{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "telefonos", Message: "ningún teléfono es válido"})
	}
	exclude, err := s.blocks.BlockedWith(ctx, me)
	if err != nil {
		return ContactsResponse{}, errUnavailable(err)
	}
	found, err := s.profile.profiles.FindByPhonesSearchable(ctx, hashes, append(exclude, me))
	if err != nil {
		return ContactsResponse{}, errUnavailable(err)
	}
	out := ContactsResponse{Results: make([]ContactMatch, 0, len(found))}
	for _, u := range found {
		status, mine, err := s.blocks.Relation(ctx, me, u.ID)
		if err != nil {
			return ContactsResponse{}, errUnavailable(err)
		}
		rel, visible := relationOf(status, mine)
		if !visible {
			continue
		}
		out.Results = append(out.Results, ContactMatch{Index: first[u.PhoneHash], Person: newPerson(u), Relation: rel})
	}
	sortByIndex(out.Results)
	return out, nil
}

func trimPhone(raw string) string { return strings.TrimSpace(raw) }

// sortByIndex deja los resultados en el orden de la agenda.
func sortByIndex(rows []ContactMatch) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].Index < rows[j].Index })
}
