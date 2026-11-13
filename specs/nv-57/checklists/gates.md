# gates.md — NV-57 Chat de grupo

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | — | pendiente |
| critic B2 | Sin texto de mensajes, tokens ni datos personales en logs | — | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | — | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | — | pendiente |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | — | pendiente |
| Pruebas | Integration, Security, Database y Contract con servicios reales | CI del PR | pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | — | pendiente |
| Redis | Comandos por mensaje medidos y dentro del 60 % de la cuota | `TestPresupuestoRedis_Chat` | pendiente |
| Privacidad | Solo integrantes confirmados ven el chat; ARCO vacía y exporta solo lo propio | `tests/integration/chat_test.go` | pendiente |
| Idempotencia | Mismo `id_cliente` = un documento | `TestChat_Concurrencia_MismoIdCliente` | pendiente |
