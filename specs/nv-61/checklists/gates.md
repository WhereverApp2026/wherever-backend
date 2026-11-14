# gates.md — NV-61 Cupones

CI del PR #97: run 43560822937 (2026-11-14), los 7 jobs en verde (changes, guardrails, semgrep, go-security, image-scan, pipeline, pr-gate).

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `scripts/critic-guardrails.sh` → PASS (local y job `guardrails`) | ✅ |
| critic B2 | Sin tokens, códigos de canje ni datos personales en logs | critic PASS; los avisos de cupones llevan `evento` y el error, nunca el código; la exportación omite `codigo_canje` (`TestCupones_ARCO`, job `pipeline`) | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin variables, Terraform ni pipeline | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (186 rutas)` en Docker y job `guardrails` | ✅ |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | local en Docker `golang:1.27.3` (`golangci-lint` v2.15.0 `0 issues`) y job `pipeline` | ✅ |
| Pruebas | Integration, Security, Database y Contract con servicios reales | job `pipeline` del PR #97 en verde | ✅ |
| Cobertura | Mínimo 45 % | `check_coverage` en el job `pipeline` | ✅ |
| Semgrep | 0 hallazgos | job `semgrep` del PR #97 | ✅ |
| SDD | `check_sdd` OK | `SDD-OK (73 specs)` (local y job `guardrails`) | ✅ |
| Canje único | Un código se canjea una vez | `TestCupones_Concurrencia_CanjeUnaVez` en el job `pipeline` | ✅ |
| Límite de usos | No se pasa `limite_usos` | `TestCupones_Concurrencia_LimiteUsos` en el job `pipeline` | ✅ |
