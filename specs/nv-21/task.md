# task.md — NV-21 Lista, ficha y mapa de lugares

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec (pendiente de firma); rama `nv-21`.
- [x] T01b Corregir el drift de horarios: «minutos de la semana» → `horario_min[]` por día de NV-20 con cruce de medianoche.

## Fase 1 — Implementación
- [x] T02 Lista `GET /lugares` con filtros combinables (categoría, precio máximo, abierto ahora, cerca con radio ≤ 20 km) y orden por distancia, calificación o precio; cursor por distancia con `minDistance` (`lugares_service.go`, `lugares_repository.go`).
- [x] T03 Mapa `GET /lugares/mapa`: hasta 100 pines del área; con más, celdas por zoom con conteo y centro.
- [x] T04 Ficha `GET /lugares/{id}` con horarios y `abierto_ahora` en hora de México (`horario.go`); sin campos de control.

## Fase 2 — Cierre
- [x] T05 Pruebas (`internal/negocios/horario_test.go`, `tests/integration/lugares_test.go`, `tests/security/negocios_security_test.go`, `tests/contract/negocios_contract_test.go`), `docs/endpoints.md` y `Memoria.md`.
- [x] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; evidencia en `gates.md`. PR pendiente.
