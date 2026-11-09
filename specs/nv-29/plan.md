# plan.md — NV-29 Modelo de plan, paradas y cálculo de traslados

1. Mapeo
- Crear: `internal/planes/*`, `internal/planes/recalculo.go`, pruebas en `tests/`.
- Modificar: `cmd/api`, `.env.example` (velocidades), `docs/endpoints.md`, `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Toda consulta de planes filtra por dueño y estado en la consulta.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
