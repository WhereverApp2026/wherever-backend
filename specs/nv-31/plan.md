# plan.md — NV-31 Interpretar la frase y armar candidatos

1. Mapeo
- Crear: `internal/asistente/*`, `internal/asistente/prompts/*`, pruebas en `tests/`.
- Modificar: `internal/recomendador` (exportar `Recomendar`), `internal/planes` (función de armado), `cmd/api`, `docs/endpoints.md`, `docs/ia.md`, `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- La IA solo recibe ids de la lista; la API revalida todo.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
