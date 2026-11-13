# plan.md — NV-58 La Vaca: gastos compartidos del grupo

1. Mapeo
- Crear: `internal/vaca/` (model, repository, service, handler, dto, `corte.go`, privacy), `internal/testutil/vacatest`, `tests/integration/vaca_test.go`, `tests/security/vaca_security_test.go`.
- Modificar: `scripts/indexes` (`gastos`, `deudas_vaca`), `internal/http` (registro de rutas), `internal/privacidad` (registro de `Purger`/`Exporter`), `docs/endpoints.md` (7 filas), `tests/database/indexes_test.go`, fixtures de contrato de la exportación.
- Sin cambios: `grupos` (nv-37) solo se lee por la interfaz `Members`; `/sync/pendientes` es de nv-47.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Dinero en centavos enteros; ningún `float` en el cálculo.
- Candado `lock:vaca:{grupo_id}` con TTL; Redis caído = 503.

3. Secuencia
1. Preparación: nv-37 en `main`; revisor resolvió las decisiones de la spec.
2. Implementación: `corte.go` con sus pruebas primero; luego repositorio, índices, service, handler.
3. Cierre: privacidad, contrato, documentos y `Memoria.md`.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0.
