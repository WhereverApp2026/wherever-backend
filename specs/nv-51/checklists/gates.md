# gates.md — NV-51 Búsqueda semántica y evaluación

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-11) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; el log solo dice la causa, la llave de caché es una huella | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin cambios de infraestructura ni pipeline | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (65 rutas)` | ✅ |
| Build y lint | `go build`, `go vet`, `golangci-lint` v2.15.0 | 0 errores, `0 issues` (Docker `golang:1.27.3`) | ✅ |
| Pruebas Unit | `go test ./internal/... ./cmd/... ./scripts/...` | verde en local | ✅ |
| Pruebas con contenedor | Integration (`semantica_test.go`, `recomendador_test.go`), Security, Database, Contract | CI del PR | pendiente |
| Cobertura | Mínimo 45 % | CI del PR | pendiente |
| Semgrep | 0 hallazgos | CI del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)` | ✅ |
| Evaluación | Informe top-5 con y sin semántica | `docs/ml-evaluacion.md`: script y método listos; corrida real pendiente (sin acceso a SSM) | pendiente |
