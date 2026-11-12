# task.md — NV-51 Búsqueda semántica y evaluación

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec (firma pendiente); [x] rama `nv-51` desde `main`.

## Fase 1 — Implementación
- [x] T02 `GET /lugares/semantica?q=` con producto punto detrás de `semantica.Indice`, caché de 24 h y respaldo por reglas.
- [x] T03 Factor `afinidad_semantica` (pesos × 0,8 + 0,20) y candidatos del asistente con `WithSemantica`.
- [ ] T04 Medición: script y conjunto listos (`scripts/evaluar-semantica`); falta correrlo con IA real y llenar `docs/ml-evaluacion.md`.

## Fase 2 — Cierre
- [x] T05 Pruebas (Unit e `Integration/Semántica 8`), documentos y `Memoria.md`.
- [ ] T06 Local verde (`go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints`); PR abierto; falta el CI del PR (Integration, Security, Database, Contract, cobertura, Semgrep).
