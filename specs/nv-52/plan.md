# plan.md — NV-52 Proyección de amistades a Neo4j

1. Mapeo
- Crear: `internal/grafo/*` (cola, trabajador, reconstrucción, ARCO), `internal/platform/neo4j/proyeccion.go`, `scripts/reconstruir-grafo` (`make reconstruir-grafo`), `tests/integration/grafo_test.go`.
- Modificar: `internal/amistades` y `internal/planes` (`WithGraph`: encolan al usuario), `internal/platform/neo4j/grafo.go` (restricción `negocio_id`), `internal/indexes`, `cmd/api/main.go` (`startGraph`), `internal/testutil/authtest`, `tests/database/indexes_test.go`, `Memoria.md`. `docs/endpoints.md` sin cambios: no hay rutas nuevas.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Cypher siempre parametrizado.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
