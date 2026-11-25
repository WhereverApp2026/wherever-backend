# gates.md — NV-93 Lista de cobros del negocio

Evidencia local del 2026-11-25, en Docker `golang:1.27.3`. Integration, Security, Database y Contract las corre el CI del PR.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1–B4 | Sin secretos; rutas en `docs/endpoints.md` | `critic-guardrails: PASS`; `check_endpoints: OK (286 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `go build ./...` y `go vet ./...` sin salida; golangci-lint v2.15.0: `0 issues.` | ✅ |
| Unit | Consulta, rango, totales y fila sin datos de quien pagó | `go test ./internal/... ./cmd/... ./scripts/...` verde: `internal/pagos/cobros_test.go` | ✅ |
| Integration | Varios pagos, cursor, filtros, totales, 422 y ajeno 404 | `TestPagos_CobrosDelNegocio` (CI del PR) | ⏳ CI |
| Security | 401 sin token, 403 sin rol, 404 ajeno | Ruta en `rutasPagos` (`TestPagosSecurity_SesionRolYAjeno`, CI del PR) | ⏳ CI |
| Database | Índice `ix_pago_negocio` | `tests/database/indexes_test.go` (CI del PR) | ⏳ CI |
| Contract | Forma de la lista con totales | `pagos/cobros_200` en `TestContract_PagosCobros` (CI del PR) | ⏳ CI |
| SDD | `check_sdd` OK y `Memoria.md` al día | `SDD-OK`; `Memoria.md` con la sección NV-93 | ✅ |
