# task.md — NV-54 Favoritos de negocios y planes

## Fase 0 — Preparación
- [x] T01 Spec revisada y drift corregido; rama de trabajo contra `main`.
- [x] T02 Colección `favoritos` con `ux_favorito`, `ix_lista` e `ix_objetivo` en `scripts/indexes` y `/ready`.

## Fase 1 — Implementación
- [x] T03 PUT /favoritos/{tipo}/{objetivo_id} idempotente; negocio aprobado o plan propio; tope 500 por tipo.
- [x] T04 DELETE /favoritos/{tipo}/{objetivo_id} (204 aunque no exista).
- [x] T05 GET /favoritos?tipo= con cursor firmado, `cupon_hoy`, `abierto_ahora`, `con_cupon` y limpieza de objetivos inexistentes.
- [x] T06 GET /favoritos/ids y GET /favoritos/buscar.
- [x] T07 Purger y Exporter de nv-05.

## Fase 2 — Cierre
- [x] T08 Pruebas, `docs/endpoints.md` y `Memoria.md`.
- [x] T09 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
