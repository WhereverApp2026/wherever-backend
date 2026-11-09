# task.md — NV-41 Envío de push con Firebase

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-41`.

## Fase 1 — Implementación
- [ ] T02 Todo evento crea la notificación en el centro y, si corresponde, envía push por FCM.
- [ ] T03 El push respeta las preferencias del usuario por tipo y el horario silencioso; en…
- [ ] T04 Tokens inválidos (UNREGISTERED) se borran solos.
- [ ] T05 Envío en segundo plano con reintentos; un fallo de FCM nunca cambia la respuesta de la…
- [ ] T06 Sin contenido sensible en el push: título y texto genéricos.

## Fase 2 — Cierre
- [ ] T07 Pruebas, documentos y `Memoria.md`.
- [ ] T08 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
