# task.md — NV-65 Eventos, ferias y tianguis

## Fase 0 — Preparación
- [x] T01 Spec revisada y aprobada; ola asignada; rama de trabajo contra `main`.
- [x] T02 Colección `eventos` con `ix_eventos_cerca`, `ix_eventos_fin`, `ix_eventos_creador` e `ix_eventos_negocio` en `scripts/indexes` y `/ready`.

## Fase 1 — Implementación
- [x] T03 GET /eventos (cerca, próximos `dias`, cursor firmado) y GET /eventos/{id}.
- [x] T04 POST /eventos: rol, negocio aprobado propio, fechas, moderación de texto; 10 por día.
- [x] T05 PATCH y DELETE /eventos/{id} con `If-Match`, permisos y cancelación sin reabrir.
- [x] T06 `eventos.Privacy` registrado como `Purger` y `Exporter`.

## Fase 2 — Cierre
- [x] T07 Pruebas, `docs/endpoints.md`, `docs/rate-limit-matrix.md` y `Memoria.md`.
- [x] T08 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
