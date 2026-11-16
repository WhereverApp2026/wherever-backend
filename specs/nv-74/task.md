# task.md — NV-74 Estadísticas y reporte mensual del negocio

## Fase 0 — Preparación
- [ ] T01 Spec revisada y aprobada (incluye la decisión del PDF); rama de trabajo contra `main`.
- [ ] T02 `estadisticas_diarias` (`ux_negocio_fecha`) y `estadisticas_categoria` en `scripts/indexes` y `/ready`.

## Fase 1 — Implementación
- [ ] T03 `vistas.Contador`: `$inc` de `vistas_ficha` y `origen` al escribir una vista.
- [ ] T04 Trabajo cada hora (hoy y ayer, idempotente), nocturno (categorías y limpieza de 400 días) y relleno de 90 días.
- [ ] T05 GET /negocios/{id}/estadisticas con rango, agrupación, periodo anterior, frase y avanzadas premium.
- [ ] T06 Escritor de PDF y GET /negocios/{id}/reportes/mensual.
- [ ] T07 Baja del negocio borra sus estadísticas.

## Fase 2 — Cierre
- [ ] T08 Pruebas, `docs/endpoints.md`, estimación de Redis y `Memoria.md`.
- [ ] T09 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; suites con contenedor en el CI del PR; evidencia en `gates.md`.
