# task.md — NV-38 Enlace de invitación y vista pública

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec (firma pendiente); rama `nv-38` desde `nv-37`.

## Fase 1 — Implementación
- [x] T02 `POST /planes/{id}/compartir` crea un código de 128 bits con vencimiento de 7 días; `DELETE` lo revoca; crear otro reemplaza al anterior.
- [x] T03 Vista pública `POST /planes/compartidos` sin personas, con límite por IP.
- [x] T04 `POST /grupos/unirse` deja a la persona confirmada con el tope de 12 y los bloqueos.

## Fase 2 — Cierre
- [x] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
