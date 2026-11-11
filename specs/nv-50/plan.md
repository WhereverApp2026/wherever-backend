# plan.md — NV-50 Embeddings de negocios

1. Mapeo (conciliado con lo implementado)
- Crear: `internal/ia/vectores.go` y `vectores_limite.go` (adaptador y límites), `internal/embeddings/*` (plantilla, `vectores_negocios`, recálculo, armado), `scripts/recalcular-embeddings` (`make embeddings`), `internal/testutil/iatest/vectores.go`, pruebas en `internal/ia`, `internal/embeddings` y `tests/integration/embeddings_test.go`.
- Modificar: `internal/negocios/service.go` (gancho al importar), `internal/ia/gemini.go` (POST compartido), `internal/ia/limite.go` (`Sumar`), `internal/platform/config`, `cmd/api/main.go`, `configs/.env.example`, `docs/rate-limit-matrix.md`, `Makefile`, `Memoria.md`.
- Para nv-51: `internal/recomendador` (factor), `internal/asistente` (candidatos) y `docs/ml-evaluacion.md`.
- Sin cambios: `docs/endpoints.md` (sin rutas), ARCO (sin datos de personas), `scripts/indexes` (sin índices nuevos).

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs.
- El texto vectorizado es solo público.

3. Secuencia
1. Preparación
2. Implementación
3. Cierre

4. Verificación
- Local: `go build`, `go vet`, Unit, `golangci-lint`, `critic`, `check_endpoints`, `check_sdd`. CI del PR: Integration, Security, Database, Contract, cobertura ≥ 45 %, Semgrep en 0.
