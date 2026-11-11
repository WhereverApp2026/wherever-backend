# gates.md — NV-28 Límite por usuario y protección de datos en la IA

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-11) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; la guardia solo registra `parte` y el error | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | No aplica: sin cambios de infraestructura ni pipeline | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (43 rutas)`; sin endpoints nuevos | ✅ |
| Build y vet | `go build ./...` y `go vet ./...` sin errores | Docker `golang:1.27.3` | ✅ |
| Unit | `internal/...` y `cmd/...` verdes (`-count=1`) | `internal/ia/limite_test.go`, `esquema_test.go`, `config_test.go` | ✅ |
| Lint | `golangci-lint` v2.15.0 | `0 issues.` | ✅ |
| Pruebas con contenedor | Integration (`ia_limite_test.go`, 8 + presupuesto), Security (`ia_security_test.go`, 4), Database y Contract | CI del PR (AGENTS.md §4) | pendiente CI |
| Cobertura | Mínimo 45 % | CI del PR | pendiente CI |
| Semgrep | 0 hallazgos | CI del PR | pendiente CI |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)`; sección NV-28 en `Memoria.md` | ✅ |
| Sin datos personales | Ninguna petición lleva nombre, correo, teléfono ni coordenadas | `TestIA_SinDatosPersonalesHaciaElModelo` (Security, CI) | pendiente CI |
