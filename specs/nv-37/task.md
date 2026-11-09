# task.md — NV-37 Grupos e integrantes

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-37`.

## Fase 1 — Implementación
- [ ] T02 POST /grupos crea un grupo sobre un plan propio (hasta 12 integrantes, el organizador…
- [ ] T03 POST /planes/{id}/invitaciones y POST /grupos/{id}/integrantes invitan solo a…
- [ ] T04 El integrante confirma o declina (PATCH /grupos/{id}/integrantes/yo); puede salir, y el…
- [ ] T05 El organizador ve quién aceptó y quién no.
- [ ] T06 El plan del grupo lo ven y lo leen los integrantes; solo el organizador lo edita.

## Fase 2 — Cierre
- [ ] T07 Pruebas, documentos y `Memoria.md`.
- [ ] T08 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
