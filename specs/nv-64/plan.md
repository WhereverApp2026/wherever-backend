# plan.md — NV-64 Reporte de errores de la app y búsqueda por código de amigo

1. Mapeo
- Crear: `internal/errores/{repository,service,handler,dto}.go` y pruebas de las cinco suites. El armado para pruebas va en `internal/testutil/authtest` (no hizo falta `errorestest`).
- Modificar: `internal/platform/logger` (exportar `RedactText`), `internal/shared/validation` (`modulo`, `semver`, `traceid`), `internal/personas` (handler, service y DTO de la ruta por código), `internal/usuarios` (`FindByFriendCode`, `ValidFriendCode`), `internal/amistades` (`Relation`), `internal/indexes` (los índices de `scripts/indexes` y `/ready`), `cmd/api` y `scripts/routes`, `docs/endpoints.md` y `docs/rate-limit-matrix.md`.
- Sin cambios: índices de `usuarios` (`ux_codigo_amigo` ya existe), variables.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs ni en `errores_app`, rol validado en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Cada política nueva de Redis se mide con `TestPresupuestoRedis_*`.

3. Secuencia
1. Preparación: índices de `errores_app` y `RedactText`.
2. Implementación: reporte y lista de errores; ruta por código de amigo.
3. Cierre: pruebas, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
