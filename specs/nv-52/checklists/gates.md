# gates.md — NV-52 Proyección de amistades a Neo4j

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (local, 2026-11-12) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; los logs del grafo no llevan ids | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin cambios de infraestructura ni pipeline | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (64 rutas)`; sin rutas nuevas | ✅ |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` v2.15.0 | local en Docker: sin errores, `0 issues` | ✅ |
| Pruebas | Integration, Security, Database y Contract con servicios reales | CI del PR | pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)` | ✅ |
| Fuente de verdad | Mongo sin cambios si Neo4j falla | `TestProyeccion_Neo4jCaidoReintentaYLaAmistadSeGuarda` (CI) | pendiente |
| Cypher | Sin concatenación de texto | `internal/platform/neo4j/proyeccion.go`: consultas constantes con `$id`, `$amigos` y `$negocios`; Semgrep del PR | pendiente |
| Reconstrucción | Grafo igual a Mongo | `TestReconstruir_CoincideConMongo` (CI) | pendiente |
