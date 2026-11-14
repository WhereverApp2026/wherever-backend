# gates.md — NV-56 Panel del negocio: perfil, horarios, pausa, menú y reservas

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `scripts/critic-guardrails.sh` → PASS (2026-11-13) | ok |
| critic B2 | Sin tokens ni datos personales en logs | logs del panel solo con `evento`; critic PASS | ok |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin cambios de infraestructura ni pipeline | ok |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (154 rutas)` en Docker | ok |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | build y vet limpios; Unit `./internal/... ./cmd/...` verde; `golangci-lint` v2.15.0: 0 issues (Docker `golang:1.27.3`) | ok |
| Pruebas | Integration, Security, Database y Contract con servicios reales | CI del PR | pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (65 specs)`; `Memoria.md` con la sección nv-56 | ok |
| Pausa | Oculta en todo lo público y observadores avisados | `tests/integration/negocios_panel_test.go` (`TestNegociosPanel_PausaObservadorYReanudacion`) | CI |
| Contrato con nv-59 | `reservas_config` con los campos que lee nv-59 | `tests/contract/testdata/negocios/reservas_config_200.json` | CI |
| Privacidad | El panel no expone el expediente | `tests/security/negocios_panel_security_test.go` | CI |
