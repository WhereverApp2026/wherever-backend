# gates.md — NV-61 Cupones

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `scripts/critic-guardrails.sh` → PASS (local, 2026-11-14) | ✅ |
| critic B2 | Sin tokens, códigos de canje ni datos personales en logs | critic PASS; los avisos de cupones llevan `evento` y el error, nunca el código; la exportación omite `codigo_canje` (`TestCupones_ARCO`, CI) | ✅ local |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin variables, Terraform ni pipeline | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (186 rutas)` en Docker | ✅ |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | local en Docker `golang:1.27.3`: build, vet y Unit verdes; `golangci-lint` v2.15.0 `0 issues` | ✅ |
| Pruebas | Integration, Security, Database y Contract con servicios reales | CI del PR (no se corrieron en local, AGENTS.md §4) | pendiente de CI |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente de CI |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente de CI |
| SDD | `check_sdd` OK | `SDD-OK (73 specs)` | ✅ |
| Canje único | Un código se canjea una vez | `TestCupones_Concurrencia_CanjeUnaVez` | pendiente de CI |
| Límite de usos | No se pasa `limite_usos` | `TestCupones_Concurrencia_LimiteUsos` | pendiente de CI |
