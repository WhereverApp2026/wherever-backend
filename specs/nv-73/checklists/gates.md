# gates.md — NV-73 Mensajes del negocio en una reserva y recordatorio de 2 h

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git | `critic-guardrails` PASS (local, 2026-11-16) | ✅ |
| critic B2 | Ningún log ni aviso lleva el texto del mensaje | `reservas_mensajes_security_test.go` | CI del PR |
| critic B3 | Cambios de infraestructura o pipeline aprobados | sin cambios; índices por `scripts/indexes` | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (249 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `go build`, `go vet`, `golangci-lint` v2.15.0 `0 issues` en Docker `golang:1.27.3` | ✅ |
| Unit | Verde | `internal/reservas/mensajes_test.go` (DTO, estados, ventana, `Retry-After`), `internal/notificaciones` (canal y push) | ✅ |
| Integration | `reservas_mensajes_test.go`, `reservas_recordatorio_test.go` | CI del PR | CI del PR |
| Security | `reservas_mensajes_security_test.go` + fila nueva en `reservas_security_test.go` | CI del PR | CI del PR |
| Database | `ix_mensajes_reserva`, `ix_mensajes_usuario`, `ttl_mensajes_reserva` | `tests/database/indexes_test.go`, CI del PR | CI del PR |
| Contract | `reservas_mensajes_contract_test.go` (`testdata/reservas/mensaje_201.json`, `detalle_con_mensajes_200.json`) | CI del PR | CI del PR |
| Redis | Mensaje = escritura sin política; recordatorio sin Redis | `TestPresupuestoRedis_MensajesReservaNV73`, CI del PR | CI del PR |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | CI del PR | CI del PR |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (76 specs)` (spec en Borrador con evidencia) | ✅ |
