# plan.md — NV-34 Nombre de usuario y privacidad de búsqueda

1. Mapeo
- Crear: `internal/shared/validation/username_test.go`.
- Modificar: `internal/usuarios` (campo, índice `ux_nombre_usuario`, privacidad), `internal/usuarios/account` (DTO y service), `internal/shared/validation/registry.go` (regla `username`), pruebas de Integration, Database y Contract, `docs/endpoints.md`, `Memoria.md`.
- Fuera (drift corregido): `internal/amistades` y `internal/personas` son de nv-35/36; sin colección nueva, así que el purger ARCO y `docs/rate-limit-matrix.md` no cambian.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- El teléfono solo se guarda por HMAC (nv-05); la consulta por teléfono llega en nv-35.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
