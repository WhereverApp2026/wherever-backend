# gates.md — NV-69 Extras de grupos: editar y disolver, punto de encuentro, modo sorpresa y «Me apunto»

Spec en borrador: sin ejecutar. Integration, Security, Database y Contract las corre el CI del PR (`AGENTS.md` §4). Sin firma.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | — | pendiente |
| critic B2 | Sin tokens ni coordenadas de integrantes en logs ni respuestas | — | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados | No aplica | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | — | pendiente |
| Build y vet | Sin errores | — | pendiente |
| Lint | 0 hallazgos | — | pendiente |
| Unit | Verde | — | pendiente |
| Integration | `grupos_extras_test.go`, `punto_encuentro_test.go`, `sorpresa_test.go`, `me_apunto_test.go`, presupuesto de Redis | — | pendiente CI |
| Security | `grupos_extras_security_test.go` | — | pendiente CI |
| Database y Contract | sin índices nuevos; `grupos_extras_contract_test.go` | — | pendiente CI |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | — | pendiente CI |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `check_sdd` OK al escribir la spec | pendiente |
