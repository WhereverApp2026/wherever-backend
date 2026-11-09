# plan.md — NV-46 Descarga de cambios

1. Mapeo
- Crear: `internal/sync/*`, pruebas en `tests/`.
- Modificar: `internal/planes` y `internal/usuarios` (marcar `sync_seq` al escribir), `cmd/api`, `docs/endpoints.md`, `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Toda acción reutiliza el servicio de su endpoint, no una ruta paralela.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
