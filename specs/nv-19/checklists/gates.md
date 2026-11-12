# gates.md — NV-19 Aceptación de términos y aviso de privacidad

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (host, 2026-11-12) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; la ruta nueva no escribe logs | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | No aplica: sin cambios de infraestructura ni pipeline | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (75 rutas)` (Docker `golang:1.27.3`) | ✅ |
| Build/vet/lint | `go build`, `go vet`, `golangci-lint` v2.15.0 | sin errores; lint `0 issues.` | ✅ |
| Unit | `go test ./internal/... ./cmd/...` | verde, incluye `internal/usuarios/account/consents_test.go` | ✅ |
| Pruebas con contenedor | Integration, Security, Database y Contract | CI del PR (`AGENTS.md` §4) | pendiente del CI |
| Cobertura | Mínimo 45 % | CI del PR | pendiente del CI |
| Semgrep | 0 hallazgos | CI del PR | pendiente del CI |
| Redis | Sin comandos nuevos | la ruta no usa Redis; `TestPresupuestoRedis_*` sin cambios | ✅ |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)` | ✅ |
