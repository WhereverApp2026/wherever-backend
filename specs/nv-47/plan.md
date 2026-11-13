# plan.md — NV-47 Subida de acciones hechas sin conexión

1. Mapeo
- Crear: `internal/sincronizacion/acciones.go` (registro idempotente) y `pendientes.go` (aplicar el lote); `tests/integration/pendientes_test.go`.
- Modificar: `internal/sincronizacion/dto.go` (`Action`, `PendingRequest`, `ActionResult`), `handler.go` (`WithPending`, límite) y `repository.go` (ARCO también borra `sync_acciones`); `cmd/api/main.go`; `internal/testutil/authtest/authtest.go`; `internal/indexes/indexes.go`; `tests/database/indexes_test.go`; `docs/endpoints.md`; `docs/rate-limit-matrix.md`; `Memoria.md`.
- Sin cambios: `planes` y `account` (se reutilizan tal cual), pipeline e infraestructura.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Cada acción se reclama antes de aplicarse; nada se sobrescribe con una versión vieja.

3. Secuencia
1. Preparación: corregir la spec (413 → 422, conflicto de ajustes por versión).
2. Implementación: registro idempotente, aplicar con los services de la API, resultados por acción.
3. Cierre: pruebas, documentos, `Memoria.md`, PR.

4. Verificación
- `go build`, `go vet`, Unit y `golangci-lint` en local; Integration, Security, Database y Contract en el CI del PR; `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
