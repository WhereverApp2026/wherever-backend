# gates.md — NV-56 Panel del negocio: perfil, horarios, pausa, menú y reservas

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | — | pendiente |
| critic B2 | Sin tokens ni datos personales en logs | — | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin cambios de infraestructura ni pipeline | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | — | pendiente |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | — | pendiente |
| Pruebas | Integration, Security, Database y Contract con servicios reales | CI del PR | pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | spec en Borrador: `SDD-OK` (2026-11-13) | pendiente |
| Pausa | Oculta en todo lo público y observadores avisados | `tests/integration/negocios_panel_test.go` | pendiente |
| Contrato con nv-59 | `reservas_config` con los campos que lee nv-59 | `tests/contract` | pendiente |
| Privacidad | El panel no expone el expediente | `tests/security/negocios_panel_security_test.go` | pendiente |
