# gates.md — NV-64 Reporte de errores de la app y búsqueda por código de amigo

Evidencia local del 2026-11-13 (rama `nv-64`, en `golang:1.27.3`). Integration, Security, Database y Contract las corre el CI del PR (`AGENTS.md` §4). Sin firma.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` | ✅ local |
| critic B2 | Sin tokens ni datos personales en logs ni en `errores_app` | `critic-guardrails: PASS`; `errores.Scrub` antes de guardar; sin campos de log nuevos | ✅ local |
| critic B3 | Cambios de infraestructura o pipeline aprobados | No aplica | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (109 rutas)` | ✅ local |
| Build y vet | Sin errores | `go build ./...` y `go vet ./...` | ✅ local |
| Lint | 0 hallazgos | `golangci-lint` v2.15.0: `0 issues.` | ✅ local |
| Unit | Verde | `go test -count=1 ./internal/... ./cmd/...` | ✅ local |
| Integration | `errores_test.go`, `personas_codigo_test.go`, `TestPresupuestoRedis_VentanasPorUsuarioNV64` | CI del PR | pendiente CI |
| Security | `errores_security_test.go` | CI del PR | pendiente CI |
| Database y Contract | índices y TTL de `errores_app`; `errores_contract_test.go` (capturas a mano) | CI del PR | pendiente CI |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | CI del PR | pendiente CI |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (63 specs)`; entrada «NV-64» | ✅ local |
