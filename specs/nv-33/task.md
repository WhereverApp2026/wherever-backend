# task.md — NV-33 Respaldo, cancelación y «otra idea»

## Fase 0 — Preparación
- [x] T01 Rama `nv-33` desde `origin/nv-32` con `origin/nv-28` fusionada (trae nv-27 y nv-28); spec conciliada. Falta la firma.

## Fase 1 — Implementación
- [x] T02 Respaldo (US-039): `internal/asistente/respaldo.go` y `service.go`; cada falla de la IA, el tope y los límites terminan en el motor de reglas con `fuente: reglas`.
- [x] T03 Cancelar (US-040): plazo de 7 s para las dos llamadas; un contexto cancelado corta la IA y no arma el respaldo.
- [x] T04 `POST /planes/generar/otra-idea` con `excluir` (1 a 30 ids) sobre `recomendador.Contexto.Excluidos`.
- [ ] T05 Medición con IA real: 20 frases, p50/p95, descartes, 429 y tokens por plan (necesita la llave de SSM).

## Fase 2 — Cierre
- [x] T06 Pruebas (Unit, Integration, Security), `docs/endpoints.md`, `docs/rate-limit-matrix.md`, `.env.example` y `Memoria.md`.
- [x] T07 `go build`, `go vet`, Unit, `golangci-lint`, las cuatro suites con contenedor en local, `critic`, `check_sdd`, `check_endpoints`; PR contra `nv-32`; evidencia en `gates.md`.