# task.md — NV-71 Dispositivos por id y reglas de moderación por filtro

## Fase 0 — Preparación
- [ ] T01 Spec revisada y decisiones del revisor tomadas; filas en `docs/endpoints.md`; rama de trabajo contra `main`.

## Fase 1 — Implementación
- [ ] T02 `Location` en `PUT /dispositivos`; `PUT /dispositivos/{id}` con `app_version` y `canales_activos`; `DELETE /dispositivos/{id}`; dueño en la consulta.
- [ ] T03 El envío de nv-41 respeta `canales_activos` (salvo `reservas`); exportación ARCO con los campos nuevos.
- [ ] T04 Mapa `filtros` en `moderacion_reglas`; `PATCH`/`DELETE /admin/moderacion/reglas/{id}` con `If-Match`; `datos_personales` no se desactiva; `GET /reglas` con `filtros`.
- [ ] T05 `Moderador` decide con umbrales por filtro y salta los inactivos.

## Fase 2 — Cierre
- [ ] T06 Pruebas, `docs/endpoints.md` y `Memoria.md`.
- [ ] T07 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
