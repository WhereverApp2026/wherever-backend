# plan.md — NV-19 Aceptación de términos y aviso de privacidad

1. Mapeo
- Crear: `internal/usuarios/consentimientos.go`, `internal/usuarios/account/consents.go`, `internal/usuarios/account/consents_test.go`, `tests/integration/consentimientos_test.go`, `tests/contract/testdata/usuarios/consentimientos_201.json`.
- Modificar: `internal/usuarios/repository.go` (`Privacy.NoticeVersion`), `internal/usuarios/account/{dto,handler,service}.go`, `internal/indexes/indexes.go`, `cmd/api/main.go`, `internal/testutil/authtest/authtest.go`, `tests/database/indexes_test.go`, `tests/contract/usuarios_contract_test.go` y sus snapshots, `docs/endpoints.md`, `docs/aviso-privacidad.md`.
- Sin cambios: `internal/configuracion` (ya publica las versiones), `internal/privacidad` (solo se registra un Purger/Exporter más), `internal/platform/cache` y Redis.

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- Sin `if err` nuevo en `buildModules` (gocyclo 10): `WithConsents` se encadena.

3. Secuencia
1. Preparación: conciliar la spec con nv-05 y nv-16.
2. Implementación: repositorio, service, handler, armado e índices.
3. Cierre: pruebas, documentos, gates y `Memoria.md`.

4. Verificación
- Local: `go build`, `go vet`, Unit, `golangci-lint` v2.15.0, `critic`, `check_sdd`, `check_endpoints`.
- CI del PR: Integration, Security, Database, Contract, cobertura ≥ 45 %, Semgrep en 0.
