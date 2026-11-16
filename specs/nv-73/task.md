# task.md — NV-73 Mensajes del negocio en una reserva y recordatorio de 2 h

## Fase 0 — Preparación
- [x] T01 Spec revisada (decisiones aplicadas como se proponen; firma pendiente); rama `claude/nv-73-87ba71` contra `main`.
- [x] T02 Colección `mensajes_reserva` con `ix_mensajes_reserva`, `ix_mensajes_usuario` y TTL en `scripts/indexes` y `/ready`; tipos `reserva_mensaje` y `reserva_recordatorio`.

## Fase 1 — Implementación
- [x] T03 POST /negocios/{id}/reservas/{rid}/mensajes (dueño, estados contactables, 5 en 24 h en MongoDB, aviso con ids).
- [x] T04 `GET /reservas/{id}` con los 20 mensajes más nuevos.
- [x] T05 Recordatorio de 2 h en el barrido con `recordatorio_en` condicional; el cambio de hora lo reinicia.
- [x] T06 `reservas.Privacy`: purga y exportación de mensajes.

## Fase 2 — Cierre
- [x] T07 Pruebas, `docs/endpoints.md`, `docs/rate-limit-matrix.md`, estimación de Redis y `Memoria.md`.
- [x] T08 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local (verdes); suites con contenedor en el CI del PR; evidencia en `gates.md`.
