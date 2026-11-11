# plan.md — NV-28 Límite por usuario y protección de datos en la IA

1. Mapeo
- Crear: `internal/ia/limite.go`, `internal/ia/limite_test.go`, `tests/integration/ia_limite_test.go`, `tests/security/ia_security_test.go`.
- Modificar: `internal/ia/guardia.go` (límites en `Generar` y `GenerarValidado`), `internal/ia/esquema.go` (`SoloDeLaLista`), `internal/platform/config`, `configs/.env.example`, `docs/rate-limit-matrix.md`.
- Sin cambios: lo que esta spec no nombra.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Instrucciones y entrada del usuario separadas siempre.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- En local: `go build`, `go vet`, Unit y `golangci-lint` (Docker). En el CI del PR: Integration, Security, Database, Contract, cobertura ≥ 45 % y Semgrep en 0. Cierre con `critic`, `check_endpoints` y `check_sdd`.
