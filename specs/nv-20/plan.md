# plan.md — NV-20 Negocios aprobados, categorías y carga semilla

1. Mapeo
- Crear: `internal/negocios/*`, `internal/categorias/*`, `internal/busquedas/*`, `docs/semilla-negocios.csv`, `scripts/indices_negocios.*`, pruebas en `tests/`.
- Modificar: `cmd/api` (rutas), `docs/endpoints.md`, `docs/rate-limit-matrix.md`, `Memoria.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Los endpoints públicos filtran siempre por `estado = aprobado` en la consulta, no después.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
