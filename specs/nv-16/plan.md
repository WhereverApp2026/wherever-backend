# plan.md — NV-16 Versión de la API y versión mínima de la app

1. Mapeo
- Crear: `internal/shared/paginacion/*`, `internal/shared/cache/*`, `internal/platform/versionado/*`, `internal/configuracion/*`, `internal/consentimientos/*`, `docs/convenciones-api.md`, pruebas en `tests/`.
- Modificar: `cmd/api` (middleware en el pipeline, después de la IP del cliente), `internal/platform/config`, `.env.example`, `internal/usuarios` (campo `consentimiento_vigente`), `docs/endpoints.md`, `docs/rate-limit-matrix.md`, `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- `X-App-Version` solo bloquea con un semver válido y menor; un valor basura se ignora.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
