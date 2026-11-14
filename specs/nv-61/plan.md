# plan.md — NV-61 Cupones

1. Mapeo
- Crear: `internal/cupones` (modelo, reglas puras, repositorio, service, editar, cartera, validar, estadísticas e IA, trabajo y observador, `Privacy`, handler, DTO) y pruebas de las cinco suites (sin `cuponestest`: basta `reservastest.Negocio`).
- Modificar: `internal/indexes.All()`, `scripts/routes`, `cmd/api` (`buildReservas` registra el observador y ARCO, `buildPanel` la fuente de P-49, `buildGuardia` compartida con el asistente, `startCupones`), `negocios.Repository.FijarCuponesActivos`, `authtest`, `docs/endpoints.md` y `docs/rate-limit-matrix.md`.
- Sin cambios: `negocios.cupones_activos` ya existe (nv-20); solo lo escribe el trabajo.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin códigos de canje ni datos personales en logs, rol y propiedad validados en el service, cada ruta nueva en `docs/endpoints.md`.
- Canje atómico con filtro condicional; nunca leer y luego escribir.
- Cada llave nueva de Redis con TTL; sin `MULTI`/`EXEC`.

3. Secuencia
1. Preparación: copiar filas de J6 a `docs/endpoints.md`; índices.
2. Implementación: crear y editar → cerca y detalle → cartera → canjear y validar → estadísticas → trabajo → IA → ARCO.
3. Cierre: pruebas, gates y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, Unit y lint en local; Integration, Security, Database y Contract en el CI del PR; `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
