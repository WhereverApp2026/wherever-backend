# gates.md — NV-43 Cola de revisión y reglas de moderación

Evidencia local del 2026-11-13 (rama `nv-43`). Integration, Security, Database y Contract las corre el CI del PR (`AGENTS.md` §4). Sin firma.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (en Docker) | ✅ local |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; el log del destino solo lleva tipo, id y error | ✅ local |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | No aplica: sin Terraform ni pipeline | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (106 rutas)` en Docker | ✅ local |
| Build y vet | Sin errores | `go build ./...` y `go vet ./...` en `golang:1.27.3` | ✅ local |
| Lint | 0 hallazgos | `golangci-lint` v2.15.0: `0 issues.` | ✅ local |
| Unit | Verde | `go test -count=1 ./internal/...` (incluye `TestDTO_Validacion` y cuarentena 39 en `TestUmbrales_ReglasInvalidas`) | ✅ local |
| Integration | `tests/integration/moderacion_test.go` | CI del PR | pendiente CI |
| Security | `tests/security/moderacion_security_test.go` (401/403 en las seis rutas) | CI del PR | pendiente CI |
| Database y Contract | Sin regresiones | CI del PR | pendiente CI |
| Cobertura | Mínimo 45 % | CI del PR | pendiente CI |
| Semgrep | 0 hallazgos | CI del PR | pendiente CI |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (63 specs)`; entrada «NV-43» | ✅ local |
