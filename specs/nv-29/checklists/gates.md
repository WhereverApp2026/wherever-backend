# gates.md — NV-29 Modelo de plan, paradas y cálculo de traslados

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (local, 2026-11-11) | OK |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; `planes` no escribe logs | OK |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | No aplica: sin cambios de infraestructura ni pipeline | OK |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (43 rutas)`; nv-29 no agrega rutas | OK |
| Build y lint | `go build`, `go vet`, Unit y `golangci-lint` v2.15.0 | local en Docker: Unit verde, `0 issues` | OK |
| Pruebas | Integration, Security, Database y Contract verdes con servicios reales | CI del PR | pendiente |
| Cobertura | Mínimo 45 % | CI del PR (`check_coverage`) | pendiente |
| Semgrep | 0 hallazgos | CI del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)`; sección NV-29 en `Memoria.md` | OK |
| Recalculo | Ningún plan guardado tiene paradas traslapadas | `TestRecalcular_ParadaQueNoCabe`, `TestRecalcular_TrasladoJustoCabe`; el repositorio solo recibe planes ya recalculados | OK |
