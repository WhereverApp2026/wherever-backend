# gates.md — NV-50 Embeddings de negocios

Evidencia local del 2026-11-11 en `golang:1.27.3` con Docker. Integration, Security, Database y Contract las corre el CI del PR a `main` (AGENTS §4).

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS`; la llave sigue solo en `.env`/SSM y viaja en `x-goog-api-key` (`TestGeminiVectores_DocumentosConFormatoYLlaveEnEncabezado`) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; el recálculo registra solo conteos y el error del proveedor | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios de infraestructura ni pipeline | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | sin rutas nuevas; `check_endpoints: OK (58 rutas)` | ✅ |
| Pruebas | Cinco suites verdes con servicios reales | Unit ok (`go test ./internal/... ./cmd/...`); Integration (`tests/integration/embeddings_test.go`), Security, Database y Contract en el CI del PR | Unit ✅, resto pendiente del CI |
| Lint | `golangci-lint` v2.15.0 | `0 issues.` | ✅ |
| Cobertura | Mínimo 45 % | la corre el CI del PR a `main` | pendiente |
| Semgrep | 0 hallazgos | lo corre el CI del PR a `main` | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)`; sección NV-50 en `Memoria.md` | ✅ |
| Degradación | Servicio caído o sin cupo, nada se rompe | `TestRecalcular_FallaOLimiteNoRompenYSigueDondeQuedo`, `TestImportar_CalculaVectoresYSiFallanLaSemillaSaleIgual`, `TestVectoresLimitados_SinRedisSeCierra` | ✅ Unit, Integration en CI |
| Solo público | Texto vectorizado sin datos privados | `TestPlantilla_SinDatosPrivadosNiDeControl`, `TestPlantilla_DescripcionConContactoSeAnonimiza`, `TestGeminiVectores_ConsultaAnonimizadaConPrefijo` | ✅ |
| En vivo | Embeddings reales de los 30 negocios del piloto | necesita la llave de SSM y el parámetro `IA_EMBEDDINGS_ID`; no corrida | pendiente |
