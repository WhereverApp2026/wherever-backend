# gates.md — NV-37 Grupos e integrantes

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-12) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; los errores de MongoDB salen como 503 sin texto | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios de infraestructura ni pipeline | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (83 rutas)` | ✅ |
| Pruebas | Cinco suites verdes con servicios reales | Local: build, vet, Unit, `golangci-lint` v2.15.0 (0 issues) e Integration de grupos y planes (`-run 'TestGrupos\|TestPlanesAPI'`, 104 s) en verde. Suites completas en el CI del PR | pendiente (CI) |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente (CI) |
| Semgrep | 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; sección NV-37 en `Memoria.md` | ✅ |
| Tope | 20 intentos concurrentes respetan el máximo | `TestGrupos_Concurrencia_NuncaMasDeDoce`: entra 1 de 20, `ocupados` = 12 y 12 integrantes | ✅ |
| Amigos | Solo amigos aceptados con cuenta activa | `TestGrupos_SoloAmigosActivos` (extraño, inactivo, solicitud pendiente y uno mismo = 422) | ✅ |
| Propiedad | Grupo o plan ajeno = 404; integrante no edita el plan | `TestGrupos_PlanAjenoYGrupoAjenoSon404`, `TestGrupos_IntegranteLeeElPlanYNoLoEdita`, `TestGruposSecurity_GrupoAjenoNoRevelaNada` | ✅ |
