# plan.md — NV-37 Grupos e integrantes

1. Mapeo
- Crear: `internal/grupos/*`, `internal/votaciones/*`, `internal/planes/compartir.go`, pruebas en `tests/`.
- Modificar: `internal/planes` (acceso de integrantes en lectura), `internal/privacidad/job.go` (cierre de votaciones), `cmd/api`, `docs/endpoints.md`, `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- El código del enlace nunca se guarda ni se registra en claro.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
