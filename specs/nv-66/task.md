# task.md — NV-66 Actividad de lugares: conteo, afluencia y vistos recientemente

## Fase 0 — Preparación
- [ ] T01 Spec revisada y aprobada; ola asignada; rama de trabajo contra `main`.
- [ ] T02 Colección `historial_vistas` con `ux_vista`, `ix_recientes` y `ttl_vistas` en `scripts/indexes` y `/ready`.

## Fase 1 — Implementación
- [ ] T03 GET /lugares/conteo con los filtros de la lista, tope 1000 y `maxTimeMS`.
- [ ] T04 GET /lugares/{id}/afluencia con `dia`, `hora_actual` y `suficiente`.
- [ ] T05 POST /lugares/{id}/vistas (upsert, ventana de 10 min, 60 lugares por hora en MongoDB), GET y DELETE /lugares/recientes; rutas antes de `/lugares/:id`.
- [ ] T06 `vistas.Privacy` registrado como `Purger` y `Exporter`.

## Fase 2 — Cierre
- [ ] T07 Pruebas, `docs/endpoints.md`, `docs/rate-limit-matrix.md`, estimación de Redis y `Memoria.md`.
- [ ] T08 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
