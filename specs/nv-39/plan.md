# plan.md — NV-39 Votaciones

1. Mapeo
- Crear: `internal/votaciones/` (`model.go`, `repository.go`, `service.go`, `dto.go`, `handler.go`, `privacy.go`, `closer.go`, `model_test.go`); `tests/integration/votaciones_test.go`; `tests/security/votaciones_security_test.go`.
- Modificar: `internal/planes/service.go` (`ReplaceBusiness`); `cmd/api/main.go` (módulo antes que grupos, ARCO y `startVotes`); `internal/testutil/authtest/authtest.go` (`Env.Votaciones`); `scripts/routes/main.go`; `internal/indexes/indexes.go`; `tests/database/indexes_test.go`; `tests/contract/testdata/usuarios/exportacion_descargar_200.json`; `docs/endpoints.md`; `Memoria.md`.
- Sin cambios: `internal/privacidad/job.go` (el cierre no es nocturno) y Redis.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Cerrar es una sola operación condicionada (`estado` + `votos_version`); nunca se cuenta y se escribe sin condición.
- La respuesta no dice quién votó qué.

3. Secuencia
1. Preparación: corregir la spec (dependencias reales, cierre sin Redis).
2. Implementación: votar, cerrar condicionado, aplicar con constancia, trabajador por minuto.
3. Cierre: pruebas, documentos, `Memoria.md`, PR.

4. Verificación
- `go build`, `go vet`, Unit y `golangci-lint` en local; Integration, Security, Database y Contract en el CI del PR; `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
