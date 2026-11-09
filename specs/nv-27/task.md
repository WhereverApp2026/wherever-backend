# task.md — NV-27 Tope de gasto y caché de la IA

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-27`.

## Fase 1 — Implementación
- [ ] T02 Tope de gasto mensual (IA_TOPE_USD_MES, 5 por defecto): se estima el costo por…
- [ ] T03 Caché de respuestas por huella de la entrada normalizada, 1 hora de vida.

## Fase 2 — Cierre
- [ ] T04 Pruebas, documentos y `Memoria.md`.
- [ ] T05 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
