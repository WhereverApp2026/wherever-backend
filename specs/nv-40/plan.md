# plan.md — NV-40 Dispositivos y centro de notificaciones

1. Mapeo
- Crear: `internal/notificaciones/` (`model.go`, `repository.go`, `service.go`, `dto.go`, `handler.go`, `privacy.go`, `dto_test.go`); `tests/integration/notificaciones_test.go`; `tests/security/notificaciones_security_test.go`.
- Modificar: `internal/amistades/service.go`, `internal/grupos/service.go` y `internal/votaciones/service.go` (`WithNotifier` y los avisos); `cmd/api/main.go` (módulo, ARCO, `socialDeps.notifier`, `startVotes`); `internal/testutil/authtest/authtest.go`; `scripts/routes/main.go`; `internal/indexes/indexes.go`; `tests/database/indexes_test.go`; `tests/contract/testdata/usuarios/exportacion_descargar_200.json`; `docs/endpoints.md`; `Memoria.md`.
- Sin cambios: Redis, pipeline e infraestructura.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- El token de FCM nunca va en la ruta, en logs ni en la exportación.
- Avisar no cambia la respuesta de la acción original.

3. Secuencia
1. Preparación: corregir la spec (token en el cuerpo).
2. Implementación: centro, dispositivos, `Notifier` y avisos de los módulos.
3. Cierre: pruebas, documentos, `Memoria.md`, PR.

4. Verificación
- `go build`, `go vet`, Unit y `golangci-lint` en local; Integration, Security, Database y Contract en el CI del PR; `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
