# plan.md — NV-30 Guardar, leer, editar y repetir planes

1. Mapeo
- Crear: `internal/planes/dto.go`, `service.go`, `handler.go`, `dto_test.go`; `tests/integration/planes_api_test.go`; `tests/security/planes_security_test.go`.
- Modificar: `internal/planes/repository.go` (`List`, `DeleteIfVersion`), `internal/negocios/horario.go` (`MexicoCity`), `cmd/api/main.go`, `internal/testutil/authtest/authtest.go`, `scripts/routes/main.go`, `docs/endpoints.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Toda consulta de planes filtra por dueño y estado en la consulta.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- Local: `go build`, `go vet`, Unit y `golangci-lint`; CI: Integration, Security, Database y Contract, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
