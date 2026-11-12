# gates.md — NV-25 Clima e Inicio

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-12) | ok |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; el log del clima no lleva coordenadas | ok |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin cambios de infraestructura ni pipeline | ok |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (69 rutas)` | ok |
| Pruebas | Unit local; Integration, Security, Database y Contract en el CI | Unit verde en `golang:1.27.3`; suites con contenedor en el CI del PR | Unit ok, CI pendiente |
| Lint | `golangci-lint` v2.15.0 | `0 issues` | ok |
| Cobertura | Mínimo 45 % | CI del PR | pendiente |
| Semgrep | 0 hallazgos | CI del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)` | ok |
| Clima caído | El Inicio responde sin clima | `TestInicio_ClimaCaidoRespondeSinClima` y `TestClima_LentoYCaido` (CI); `TestHome_SinClimaFactorNeutroYErrorDeRecomendaciones` (Unit) | Unit ok, CI pendiente |
| Redis | Sin comandos nuevos | caché en memoria por instancia; `TestPresupuestoRedis_*` sin cambios | ok |
