# gates.md — NV-24 Recomendaciones y «No me interesa»

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-11) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; el error del motor no lleva el texto de MongoDB | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin cambios de infraestructura ni pipeline | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (46 rutas)` | ✅ |
| Build y lint | `go build`, `go vet`, `gofmt` y `golangci-lint` v2.15.0 | Docker `golang:1.27.3`: sin errores, `0 issues.` | ✅ |
| Pruebas | Unit verde en local; Integration, Security, Database y Contract en el CI | `go test ./internal/...` ok; resto pendiente del CI del PR | pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI | pendiente |
| Semgrep | 0 hallazgos | Semgrep en el CI | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)` | ✅ |
