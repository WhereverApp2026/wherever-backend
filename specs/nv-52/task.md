# task.md — NV-52 Proyección de amistades a Neo4j

## Fase 0 — Preparación
- [x] T01 Spec revisada y drift corregido; rama desde `main` (aprobación del revisor pendiente).

## Fase 1 — Implementación
- [x] T02 Las amistades aceptadas se reflejan en Neo4j como relaciones AMIGO_DE; los usuarios,…
- [x] T03 La proyección a Neo4j es eventual: Mongo manda; si Neo4j falla, la amistad igual se…
- [x] T04 Un comando reconstruye todo el grafo desde Mongo.
- [x] T05 Sin datos personales en el grafo: solo identificadores.

## Fase 2 — Cierre
- [x] T06 Pruebas, documentos y `Memoria.md`.
- [x] T07 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
