# task.md — NV-52 Proyección de amistades a Neo4j

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-52`.

## Fase 1 — Implementación
- [ ] T02 Las amistades aceptadas se reflejan en Neo4j como relaciones AMIGO_DE; los usuarios,…
- [ ] T03 La proyección a Neo4j es eventual: Mongo manda; si Neo4j falla, la amistad igual se…
- [ ] T04 Un comando reconstruye todo el grafo desde Mongo.
- [ ] T05 Sin datos personales en el grafo: solo identificadores.

## Fase 2 — Cierre
- [ ] T06 Pruebas, documentos y `Memoria.md`.
- [ ] T07 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
