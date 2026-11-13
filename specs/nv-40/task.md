# task.md — NV-40 Dispositivos y centro de notificaciones

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec (firma pendiente); rama `nv-40` desde `nv-39`.

## Fase 1 — Implementación
- [x] T02 Registrar y quitar el token del teléfono (`PUT/DELETE /dispositivos`, máximo 5).
- [x] T03 Centro: listar con cursor, no leídas, marcar, leer todas y borrar.
- [x] T04 Avisos `solicitud_amistad`, `amistad_aceptada`, `invitacion_plan`, `votacion_abierta` y `votacion_cerrada` desde sus módulos.

## Fase 2 — Cierre
- [x] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
