# plan.md — NV-52 Proyección de amistades a Neo4j

1. Mapeo
- Crear: `internal/grafo/*`, `cmd/reconstruir-grafo`, pruebas en `tests/`.
- Modificar: `internal/amistades` y `internal/planes` (emitir eventos), `internal/platform/neo4j`, `docs/endpoints.md`, `Memoria.md`.
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
