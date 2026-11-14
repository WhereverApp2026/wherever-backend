# task.md — NV-61 Cupones

## Fase 0 — Preparación
- [x] T01 Spec revisada (decisiones del revisor) y filas de J6 en `docs/endpoints.md`.
- [x] T02 Índices de `cupones`, `cupones_usuario` y `cupones_metricas` en `internal/indexes.All()`.

## Fase 1 — Implementación
- [x] T03 Crear, listar, editar (If-Match, solo vigencia/límite/estado con canjes) y borrar (409 con canjes); relámpago de 3 h.
- [x] T04 `GET /cupones/cerca` y `GET /cupones/{id}` públicos, solo activos, vigentes y de negocios visibles; vistas en el detalle.
- [x] T05 Cartera: guardar (409 duplicado), filtros, prefijo y quitar.
- [x] T06 Canjear (código de 6, 5 min, idempotente) y validar atómico con reintento del mismo negocio y `limite_usos`.
- [x] T07 Estadísticas por día y trabajo de activar/vencer con `cupones_activos` y caché de la ficha.
- [x] T08 Sugerir título con la IA de nv-26 a nv-28.
- [x] T09 Límites por usuario, `Purger` y `Exporter`, gancho de pausa del negocio.

## Fase 2 — Cierre
- [x] T10 Pruebas de las cinco suites, incluidas las de concurrencia (Unit en local; Integration, Security, Database y Contract las corre el CI del PR).
- [x] T11 Gates en `checklists/gates.md` y `Memoria.md`.
