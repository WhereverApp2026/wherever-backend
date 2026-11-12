# gates.md — NV-35 Buscar personas

Evidencia local del 2026-11-11 en `golang:1.27.3` con Docker. Integration, Security, Database y Contract los corre el CI del PR (`AGENTS.md` §4).

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; el teléfono solo se consulta por HMAC y nunca se registra | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios de infraestructura ni pipeline | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (59 rutas)` | ✅ |
| Pruebas | Cinco suites verdes con servicios reales | build, vet y Unit `go test ./internal/... ./cmd/...` ok; Integration, Security, Database y Contract en el CI del PR | parcial |
| Lint | `golangci-lint` v2.15.0 | `0 issues.` | ✅ |
| Cobertura | Mínimo 45 % | la corre el CI del PR a `main` | pendiente |
| Semgrep | 0 hallazgos | lo corre el CI del PR a `main` | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)`; sección NV-35 en `Memoria.md` | ✅ |
| Enumeración | Existe o no, misma respuesta | `TestPersonasSecurity_SinDatosPrivadosNiEnumeracion` (mismos bytes) y `TestBuscarPersonas_PorTelefono` | CI |
