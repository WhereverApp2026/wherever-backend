package neo4j

import (
	"context"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// projectUser deja las relaciones de un usuario igual a las de Mongo (spec nv-52). Todo va
// parametrizado: el grafo guarda solo ids. Un nodo que se queda sin relaciones se borra, para
// no gastar el límite de nodos de Aura Free.
var projectUser = []string{
	"MERGE (:Usuario {id: $id})",
	`MATCH (:Usuario {id: $id})-[r:AMIGO_DE]-(x:Usuario) WHERE NOT x.id IN $amigos
DELETE r
WITH DISTINCT x WHERE NOT (x)--()
DELETE x`,
	`MATCH (u:Usuario {id: $id})
UNWIND $amigos AS amigo
MERGE (a:Usuario {id: amigo})
MERGE (u)-[:AMIGO_DE]-(a)`,
	`MATCH (:Usuario {id: $id})-[r:FUE_A]->(n:Negocio) WHERE NOT n.id IN $negocios
DELETE r
WITH DISTINCT n WHERE NOT (n)--()
DELETE n`,
	`MATCH (u:Usuario {id: $id})
UNWIND $negocios AS negocio
MERGE (n:Negocio {id: negocio})
MERGE (u)-[:FUE_A]->(n)`,
	"MATCH (u:Usuario {id: $id}) WHERE NOT (u)--() DELETE u",
}

// clearProjection borra lo que proyecta nv-52 y deja los datos de demostración de nv-11.
var clearProjection = []string{
	"MATCH ()-[r:AMIGO_DE|FUE_A]->() DELETE r",
	"MATCH (n) WHERE (n:Usuario OR n:Negocio) AND NOT (n)--() DELETE n",
}

// ProjectUser reemplaza los AMIGO_DE y FUE_A del usuario en una sola transacción. Correrla
// de nuevo con lo mismo no cambia nada.
func (d *Driver) ProjectUser(ctx context.Context, id string, friends, businesses []string) error {
	params := map[string]any{"id": id, "amigos": toAny(friends), "negocios": toAny(businesses)}
	return d.write(ctx, projectUser, params)
}

// ClearProjection borra todas las relaciones AMIGO_DE y FUE_A y los nodos que queden solos.
func (d *Driver) ClearProjection(ctx context.Context) error {
	return d.write(ctx, clearProjection, nil)
}

func (d *Driver) write(ctx context.Context, statements []string, params map[string]any) error {
	session := d.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer func() { _ = session.Close(ctx) }()
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		for _, statement := range statements {
			result, err := tx.Run(ctx, statement, params)
			if err != nil {
				return nil, err
			}
			if _, err := result.Consume(ctx); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

func toAny(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}
