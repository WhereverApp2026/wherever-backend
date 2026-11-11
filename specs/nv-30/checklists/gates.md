# gates.md — NV-30 Guardar, leer, editar y repetir planes

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (local, 2026-11-11) | ok |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; `TestPlanesSecurity_SoloElDuenoVeYEditaSuPlan` revisa que la nota del plan no llegue a los logs | ok local, CI pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios de infraestructura ni pipeline | n/a |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (53 rutas)` | ok |
| Pruebas | Cinco suites verdes con servicios reales | local: build, vet, Unit y `golangci-lint` (0 issues); Integration, Security, Database y Contract en el CI del PR | pendiente CI |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI | pendiente CI |
| Semgrep | 0 hallazgos | Semgrep en el CI | pendiente CI |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)`; sección NV-30 en `Memoria.md` | ok |
| Aislamiento | Plan ajeno responde 404 | `TestPlanesSecurity_SoloElDuenoVeYEditaSuPlan` (8 rutas + lista) | pendiente CI |
| Concurrencia | Sin `If-Match` = 400; versión vieja = 409 | `TestPlanesAPI_ConcurrenciaIfMatch` | pendiente CI |
