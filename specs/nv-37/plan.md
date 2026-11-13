# plan.md — NV-37 Grupos e integrantes

1. Mapeo
- Crear: `internal/grupos/` (`model.go`, `repository.go`, `service.go`, `dto.go`, `handler.go`, `privacy.go`, `dto_test.go`); `tests/integration/grupos_test.go`; `tests/security/grupos_security_test.go`.
- Modificar: `internal/planes/service.go` (`Members`, `WithMembers`, `getOwned` para editar) y `repository.go` (`FindActive`, `ActiveIDs`); `cmd/api/main.go`, `internal/testutil/authtest/authtest.go` y `scripts/routes/main.go` (módulo antes que planes); `internal/indexes/indexes.go`; `tests/database/indexes_test.go`; `tests/contract/testdata/usuarios/exportacion_descargar_200.json` (`grupos`); `docs/endpoints.md`; `docs/rate-limit-matrix.md`; `Memoria.md`.
- Fuera de esta spec (drift corregido): enlace de invitación (nv-38), votaciones y su cierre (nv-39), notificaciones (nv-40/41). Sin `internal/votaciones`, `internal/planes/compartir.go` ni cambios en `privacidad/job.go`.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- El tope de 12 se aplica en la misma operación de MongoDB (`ocupados < 12` en el filtro y `$inc`), nunca leyendo antes.
- Un grupo o plan ajeno es 404, igual que uno que no existe.

3. Secuencia
1. Preparación: corregir la spec al código actual.
2. Implementación: repositorio atómico, service, handler, ARCO y lectura del plan por integrantes.
3. Cierre: pruebas, documentos, `Memoria.md`, PR.

4. Verificación
- `go build`, `go vet`, Unit y `golangci-lint` en local; Integration, Security, Database y Contract en el CI del PR; `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
