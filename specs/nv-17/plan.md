# plan.md — NV-17 Paginación por cursor y validadores HTTP

1. Mapeo
- Crear: Ver Diseño de esta spec.
- Modificar: Ver Diseño de esta spec.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- El 304 reutiliza el helper `internal/shared/etag` de nv-16 y solo se aplica a GET públicos de catálogo; una respuesta con datos del usuario lleva `no-store`.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
