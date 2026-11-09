# task.md — NV-33 Respaldo, cancelación y «otra idea»

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-33`.

## Fase 1 — Implementación
- [ ] T02 Respaldo (US-039): si la IA tarda más de 8 s, falla, se agotó el tope o el límite del…
- [ ] T03 Cancelar (US-040): si la app corta la conexión, la API cancela la llamada al modelo.
- [ ] T04 POST /planes/generar/otra-idea produce un plan distinto excluyendo los lugares ya…
- [ ] T05 Respuesta promedio por debajo de 5 s con la IA y por debajo de 1 s con el respaldo.

## Fase 2 — Cierre
- [ ] T06 Pruebas, documentos y `Memoria.md`.
- [ ] T07 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
