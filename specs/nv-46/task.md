# task.md — NV-46 Descarga de cambios

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec (firma pendiente); rama `nv-46` desde `nv-41`.

## Fase 1 — Implementación
- [x] T02 Contador por usuario sin relojes y numeración de cada cambio de los planes.
- [x] T03 Borrados con TTL de 30 días; 410 para una descarga completa.
- [x] T04 `GET /sync/cambios` paginado, solo lo propio, con ajustes cuando cambian.

## Fase 2 — Cierre
- [x] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
