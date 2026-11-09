# task.md — NV-17 Paginación por cursor y validadores HTTP

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-17`.

## Fase 1 — Implementación
- [ ] T02 Paginación por cursor en todas las listas: ?limite= (20 por defecto, 50 máximo) y…
- [ ] T03 Validadores HTTP: ETag e If-None-Match en los GET públicos de catálogo (304 si no…

## Fase 2 — Cierre
- [ ] T04 Pruebas, documentos y `Memoria.md`.
- [ ] T05 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
