# task.md — NV-25 Clima e Inicio

## Fase 0 — Preparación
- [x] T01 Corregir el drift de spec y plan (paquetes reales, contratos, caché sin Redis); rama desde `main`. Falta la firma.

## Fase 1 — Implementación
- [x] T02 `GET /inicio` arma la pantalla en una llamada: saludo y chips por franja, clima y 5 recomendaciones (`internal/inicio`, `recomendaciones.Service.ForHome`).
- [x] T03 `GET /contexto/clima` con Open-Meteo, plazo de 3 s y caché de 30 min (`internal/clima`); factor clima en `internal/recomendador`.

## Fase 2 — Cierre
- [x] T04 Pruebas (`Unit/Clima`, `Unit/Inicio`, `Integration/Clima`, `Integration/Inicio`), `docs/endpoints.md`, `scripts/routes` y `Memoria.md`.
- [ ] T05 Local verde (`go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints`); PR contra `main`; suites con contenedor en el CI; evidencia en `gates.md`.
