# gates.md — NV-33 Respaldo, cancelación y «otra idea»

Evidencia local del 2026-11-11 en `golang:1.27.3` con Docker (las suites con contenedor con `--network host` y el socket de Docker, una a la vez con `-p 1`).

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; las pruebas revisan que la frase no llegue a los logs | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios de infraestructura ni pipeline | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (45 rutas)` | ✅ |
| Pruebas | Cinco suites verdes con servicios reales | Unit `go test ./internal/...` ok; Integration ok (166 s); Security ok (50 s); Database ok (33 s); Contract ok tras recapturar `usuarios/exportacion_descargar_200.json` (`usos_ia` de nv-27 junto a `planes`) | ✅ |
| Lint | `golangci-lint` v2.15.0 | `0 issues.` | ✅ |
| Cobertura | Mínimo 45 % | la corre el CI del PR a `main` | pendiente |
| Semgrep | 0 hallazgos | lo corre el CI del PR a `main` | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)`; sección NV-33 en `Memoria.md` | ✅ |
| Respaldo | Con la IA caída responde 3 opciones | `TestGenerar_CadaFallaDeLaIATerminaEnReglas`, `TestGenerar_TopeYLimitesTerminanEnReglasSinGastar`, `TestAsistente_CadaFallaDeLaIATerminaEnReglas` | ✅ |
| Costo | El respaldo no cuenta contra el tope | `TestGenerar_RespaldoNoSumaGastoTrasUnaLlamadaFallida`; en Integration el contador de `cuota:ia_gasto` no cambia | ✅ |
| Plazo | Nunca pasa de 8 s hacia el respaldo | 7 s de IA + 1 s de reserva; `TestGenerar_IALentaTerminaEnReglasDentroDelPlazo` (< 1 s con plazo corto) y en Integration con `authtest.PlazoIA` | ✅ |
| Cancelación | Un contexto cancelado corta la IA y no arma respaldo | `TestGenerar_CancelacionCortaLaIAYNoArmaRespaldo` | ✅ |
| Otra idea | No repite lugares ni trae no aprobados | `TestOtraIdea_NoRepiteLugares`, `TestAsistente_OtraIdeaNoRepiteLugares` | ✅ |
| Medición | 20 frases reales con tasa de descartes y costo | necesita la llave de SSM; no corrida | pendiente |