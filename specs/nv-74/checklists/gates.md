# gates.md — NV-74 Estadísticas y reporte mensual del negocio

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git | `critic-guardrails` en Git Bash: B1 a B3 sin fallas (B4 no corre ahí porque Windows bloquea Go; se verificó en Docker) | ✅ |
| critic B2 | Estadísticas y PDF sin datos de clientes; categoría con ≥ 5 negocios | `estadisticas_security_test.go`, `TestEstadisticas_CategoriaSoloConCincoNegocios` | CI del PR |
| critic B3 | Cambios de infraestructura o pipeline aprobados | sin cambios; índices por `scripts/indexes` | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (251 rutas)` en Docker `golang:1.27.3` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `go build`, `go vet`, `golangci-lint` v2.15.0 `0 issues` en Docker `golang:1.27.3` | ✅ |
| Unit | Verde | `internal/estadisticas` (rango, semanas, cambio, frase, mes del reporte, fuentes) y `internal/estadisticas/pdf` (xref, WinAnsi, salto de página); `go test ./internal/... ./cmd/...` verde | ✅ |
| Integration | `estadisticas_test.go`, `reporte_mensual_test.go` | CI del PR | CI del PR |
| Security | `estadisticas_security_test.go` | CI del PR | CI del PR |
| Database | `ux_negocio_fecha`, `ux_categoria_mes` | `tests/database/indexes_test.go`, CI del PR | CI del PR |
| Contract | `estadisticas_contract_test.go` (`testdata/estadisticas/estadisticas_200.json` y cabeceras del PDF); la exportación ARCO no cambia | CI del PR | CI del PR |
| Redis | `GET` = lectura con sesión; contador y trabajos sin comandos | `TestPresupuestoRedis_EstadisticasNV74`, CI del PR | CI del PR |
| Dependencias | Sin librería nueva | escritor propio en `internal/estadisticas/pdf`; `go.mod` sin cambios | ✅ |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | CI del PR | CI del PR |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (76 specs)` (spec en Borrador con evidencia) | ✅ |
