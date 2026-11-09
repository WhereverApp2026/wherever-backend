# plan.md — NV-42 Evaluación de imágenes y texto

1. Mapeo
- Crear: `internal/moderacion/*`, `docs/moderacion.md`, pruebas en `tests/`.
- Modificar: boundary y rol de la instancia (Rekognition), `cmd/api`, `docs/endpoints.md`, `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Falla cerrada: sin resultado, no se publica.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
