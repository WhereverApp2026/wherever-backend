package neo4j

import (
	"context"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// mutualFriendsQuery son los amigos de mis amigos que no son ya mis amigos, por cantidad de
// amigos en común (nv-53). Todo va parametrizado; $excluir trae a quien ya tiene relación
// conmigo en Mongo (pendiente, aceptada o bloqueada).
const mutualFriendsQuery = `
MATCH (yo:Usuario {id: $id})-[:AMIGO_DE]-(amigo:Usuario)-[:AMIGO_DE]-(otro:Usuario)
WHERE otro <> yo AND NOT otro.id IN $excluir AND NOT (yo)-[:AMIGO_DE]-(otro)
WITH otro, count(DISTINCT amigo) AS comunes
RETURN otro.id AS id, comunes
ORDER BY comunes DESC, id
LIMIT $limite`

// friendsAtQuery son mis amigos con un plan activo que incluye el negocio (nv-53). $amigos
// trae mis amistades aceptadas en Mongo: el grafo es eventual y no debe adelantarse.
const friendsAtQuery = `
MATCH (:Usuario {id: $id})-[:AMIGO_DE]-(a:Usuario)-[:FUE_A]->(:Negocio {id: $negocio})
WHERE a.id IN $amigos
RETURN DISTINCT a.id AS id
ORDER BY id`

// Mutual es una persona sugerida y cuántos amigos tiene en común con quien pregunta.
type Mutual struct {
	ID     string
	Common int64
}

// MutualFriends devuelve hasta limit personas con amigos en común con id.
func (d *Driver) MutualFriends(ctx context.Context, id string, exclude []string, limit int) ([]Mutual, error) {
	res, err := d.read(ctx, mutualFriendsQuery, map[string]any{"id": id, "excluir": toAny(exclude), "limite": int64(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]Mutual, 0, len(res.Records))
	for _, r := range res.Records {
		other, _, err := neo4j.GetRecordValue[string](r, "id")
		if err != nil {
			return nil, err
		}
		common, _, err := neo4j.GetRecordValue[int64](r, "comunes")
		if err != nil {
			return nil, err
		}
		out = append(out, Mutual{ID: other, Common: common})
	}
	return out, nil
}

// FriendsAt devuelve, entre friends, los amigos de id que fueron al negocio business.
func (d *Driver) FriendsAt(ctx context.Context, id, business string, friends []string) ([]string, error) {
	res, err := d.read(ctx, friendsAtQuery, map[string]any{"id": id, "negocio": business, "amigos": toAny(friends)})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(res.Records))
	for _, r := range res.Records {
		friend, _, err := neo4j.GetRecordValue[string](r, "id")
		if err != nil {
			return nil, err
		}
		out = append(out, friend)
	}
	return out, nil
}

func (d *Driver) read(ctx context.Context, query string, params map[string]any) (*neo4j.EagerResult, error) {
	return neo4j.ExecuteQuery(ctx, d.DriverWithContext, query, params, neo4j.EagerResultTransformer, neo4j.ExecuteQueryWithReadersRouting())
}
