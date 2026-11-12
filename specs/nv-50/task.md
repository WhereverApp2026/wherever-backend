# task.md — NV-50 Embeddings de negocios

## Fase 0 — Preparación
- [x] T01 Revisar la spec y corregir el drift (colección aparte, límites propios, comando en `scripts/`); rama `claude/friendly-mahavira-0a9451`, PR contra `main`. Falta la firma del revisor.

## Fase 1 — Implementación
- [x] T02 Adaptador `ia.GeminiVectores` (`batchEmbedContents`, 768 dimensiones, formato de documento y de consulta de `gemini-embedding-2`) detrás de `ia.Vectorizador`; plantilla pública versionada en `internal/embeddings`; colección `vectores_negocios`.
- [x] T03 Recalcular al importar (`negocios.Service.WithEmbeddings`) y con `make embeddings` (`scripts/recalcular-embeddings`, `-todos` para forzar); solo los desactualizados.
- [x] T04 Límites propios `IA_EMB_LIMITE_MIN`/`IA_EMB_LIMITE_DIA` en Redis (`ia.VectoresLimitados`, fail-closed); si el proveedor falla o no hay cupo la importación sale igual.

## Fase 2 — Cierre
- [x] T05 Pruebas (Unit `internal/ia/vectores_test.go`, `internal/embeddings/plantilla_test.go`; Integration `tests/integration/embeddings_test.go`), `docs/rate-limit-matrix.md`, `.env.example` y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en verde (local); Integration, Security, Database, Contract, cobertura y Semgrep en el CI del PR; evidencia en `gates.md`. Pendiente: CI y prueba en vivo con los 30 negocios.
