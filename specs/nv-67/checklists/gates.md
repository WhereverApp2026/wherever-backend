# gates.md — NV-67 Reportes de información incorrecta de un negocio

Sin ejecutar: se llena al implementar. Integration, Security, Database y Contract las corre el CI del PR (`AGENTS.md` §4).

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | — | pendiente |
| critic B2 | Sin tokens ni datos personales en logs | — | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados | No aplica | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | — | pendiente |
| Build y vet | Sin errores | — | pendiente |
| Lint | 0 hallazgos | — | pendiente |
| Unit | Verde | — | pendiente |
| Integration | `reportes_negocio_test.go`, `privacidad_reportes_test.go` | — | pendiente |
| Security | `reportes_negocio_security_test.go` | — | pendiente |
| Database y Contract | índices de `reportes_negocio`; `reportes_negocio_contract_test.go` | — | pendiente |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | — | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | — | pendiente |
