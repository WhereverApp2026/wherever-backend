# plan.md — NV-50 Embeddings de negocios

1. Mapeo
- Crear: `internal/embeddings/*`, `cmd/recalcular-embeddings`, `docs/ml-evaluacion.md`, pruebas en `tests/`.
- Modificar: `internal/negocios` (campo y recálculo), `internal/recomendador` (factor), `internal/asistente` (candidatos), boundary y rol, `docs/endpoints.md`, `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- El texto vectorizado es solo público.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
