# task.md — NV-51 Búsqueda semántica y evaluación

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-51`.

## Fase 1 — Implementación
- [ ] T02 GET /lugares/semantica?q= devuelve los negocios más cercanos en significado a la frase,…
- [ ] T03 El motor de reglas (nv-23) suma un factor afinidad_semantica y el asistente (nv-31) usa…
- [ ] T04 Medición de calidad: un conjunto de 30 frases con los lugares esperados; se reportan…

## Fase 2 — Cierre
- [ ] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
