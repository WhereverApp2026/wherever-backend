package neo4j

import (
	"context"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// schema son las restricciones del grafo; cada una crea además su índice (nv-11, requisito 6).
var schema = []string{
	"CREATE CONSTRAINT usuario_id IF NOT EXISTS FOR (u:Usuario) REQUIRE u.id IS UNIQUE",
	"CREATE CONSTRAINT lugar_id IF NOT EXISTS FOR (l:Lugar) REQUIRE l.id IS UNIQUE",
	"CREATE CONSTRAINT categoria_nombre IF NOT EXISTS FOR (c:Categoria) REQUIRE c.nombre IS UNIQUE",
}

// seedQuery deja el grafo de demostración. Con MERGE, correrla de nuevo no duplica nada.
const seedQuery = `
UNWIND $categorias AS nombre
MERGE (:Categoria {nombre: nombre})
WITH count(*) AS listo
UNWIND $lugares AS lugar
MERGE (l:Lugar {id: lugar.id}) SET l.nombre = lugar.nombre
WITH l, lugar
MATCH (c:Categoria {nombre: lugar.categoria})
MERGE (l)-[:ES_DE]->(c)
WITH count(*) AS listo
UNWIND $usuarios AS usuario
MERGE (u:Usuario {id: usuario.id}) SET u.nombre = usuario.nombre
WITH u, usuario
MATCH (c:Categoria {nombre: usuario.gusta})
MERGE (u)-[:GUSTA]->(c)
WITH u, usuario
MATCH (l:Lugar {id: usuario.visito})
MERGE (u)-[:VISITO]->(l)`

func seedParams() map[string]any {
	return map[string]any{
		"categorias": []any{"cafe", "tacos", "museos"},
		"lugares": []any{
			map[string]any{"id": "demo-lugar-1", "nombre": "Café Demostración", "categoria": "cafe"},
			map[string]any{"id": "demo-lugar-2", "nombre": "Taquería Demostración", "categoria": "tacos"},
			map[string]any{"id": "demo-lugar-3", "nombre": "Museo Demostración", "categoria": "museos"},
		},
		"usuarios": []any{
			map[string]any{"id": "demo-usuario-1", "nombre": "Ana Demo", "gusta": "cafe", "visito": "demo-lugar-1"},
			map[string]any{"id": "demo-usuario-2", "nombre": "Luis Demo", "gusta": "tacos", "visito": "demo-lugar-2"},
			map[string]any{"id": "demo-usuario-3", "nombre": "Sara Demo", "gusta": "museos", "visito": "demo-lugar-3"},
		},
	}
}

// Size es el tamaño del grafo frente a los límites del plan gratuito.
type Size struct {
	Nodes         int64
	Relationships int64
}

// EnsureSchema crea las restricciones que falten.
func (d *Driver) EnsureSchema(ctx context.Context) error {
	for _, statement := range schema {
		if _, err := d.run(ctx, statement, nil); err != nil {
			return err
		}
	}
	return nil
}

// SeedDemo deja los datos de demostración sin tocar los demás.
func (d *Driver) SeedDemo(ctx context.Context) error {
	_, err := d.run(ctx, seedQuery, seedParams())
	return err
}

// GraphSize cuenta nodos y relaciones.
func (d *Driver) GraphSize(ctx context.Context) (Size, error) {
	nodes, err := d.count(ctx, "MATCH (n) RETURN count(n) AS total")
	if err != nil {
		return Size{}, err
	}
	relationships, err := d.count(ctx, "MATCH ()-[r]->() RETURN count(r) AS total")
	if err != nil {
		return Size{}, err
	}
	return Size{Nodes: nodes, Relationships: relationships}, nil
}

func (d *Driver) count(ctx context.Context, query string) (int64, error) {
	result, err := d.run(ctx, query, nil)
	if err != nil {
		return 0, err
	}
	total, _, err := neo4j.GetRecordValue[int64](result.Records[0], "total")
	return total, err
}

func (d *Driver) run(ctx context.Context, query string, params map[string]any) (*neo4j.EagerResult, error) {
	return neo4j.ExecuteQuery(ctx, d.DriverWithContext, query, params, neo4j.EagerResultTransformer)
}
