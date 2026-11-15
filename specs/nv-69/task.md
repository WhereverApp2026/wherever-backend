# task.md — NV-69 Extras de grupos: editar y disolver, punto de encuentro, modo sorpresa y «Me apunto»

## Fase 0 — Preparación
- [x] T01 Spec revisada, ola decidida (después de NV-68) y drift corregido; rama de trabajo contra `main`.
- [x] T02 Campos nuevos de `grupos` con valores por omisión; `If-Match` en grupos.

## Fase 1 — Implementación
- [x] T03 PATCH y DELETE de grupos (nombre, borrado lógico, saldos de La Vaca).
- [x] T04 Ubicación compartida en `integrantes/yo`, PUT punto, GET sugerido y GET traslados con rangos; límite `grupos_ubicacion`.
- [x] T05 PUT sorpresa y recorte en plan, grupo, paquete, descarga de cambios y votaciones.
- [x] T06 POST y DELETE me-apunto con amistad, `visibilidad` y carrera resuelta por índice único.

## Fase 2 — Cierre
- [x] T07 Pruebas, ARCO, `docs/endpoints.md`, `docs/rate-limit-matrix.md` y `Memoria.md`.
- [x] T08 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
