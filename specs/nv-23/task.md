# task.md — NV-23 Filtros duros y puntuación de recomendaciones

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-23`.

## Fase 1 — Implementación
- [ ] T02 Filtros duros (nunca se recomienda lo que los incumple): negocio no aprobado, cerrado…
- [ ] T03 Puntuación ponderada de 0 a 100 por presupuesto, distancia, horario, clima, gustos y…
- [ ] T04 Cada recomendación incluye razones[] (los factores que más sumaron) y por_que, una…

## Fase 2 — Cierre
- [ ] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
