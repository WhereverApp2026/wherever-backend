# task.md — NV-56 Panel del negocio: perfil, horarios, pausa, menú y reservas

## Fase 0 — Preparación
- [x] T01 Decisiones E1–E9 resueltas; nv-55 en `main`; rama contra `main`.
- [x] T02 Colección `menu_items` con `ix_menu` e `ix_menu_texto` en `scripts/indexes` y `/ready`; campos nuevos de `negocios`.

## Fase 1 — Implementación
- [x] T03 PATCH /negocios/{id} con `If-Match`, `horario_min` y `cambios_pendientes` en la cola de nv-55.
- [x] T04 POST /negocios/{id}/pausa, `Observador` y reanudación nocturna.
- [x] T05 PUT /negocios/{id}/reservas-config con los campos de nv-59.
- [x] T06 Menú: POST, PATCH, DELETE y GET /lugares/{id}/menu.
- [x] T07 GET /negocios/{id}/panel con `PanelFuente` y regla del Sello; PUT /admin/negocios/{id}/sello.
- [x] T08 GET /negocios/{id}/qr y DELETE /negocios/{id} con `ReservasFuturas` y cancelación del expediente.
- [x] T09 Exporter del comerciante.

## Fase 2 — Cierre
- [x] T10 Pruebas, `docs/endpoints.md` (filas a la tabla viva) y `Memoria.md`.
- [x] T11 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
