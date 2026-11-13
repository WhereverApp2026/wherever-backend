# task.md — NV-41 Envío de push con Firebase

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec (firma pendiente); rama `nv-41` desde `nv-40`.

## Fase 1 — Implementación
- [x] T02 Cliente FCM HTTP v1 con la cuenta de servicio (sin librerías nuevas).
- [x] T03 Cola en segundo plano con reintentos; FCM caído no frena la acción.
- [x] T04 Preferencias por tipo y horario silencioso en la hora de México.
- [x] T05 Tokens `UNREGISTERED` se borran; texto genérico sin datos.

## Fase 2 — Cierre
- [x] T06 Pruebas, documentos y `Memoria.md`.
- [ ] T07 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
- [ ] T08 Prueba en vivo con un teléfono real (requiere las credenciales de Firebase del equipo).
