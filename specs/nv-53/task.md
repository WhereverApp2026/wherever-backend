# task.md — NV-53 Sugerencias y señal social

## Fase 0 — Preparación
- [x] T01 Spec revisada y drift corregido; rama desde `main`.

## Fase 1 — Implementación
- [x] T02 GET /amistades/sugerencias: amigos de amigos en Neo4j, filtrados en Mongo, hasta 20.
- [x] T03 GET /lugares/{id}/amigos: total y hasta 3 avatares de amigos con plan activo.

## Fase 2 — Cierre
- [x] T04 Pruebas, documentos y `Memoria.md`.
- [ ] T05 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md` (local en verde; falta el CI).
