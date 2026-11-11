# plan.md — NV-24 Recomendaciones y «No me interesa»

1. Mapeo
- Crear: `internal/recomendaciones/{repository,service,handler,dto,contexto,privacy}.go`, sus pruebas Unit y `tests/{integration,security}/recomendaciones*_test.go`.
- Modificar: `internal/negocios/candidatos.go` (`ItemOf`, `MexicoCity`), `cmd/api/main.go` (pesos al arrancar, módulo y ARCO), `internal/indexes`, `internal/testutil/authtest`, `scripts/routes`, `tests/database/indexes_test.go`, `tests/integration/recomendador_test.go` (`insertRecoEnv`) y `docs/endpoints.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Los filtros duros se aplican en la consulta, no después de puntuar.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- En local: `go build`, `go vet`, Unit y `golangci-lint` v2.15.0; `critic`, `check_endpoints`, `check_sdd`. En el CI del PR: Integration, Security, Database, Contract, cobertura ≥ 45 % y Semgrep en 0.
