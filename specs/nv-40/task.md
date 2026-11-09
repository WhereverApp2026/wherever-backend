# task.md — NV-40 Dispositivos y centro de notificaciones

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-40`.

## Fase 1 — Implementación
- [ ] T02 Registrar y quitar el token del dispositivo: PUT/DELETE /dispositivos/{fcm_token}…
- [ ] T03 Centro de notificaciones: GET /notificaciones (cursor), GET…
- [ ] T04 Tipos: solicitud_amistad, amistad_aceptada, invitacion_plan, votacion_abierta,…

## Fase 2 — Cierre
- [ ] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
