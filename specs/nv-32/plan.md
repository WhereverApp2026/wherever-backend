# plan.md — NV-32 Itinerario con IA y verificación contra el catálogo

1. Mapeo
- Crear: `internal/asistente/itinerario.go`, `internal/asistente/descartes.go`, `internal/asistente/prompts/itinerario.txt`, `internal/asistente/verificar_test.go`.
- Modificar: `internal/asistente/armar.go` (recorrido común de `Armar` y `Verificar`), `internal/asistente/service.go`, `internal/indexes`, `cmd/api/main.go`, `internal/testutil/authtest`, `tests/integration/asistente_test.go`, `tests/database/indexes_test.go`, `docs/endpoints.md`.
- Sin cambios: `internal/planes`, `internal/ia`, `scripts/routes` (sin rutas nuevas).

2. Guardarraíles
- Servicios reales en las pruebas (el proveedor de IA se sustituye por `iatest.Modelo`), sin secretos en Git, sin datos personales en logs ni en `ia_descartes`.
- La IA solo recibe ids de la lista; la API revalida todo.

3. Secuencia
1. Preparación: rama y drift.
2. Implementación: itinerario, verificación, descartes.
3. Cierre: pruebas, documentos, `Memoria.md`, PR contra `nv-31`.

4. Verificación
- Local: `go build`, `go vet`, Unit, `golangci-lint`, `critic`, `check_endpoints`, `check_sdd`. CI: Integration, Security, Database, Contract, cobertura ≥ 45 %, Semgrep en 0.
