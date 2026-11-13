# gates.md — NV-59 Reservas y disponibilidad

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | — | pendiente |
| critic B2 | Sin tokens ni datos personales en logs | la agenda no lleva correo ni teléfono | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin variables ni infraestructura nuevas | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints` con las 12 rutas | pendiente |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | — | pendiente |
| Pruebas | Integration, Security, Database y Contract con servicios reales | CI del PR | pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | spec en borrador: `SDD-OK` | pendiente |
| Sin sobreventa | Un solo ganador por el último lugar | `TestReservas_Concurrencia_UnSoloGanador` (CI) | pendiente |
| Idempotencia | `Idempotency-Key` repetida no crea otra reserva | `tests/integration/reservas_test.go` (CI) | pendiente |
