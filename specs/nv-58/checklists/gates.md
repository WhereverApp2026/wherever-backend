# gates.md — NV-58 La Vaca: gastos compartidos del grupo

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | — | pendiente |
| critic B2 | Sin tokens ni datos personales en logs | — | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin variables ni infraestructura nuevas | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints` con las 7 rutas | pendiente |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | — | pendiente |
| Pruebas | Integration, Security, Database y Contract con servicios reales | CI del PR | pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | spec en borrador: `SDD-OK` | pendiente |
| Cuadre | La suma de saldos es 0 y hay a lo más n − 1 transferencias | `corte_test.go` (propiedad) | pendiente |
| Un solo corte | Altas simultáneas dejan una `version_corte` | `TestVaca_Concurrencia_UnSoloCorte` (CI) | pendiente |
