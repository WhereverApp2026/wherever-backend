# task.md — NV-47 Subida de acciones hechas sin conexión

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-47`.

## Fase 1 — Implementación
- [ ] T02 POST /sync/pendientes recibe hasta 100 acciones hechas sin conexión, cada una con un…
- [ ] T03 Idempotente: repetir el mismo lote no duplica nada; cada acción devuelve su resultado…
- [ ] T04 Conflictos: si la version que trae la acción no coincide con el servidor, no se…

## Fase 2 — Cierre
- [ ] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
