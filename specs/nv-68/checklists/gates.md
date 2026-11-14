# gates.md — NV-68 Extras de planes: alternativas, cambiar parada, populares y paquete sin conexión

Spec en borrador: sin ejecutar. Integration, Security, Database y Contract las corre el CI del PR (`AGENTS.md` §4). Sin firma.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | — | pendiente |
| critic B2 | Sin tokens, dueños ni ubicaciones exactas en logs ni respuestas públicas | — | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados | No aplica | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | — | pendiente |
| Build y vet | Sin errores | — | pendiente |
| Lint | 0 hallazgos | — | pendiente |
| Unit | Verde | — | pendiente |
| Integration | `planes_alternativas_test.go`, `planes_populares_test.go`, `planes_paquete_test.go`, presupuesto de Redis | — | pendiente CI |
| Security | `planes_extras_security_test.go` | — | pendiente CI |
| Database y Contract | `ix_populares`; `planes_extras_contract_test.go` | — | pendiente CI |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | — | pendiente CI |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `check_sdd` OK al escribir la spec | pendiente |
