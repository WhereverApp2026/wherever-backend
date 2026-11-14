# task.md — NV-68 Extras de planes: alternativas, cambiar parada, populares y paquete sin conexión

## Fase 0 — Preparación
- [ ] T01 Spec revisada, ola decidida y drift corregido; rama de trabajo contra `main`.
- [ ] T02 Campos `visibilidad`, `veces_clonado`, `plan_origen_id` y `zona`; `ix_populares` en `scripts/indexes` y `/ready`.

## Fase 1 — Implementación
- [ ] T03 GET alternativas (con y sin `q`) y POST cambiar con `If-Match`, `Calculadora` y `sync_seq`.
- [ ] T04 `visibilidad` en PATCH, clonar planes públicos ajenos con `$inc veces_clonado` y GET populares público con caché HTTP.
- [ ] T05 GET paquete-offline con ventana, `ETag` y `no-store`.

## Fase 2 — Cierre
- [ ] T06 Pruebas, `docs/endpoints.md`, `docs/rate-limit-matrix.md` y `Memoria.md`.
- [ ] T07 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
