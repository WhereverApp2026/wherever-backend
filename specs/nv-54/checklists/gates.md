# gates.md — NV-54 Favoritos de negocios y planes

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails.sh` PASS (2026-11-13, Docker) | ok |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails.sh` PASS; `errStore` no lleva el texto de MongoDB | ok |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin cambios de infraestructura ni pipeline | ok |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints.sh` OK (111 rutas) | ok |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | build y vet sin errores; `go test ./internal/favoritos` ok; `golangci-lint` v2.15.0 0 issues | ok |
| Pruebas | Integration, Security, Database y Contract con servicios reales | `tests/integration/favoritos_test.go`, `tests/security/favoritos_security_test.go`, `tests/database/indexes_test.go`, contrato de exportación; CI del PR | CI |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | CI |
| Semgrep | 0 hallazgos | job `semgrep` del PR | CI |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | SDD-OK (63 specs); entrada `## NV-54` | ok |
| Idempotencia | Marcar dos veces deja un documento | `TestFavoritos_MarcarDosVecesDejaUnDocumento` | CI |
| Propiedad | Plan ajeno, negocio pausado o inexistente = 404; otra persona no ve mis favoritos | `TestFavoritos_ObjetivoNoValidoEs404`, `TestFavoritos_IdsYBuscarSinAcentos` | CI |
| ARCO | Exporta `tipo`, `objetivo_id`, `nombre`, `creado_en`; borrar historial no los toca; borrar cuenta sí, solo los suyos | `TestFavoritos_ARCOExportaYBorraConLaCuenta` | CI |
