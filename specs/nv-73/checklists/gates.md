# gates.md — NV-73 Mensajes del negocio en una reserva y recordatorio de 2 h

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git | `critic-guardrails` | pendiente |
| critic B2 | Ningún log ni aviso lleva el texto del mensaje | `reservas_mensajes_security_test.go` | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados | sin cambios; índices por `scripts/indexes` | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints` | pendiente |
| Build, vet y lint | Sin errores; 0 hallazgos | `golangci-lint` | pendiente |
| Unit | Verde | `internal/reservas` (DTO, estados, ventana) | pendiente |
| Integration | `reservas_mensajes_test.go`, `reservas_recordatorio_test.go` | CI del PR | pendiente |
| Security | `reservas_mensajes_security_test.go` | CI del PR | pendiente |
| Database | `ix_mensajes_reserva`, `ix_mensajes_usuario`, TTL | CI del PR | pendiente |
| Contract | `reservas_mensajes_contract_test.go` | CI del PR | pendiente |
| Redis | Mensaje = escritura sin política; recordatorio sin Redis | `TestPresupuestoRedis_MensajesReservaNV73` | pendiente |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | CI del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK` (spec en borrador) | pendiente |
