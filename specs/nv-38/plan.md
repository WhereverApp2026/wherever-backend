# plan.md — NV-38 Enlace de invitación y vista pública

1. Mapeo
- Crear: `internal/grupos/enlace.go`; `tests/integration/enlace_test.go`; `tests/security/enlace_security_test.go`.
- Modificar: `internal/grupos/model.go` (`codigo_hash`, `codigo_vence`), `repository.go` (`ux_codigo_hash`, `SetCode`, `ClearCode`, `FindByCode`, `Join`), `service.go` (`groupForPlan`, `Plans.FindActive`), `dto.go` (`CodeRequest`, `ShareResponse`, `PublicPlan`), `handler.go` (rutas y límites) y `dto_test.go`; `internal/negocios/paradas.go` (`FindCards`); `internal/http/middleware/compress.go` y su prueba; `cmd/api/main.go` y `internal/testutil/authtest/authtest.go` (`WithShare`); `tests/database/indexes_test.go`; `docs/endpoints.md`; `docs/rate-limit-matrix.md`; `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- El código del enlace nunca se guarda ni se registra en claro: va en el cuerpo y en el fragmento, nunca en la ruta.

3. Secuencia
1. Preparación: corregir la spec (código fuera de la ruta, revocar).
2. Implementación: código y hash, vista pública, unirse atómico, límites.
3. Cierre: pruebas, documentos, `Memoria.md`, PR.

4. Verificación
- `go build`, `go vet`, Unit y `golangci-lint` en local; Integration, Security, Database y Contract en el CI del PR; `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
