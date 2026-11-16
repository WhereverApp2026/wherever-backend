# plan.md — NV-73 Mensajes del negocio en una reserva y recordatorio de 2 h

1. Mapeo
- Crear: `internal/reservas/mensajes.go` (modelo, DTO, repositorio y service de `mensajes_reserva` y del recordatorio) y su `mensajes_test.go`; ruta en `internal/reservas/handler.go`; pruebas `tests/integration/reservas_mensajes_test.go`, `reservas_recordatorio_test.go`, `tests/security/reservas_mensajes_security_test.go`, `tests/contract/reservas_mensajes_contract_test.go` (+ `testdata/reservas/`).
- Modificar: `internal/reservas/barrido.go` (paso del recordatorio), `model.go` (`recordatorio_en`), `service.go` (cambio de hora borra `recordatorio_en`; detalle con `mensajes`), `privacy.go` (purga y exportación), `internal/notificaciones/model.go` (`KindReservaMensaje`, `KindReservaRecordatorio`) y `push.go` (texto y canal `reservas`), `cmd/api/reservas.go` (`WithNotifier`, `WithNegocios`) y `cmd/api/main.go` (`backgroundNotifier` compartido con votaciones), `internal/testutil/authtest`, `tests/database/indexes_test.go`, `tests/integration/presupuesto_redis_test.go`, `docs/endpoints.md`, `docs/rate-limit-matrix.md` (límite en MongoDB).
- Sin cambios: variables de entorno, políticas de Redis, `ix_recordatorios` (ya existe); `internal/indexes`, `scripts/indexes`, `/ready` y `scripts/routes` ya toman `reservas.Indexes()` y el handler de reservas.

2. Guardarraíles
- Servicios reales en las pruebas; avisos solo con ids; ningún log con `texto`; ARCO completo.
- Sin política nueva de Redis; `TestPresupuestoRedis_MensajesReservaNV73` fija el costo.

3. Secuencia
1. Preparación: colección, índices y tipos de aviso.
2. Implementación: mensaje (POST), lectura en el detalle, recordatorio en el barrido, ARCO.
3. Cierre: pruebas, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, Unit y lint en local; Integration, Security, Database y Contract en el CI del PR; `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
