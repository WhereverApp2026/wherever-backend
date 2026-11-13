# task.md — NV-37 Grupos e integrantes

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec (firma pendiente); rama `nv-37` creada desde `main` con nv-29/30 y nv-34/36.

## Fase 1 — Implementación
- [x] T02 `POST /grupos` crea el grupo de un plan propio (uno por plan, hasta 12 con el organizador) e invita en el mismo paso.
- [x] T03 `POST /grupos/{id}/integrantes` y `POST /planes/{id}/invitaciones` invitan solo a amigos aceptados con cuenta activa; la segunda crea el grupo si falta. Notificación `invitacion_plan` diferida a nv-40.
- [x] T04 `PATCH /grupos/{id}/integrantes/yo` confirma o declina; `DELETE …/yo` sale; el organizador saca con `DELETE …/{usuario_id}`.
- [x] T05 El organizador ve confirmados, invitados y declinados (`resumen`); los demás no ven a quien declinó.
- [x] T06 Los integrantes invitados o confirmados leen el plan; solo el organizador lo edita, lo borra o lo clona.

## Fase 2 — Cierre
- [x] T07 Pruebas (Unit, Integration, Security, Database, Contract), documentos y `Memoria.md`.
- [ ] T08 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
