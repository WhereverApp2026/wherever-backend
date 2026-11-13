# gates.md — NV-55 Alta de negocio por pasos y revisión del administrador

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | — | pendiente |
| critic B2 | Sin tokens, RFC, CLABE ni documentos en logs | — | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | solo si nv-44 necesita ajustar la política de CloudFront | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | — | pendiente |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | — | pendiente |
| Pruebas | Integration, Security, Database y Contract con servicios reales | CI del PR | pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | spec en Borrador: `SDD-OK` (2026-11-13) | pendiente |
| Cifrado | Nada del expediente en claro en MongoDB ni S3 | `tests/integration/negocios_expediente_test.go` | pendiente |
| Bitácora | Cada lectura del admin registrada; sin registro no hay archivo | `tests/integration/negocios_expediente_test.go` | pendiente |
| Privacidad pública | `/lugares` sin datos del expediente | `tests/security/negocios_alta_security_test.go` | pendiente |
| Retención | INE a 90 días, fiscales a 5 años | `tests/integration/negocios_expediente_test.go` | pendiente |
