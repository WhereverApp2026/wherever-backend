# plan.md — NV-22 Búsqueda y autocompletar

1. Mapeo
- Crear: `internal/busquedas/{repository,service,handler,dto,privacy,fuzzy}.go`, `internal/busquedas/fuzzy_test.go`, `tests/integration/busqueda_test.go`.
- Modificar: `cmd/api/main.go` (módulo y ARCO), `internal/testutil/authtest/authtest.go`, `internal/indexes/indexes.go`, `scripts/routes/main.go`, `docs/endpoints.md`, `tests/database/indexes_test.go`, `tests/contract/testdata/usuarios/exportacion_descargar_200.json`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Los endpoints públicos filtran siempre por `estado = aprobado` en la consulta, no después.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
