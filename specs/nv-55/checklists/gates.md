# gates.md — NV-55 Alta de negocio por pasos y revisión del administrador

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails.sh` local (2026-11-13) | OK |
| critic B2 | Sin tokens, RFC, CLABE ni documentos en logs | `critic` local; el logger oculta `clabe`, `rfc`, `razon_social`, `archivo` (`logger_test`) | OK |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios de infraestructura ni pipeline | N/A |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (121 rutas)` en Docker (en Windows la política de aplicaciones bloquea el binario de `scripts/routes`) | OK |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | Docker `golang:1.27.3` y `golangci-lint v2.15.0`: 0 issues, Unit verdes (2026-11-13) | OK |
| Pruebas | Integration, Security, Database y Contract con servicios reales | CI del PR | pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK` y `Memoria.md` (2026-11-13) | OK |
| Cifrado | Nada del expediente en claro en MongoDB ni S3 | `tests/integration/negocios_expediente_test.go`; Unit `TestCifradoDeArchivo_AtadoAlNegocioYTipo` | CI del PR |
| Bitácora | Cada lectura del admin registrada; sin registro no hay archivo | `tests/integration/negocios_expediente_test.go` | CI del PR |
| Privacidad pública | `/lugares` sin datos del expediente | `tests/security/negocios_alta_security_test.go` | CI del PR |
| Retención | INE a 90 días, fiscales a 5 años | `tests/integration/negocios_expediente_test.go` | CI del PR |
