# plan.md — NV-34 Nombre de usuario y privacidad de búsqueda

1. Mapeo
- Crear: `internal/amistades/*`, `internal/personas/*`, pruebas en `tests/`.
- Modificar: `internal/usuarios` (campo, privacidad), `internal/privacidad/purger.go`, `cmd/api`, `docs/endpoints.md`, `docs/rate-limit-matrix.md`, `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- El teléfono solo se consulta por HMAC.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
