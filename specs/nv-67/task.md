# task.md — NV-67 Reportes de información incorrecta de un negocio

## Fase 0 — Preparación
- [ ] T01 Spec revisada y aprobada; ola asignada; rama de trabajo contra `main`.
- [ ] T02 Colección `reportes_negocio` con `ix_reportes_negocio`, `ix_reportes_cola`, `ux_reporte_abierto` e `ix_reportes_usuario` en `scripts/indexes` y `/ready`.

## Fase 1 — Implementación
- [ ] T03 POST /lugares/{id}/reportes: validación, moderación del detalle, un abierto por negocio, 10 por día en MongoDB, dueño = 403.
- [ ] T04 GET /admin/reportes-negocio con estado, negocio y cursor firmado; política `admin`.
- [ ] T05 PATCH /admin/reportes-negocio/{id} con `If-Match` y transición solo desde `abierto`.
- [ ] T06 `reportesnegocio.Privacy` registrado como `Purger` y `Exporter`.

## Fase 2 — Cierre
- [ ] T07 Pruebas, `docs/endpoints.md`, `docs/rate-limit-matrix.md` y `Memoria.md`.
- [ ] T08 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
