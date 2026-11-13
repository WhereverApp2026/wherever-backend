# task.md — NV-64 Reporte de errores de la app y búsqueda por código de amigo

## Fase 0 — Preparación
- [ ] T01 Spec revisada y drift corregido; rama de trabajo contra `main`.
- [ ] T02 Colección `errores_app` con `ux_huella`, `ix_errores_modulo` y `ttl_errores` en `scripts/indexes` y `/ready`.

## Fase 1 — Implementación
- [ ] T03 POST /errores: validación, filtro de datos personales, huella y `upsert`; 30 por hora.
- [ ] T04 GET /admin/errores con módulo, `desde` y cursor firmado; solo administrador.
- [ ] T05 GET /personas/codigo/{codigo} con `relacion`, 404 uniforme y 30 por hora.

## Fase 2 — Cierre
- [ ] T06 Pruebas, `docs/endpoints.md`, `docs/rate-limit-matrix.md` y `Memoria.md`.
- [ ] T07 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
