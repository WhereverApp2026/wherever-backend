# task.md — NV-46 Descarga de cambios

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-46`.

## Fase 1 — Implementación
- [ ] T02 GET /sync/cambios?desde= devuelve, paginado, lo que cambió para esta persona desde un…
- [ ] T03 Un reloj atrasado en el teléfono no rompe la sincronización.
- [ ] T04 Solo se sincronizan datos de la propia persona.

## Fase 2 — Cierre
- [ ] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
