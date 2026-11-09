# task.md — NV-21 Lista, ficha y mapa de lugares

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-21`.

## Fase 1 — Implementación
- [ ] T02 Lista GET /lugares con filtros combinables (categoría, precio máximo, abierto…
- [ ] T03 Mapa GET /lugares/mapa: pines del área visible, agrupados por celda cuando hay…
- [ ] T04 Ficha GET /lugares/{id} con datos, horarios y el campo calculado abierto_ahora…

## Fase 2 — Cierre
- [ ] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
