# gates.md — NV-22 Búsqueda y autocompletar

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `scripts/critic-guardrails.sh` → PASS (2026-11-12) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | critic PASS; el fallo al guardar recientes se registra sin el texto | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin cambios de infraestructura ni pipeline | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `scripts/check_endpoints.sh` → OK | ✅ |
| Build y lint | `go build`, `go vet`, `gofmt`, `golangci-lint` v2.15.0 | Docker `golang:1.27.3`: sin errores; lint 0 issues | ✅ |
| Pruebas | Unit en local; Integration, Security, Database y Contract en el CI del PR | Unit verde (`internal/busquedas`); `tests/integration/busqueda_test.go` en el CI | ⏳ CI |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI | ⏳ CI |
| Semgrep | 0 hallazgos | job `semgrep` del CI | ⏳ CI |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `scripts/check_sdd.sh` → SDD-OK | ✅ |
