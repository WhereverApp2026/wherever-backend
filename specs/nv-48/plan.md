# plan.md — NV-48 Capa de cifrado de campos

1. Mapeo
- Crear: `internal/platform/cifrado/*`, `cmd/migrar-cifrado`, pruebas en `tests/`.
- Modificar: repositorios de `usuarios` (gustos, preferencias), `internal/privacidad/exporter.go`, `internal/platform/config`, `.env.example`, `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- La llave nunca entra a Git ni a logs.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
