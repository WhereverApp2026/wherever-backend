# NV-02 — Molde sin endpoints

## T1 — Base + registro + test, cero rutas
- **Crear**: `shared/pagination/page_result.go {items, siguiente_cursor, limite}`, `cursor.go`, `shared/validation/registry.go + pagination_query.go`, `shared/apperr/apperr.go`, `shared/service/service.go`, `internal/arch/arch_test.go (6)`, `pagination/cursor_test.go (4)`.
- **Modificar**: `internal/http/app.go` registra los validators (1 línea).
- **Verificar**: `build + Unit 100% + critic PASS + endpoints OK + coverage ≥45% + semgrep 0`.
- **No hacer**: handlers, colecciones/repositorios, índices, `AdminPolicy` usada, emisión JWT, `cache:{entidad}`, filas en `endpoints.md`, reorden del pipeline, secretos.
