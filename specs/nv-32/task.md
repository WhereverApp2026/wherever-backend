# task.md — NV-32 Itinerario con IA y verificación contra el catálogo

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-32`.

## Fase 1 — Implementación
- [ ] T02 Itinerario (US-042): la IA elige de 2 a 4 paradas, en orden horario, con duración…
- [ ] T03 Verificación (US-043): la API descarta cualquier parada que no esté en los…
- [ ] T04 Cada parada trae su por_que (nv-23).

## Fase 2 — Cierre
- [ ] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
