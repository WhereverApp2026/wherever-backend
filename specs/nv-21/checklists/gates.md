# gates.md — NV-21 Lista, ficha y mapa de lugares

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-10) | ok |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; `TestLugares_LocationNeverLogged` | ok |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin cambios de infraestructura ni pipeline | ok |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (43 rutas)` | ok |
| Pruebas | Cinco suites verdes con servicios reales | Unit, Integration (175 s), Security (54 s), Database (36 s) y Contract (34 s) `ok` con `-count=1 -p 1` en `golang:1.27.3`; `golangci-lint` v2.15.0: 0 | ok |
| Cobertura | Mínimo 45 % | `check_coverage: OK (46.5% >= 45%)` | ok |
| Semgrep | 0 hallazgos | no corrido en esta sesión; lo corre CI (`semgrep`) | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)` | ok |
| Rendimiento | p95 lista < 150 ms con 1,000 negocios | `TestLugares_PerformanceP95` (40 peticiones con filtros, ubicación y radio de 20 km) pasa la aserción | ok |
