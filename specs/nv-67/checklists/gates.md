# gates.md — NV-67 Reportes de información incorrecta de un negocio

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git | `critic-guardrails: PASS` (2026-11-15) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | el módulo no registra el detalle ni a quien reporta | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados | sin cambios; índices nuevos por `scripts/indexes` | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (238 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `golangci-lint` v2.15.0: 0 issues | ✅ |
| Unit | Verde | `service_test.go` (DTO, `otro` sin detalle, moderación) | ✅ |
| Integration | `reportes_negocio_test.go` | local en verde | ✅ local, CI pendiente |
| Security | `reportes_negocio_security_test.go` | local en verde | ✅ local, CI pendiente |
| Database | los cuatro índices y `ux_reporte_abierto` parcial | `tests/database` local en verde | ✅ local, CI pendiente |
| Contract | `reportes_negocio_contract_test.go` y exportación ARCO | suite completa local en verde | ✅ local, CI pendiente |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; sección NV-67 en `Memoria.md` | ✅ |
| Criterio | Un abierto por persona y negocio, 10 al día; solo el administrador resuelve | `TestReportesNegocio_*`, `TestReportesNegocioSecurity_RolesYModeracion` | ✅ |
