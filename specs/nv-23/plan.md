# plan.md — NV-23 Filtros duros y puntuación de recomendaciones

1. Mapeo
- Crear: `internal/recomendador/*`, `internal/clima/*`, `internal/inicio/*`, `configs/pesos_recomendador.json`, pruebas en `tests/`.
- Modificar: `internal/usuarios` (lectura de gustos y preferencias, solo lectura), `cmd/api`, `.env.example`, `docs/endpoints.md`, `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Los filtros duros se aplican en la consulta, no después de puntuar.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
