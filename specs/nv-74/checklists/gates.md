# gates.md — NV-74 Estadísticas y reporte mensual del negocio

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git | `critic-guardrails` | pendiente |
| critic B2 | Estadísticas y PDF sin datos de clientes; categoría con ≥ 5 negocios | `estadisticas_security_test.go` | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados | sin cambios; índices por `scripts/indexes` | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints` | pendiente |
| Build, vet y lint | Sin errores; 0 hallazgos | `golangci-lint` | pendiente |
| Unit | Verde | `internal/estadisticas` y `pdf` | pendiente |
| Integration | `estadisticas_test.go`, `reporte_mensual_test.go` | CI del PR | pendiente |
| Security | `estadisticas_security_test.go` | CI del PR | pendiente |
| Database | `ux_negocio_fecha`, `ux_categoria_mes` | CI del PR | pendiente |
| Contract | `estadisticas_contract_test.go` | CI del PR | pendiente |
| Redis | `GET` = lectura con sesión; trabajo y vistas sin comandos | `TestPresupuestoRedis_EstadisticasNV74` | pendiente |
| Dependencias | Sin librería nueva, o `gopdf` con excepción de nv-00 | `go.mod` | pendiente |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | CI del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK` (spec en borrador) | pendiente |
