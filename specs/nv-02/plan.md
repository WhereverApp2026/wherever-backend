# plan.md — NV-02 Molde sin endpoints

1. Mapeo
- Crear: `internal/shared/pagination/{cursor.go,page_result.go}`, `internal/shared/validation/{registry.go,pagination_query.go}`, `internal/shared/apperr/apperr.go`, `internal/shared/service/service.go` (interfaz marcadora), `internal/arch/arch_test.go`.
- Modificar: `internal/http/app.go` (1 línea para registrar los validators); sin reordenar el pipeline.

2. Guardarraíles
- El cursor es opaco (Base64 de `{_id, orden}`); inválido → `ValidationError`.
- `limite` por defecto 20, máximo 50.
- Los validators se registran desde un solo punto, no por módulo.
- Ningún handler importa `go.mongodb.org/mongo-driver` ni `neo4j-go-driver`.

3. Pruebas
- `arch_test.go (6)`: imports prohibidos, validators registrados desde un solo punto, sin `recover` en handlers, `PageResult` JSON, cursor inválido, `limite` fuera de rango.
- `pagination/cursor_test.go (4)`: ida y vuelta, vacío, manipulado, ordenamiento estable.

4. Secuencia
1. `apperr` (los 11 errores tipados de constitución §5) y mapeo en el `ErrorHandler` existente.
2. `pagination` (cursor + `PageResult`).
3. `validation` (registro único + validator de `PaginationQuery`).
4. Test de arquitectura.

5. Verificación
- `go build ./... → sin errores`
- `go test ./internal/... -count=1 → 100% verdes`
- `scripts/critic-guardrails.sh → PASS` · `scripts/check_endpoints.sh → OK`
