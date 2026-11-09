# task.md — NV-36 Solicitudes y lista de amigos

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-36`.

## Fase 1 — Implementación
- [ ] T02 POST /amistades envía solicitud: no a uno mismo, no repetida, no a quien me bloqueó; si…
- [ ] T03 GET /amistades?estado= lista amigos (por actividad reciente, con búsqueda por nombre),…
- [ ] T04 PATCH /amistades/{id} acepta o rechaza; rechazar no avisa a quien envió.
- [ ] T05 DELETE /amistades/{id} elimina al amigo o cancela una solicitud; POST…

## Fase 2 — Cierre
- [ ] T06 Pruebas, documentos y `Memoria.md`.
- [ ] T07 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
