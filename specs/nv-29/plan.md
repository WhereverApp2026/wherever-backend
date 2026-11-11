# plan.md — NV-29 Modelo de plan, paradas y cálculo de traslados

1. Mapeo
- Crear: `internal/planes/` (`model.go`, `recalculo.go`, `calculadora.go`, `repository.go`, `privacy.go`, `recalculo_test.go`), `internal/negocios/paradas.go` (`FindVisibleMany`) y `tests/integration/planes_test.go`.
- Modificar: `internal/indexes` (índices de `planes`), `cmd/api/main.go` e `internal/testutil/authtest` (ARCO de planes), `tests/database/indexes_test.go`, `Memoria.md`.
- Sin cambios: `docs/endpoints.md` y `scripts/routes` (sin endpoints), `.env.example` (velocidades en código).

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs.
- Toda consulta de planes filtra por dueño y estado en la consulta.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- Local: `go build`, `go vet`, Unit y `golangci-lint`. CI: Integration, Security, Database y Contract, cobertura ≥ 45 %, Semgrep en 0. `critic`, `check_endpoints` y `check_sdd`.
