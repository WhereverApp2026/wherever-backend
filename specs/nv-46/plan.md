# plan.md — NV-46 Descarga de cambios

1. Mapeo
- Crear: `internal/sincronizacion/` (`repository.go`, `service.go`, `dto.go`, `handler.go`); `tests/integration/sync_test.go`.
- Modificar: `internal/planes/model.go` (`sync_seq`), `repository.go` (`ix_dueno_sync`, `ChangedAfter`, `sync_seq` al editar y borrar) y `service.go` (`Sync`, `WithSync`, `stamp`); `internal/usuarios/account/dto.go` (`SettingsOf`); `cmd/api/main.go` (módulo, ARCO y `startVotes`); `internal/testutil/authtest/authtest.go`; `scripts/routes/main.go`; `internal/indexes/indexes.go`; `tests/database/indexes_test.go`; `docs/endpoints.md`; `Memoria.md`.
- Sin cambios: Redis, pipeline e infraestructura.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Nunca se usa la hora del teléfono; el cursor va firmado y atado al usuario.

3. Secuencia
1. Preparación: corregir la spec (dependencias y ajustes por versión).
2. Implementación: contador, borrados, numeración de planes, descarga paginada.
3. Cierre: pruebas, documentos, `Memoria.md`, PR.

4. Verificación
- `go build`, `go vet`, Unit y `golangci-lint` en local; Integration, Security, Database y Contract en el CI del PR; `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
