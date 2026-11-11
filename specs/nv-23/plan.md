# plan.md — NV-23 Filtros duros y puntuación de recomendaciones

1. Mapeo
- Crear: `internal/recomendador/*` (con `pesos_recomendador.json` embebido), `internal/negocios/candidatos.go`, `tests/integration/recomendador_test.go`.
- Modificar: `internal/negocios/catalog.go` e `import.go` (`ninguno` en `alergenos`), `Memoria.md`.
- Fuera de esta spec (drift corregido): `internal/clima` es nv-25 e `internal/inicio` es nv-24; sin cambios en `cmd/api`, `.env.example` ni `docs/endpoints.md` porque no hay endpoints ni variables nuevas. `internal/usuarios` no cambia: `Contexto` recibe los gustos ya leídos.
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
