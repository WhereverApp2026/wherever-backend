# gates.md — NV-59 Reservas y disponibilidad

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `scripts/critic-guardrails.sh` PASS (local) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | la agenda no lleva correo ni teléfono (`TestReservas_AgendaEscanearYBloqueos`); los logs no llevan ids ni textos de Mongo/Redis | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin variables ni infraestructura nuevas | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (166 rutas)` en Docker | ✅ |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | build, vet y Unit verdes; `golangci-lint` v2.15.0: 0 issues (Docker) | ✅ |
| Pruebas | Integration, Security, Database y Contract con servicios reales | local en Docker: reservas y presupuesto (Integration), reservas (Security), Database y Contract completas verdes; suites completas en el CI del PR | ✅ local / CI |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; `Memoria.md` con la sección nv-59 | ✅ |
| Sin sobreventa | Un solo ganador por el último lugar | `TestReservas_Concurrencia_UnSoloGanador`: 20 a la vez, 3 rondas, 1 ganador por ronda (local) | ✅ |
| Idempotencia | `Idempotency-Key` repetida no crea otra reserva | `TestReservas_DisponibilidadCrearEIdempotencia` (200, un documento) (local) | ✅ |
