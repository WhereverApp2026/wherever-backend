# gates.md — NV-53 Sugerencias y señal social

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails` PASS (Docker, 2026-11-12) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails` PASS; el log degradado no lleva ids | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios de infraestructura ni pipeline | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (67 rutas)` | ✅ |
| Pruebas | Cinco suites verdes con servicios reales | local: build, vet, Unit y lint v2.15.0 en verde; Integration, Security, Database y Contract en el CI del PR | ⏳ CI |
| Cobertura | Mínimo 45 % | en el CI del PR | ⏳ CI |
| Semgrep | 0 hallazgos | en el CI del PR | ⏳ CI |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)`; `Memoria.md` actualizado | ✅ |
