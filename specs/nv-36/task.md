# task.md — NV-36 Solicitudes y lista de amigos

## Fase 0 — Preparación
- [x] T01 Spec revisada y drift corregido; rama de trabajo contra `main`.

## Fase 1 — Implementación
- [x] T02 POST /amistades envía solicitud: no a uno mismo, no repetida, no a quien me bloqueó; si la otra persona ya envió, quedan amigos; 20 por hora.
- [x] T03 GET /amistades?estado= lista amigos (por actividad reciente, con búsqueda por nombre), recibidas y enviadas, con el contador de pendientes.
- [x] T04 PATCH /amistades/{id} acepta o rechaza; rechazar no avisa a quien envió.
- [x] T05 DELETE /amistades/{id} elimina al amigo o cancela una solicitud; POST /amistades/{id}/bloquear bloquea y oculta en `GET /personas/buscar`.

## Fase 2 — Cierre
- [x] T06 Pruebas, documentos y `Memoria.md`.
- [ ] T07 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local ✅; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
