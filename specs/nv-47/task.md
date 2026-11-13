# task.md — NV-47 Subida de acciones hechas sin conexión

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec (firma pendiente); rama `nv-47` desde `nv-46`.

## Fase 1 — Implementación
- [x] T02 `POST /sync/pendientes` con hasta 100 acciones y `id_cliente` único.
- [x] T03 Idempotencia con `sync_acciones` (7 días): repetir responde `duplicada`.
- [x] T04 Conflictos sin sobrescribir, con la copia del servidor.

## Fase 2 — Cierre
- [x] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
