# gates.md — NV-58 La Vaca: gastos compartidos del grupo

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-13) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; el service no registra nada, `errStore` no repite textos de MongoDB ni Redis | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin variables ni infraestructura nuevas | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (118 rutas)` con las 7 de la Vaca | ✅ |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | build y vet locales; Unit y `golangci-lint` v2.15.0 en Docker `golang:1.27.3`: verde, `0 issues` | ✅ |
| Pruebas | Integration, Security, Database y Contract con servicios reales | `tests/integration/vaca_test.go`, `tests/security/vaca_security_test.go`, `tests/database/indexes_test.go`, fixture de exportación: CI del PR | pendiente (CI) |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente (CI) |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (65 specs)`; sección «NV-58 — La Vaca» en `Memoria.md` | ✅ |
| Cuadre | La suma de saldos es 0 y hay a lo más n − 1 transferencias | `TestCompute_Propiedad` (300 rondas, 2 a 12 personas, con deudas pagadas) | ✅ |
| Un solo corte | Altas simultáneas dejan una `version_corte` | `TestVaca_Concurrencia_UnSoloCorte` (CI) | pendiente (CI) |
