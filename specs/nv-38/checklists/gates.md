# gates.md — NV-38 Enlace de invitación y vista pública

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-12) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; `TestEnlaceSecurity_CodigoSoloComoHashYFueraDeLogs` (el código no está en los logs) | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios de infraestructura ni pipeline; `carriesTokens` suma una ruta (nv-12) | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (87 rutas)` | ✅ |
| Pruebas | Cinco suites verdes con servicios reales | Local: build, vet, Unit, `golangci-lint` v2.15.0 (0 issues); Integration (`TestGrupos*`, `TestEnlace*`, 78 s), Security (39 s) y Database (35 s) en verde. Suites completas en el CI del PR | pendiente (CI) |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente (CI) |
| Semgrep | 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; sección NV-38 en `Memoria.md` | ✅ |
| Código | Solo hash en la base; nunca en la ruta ni en logs | `TestCodigo_AleatorioYSoloHash`, `TestEnlaceSecurity_CodigoSoloComoHashYFueraDeLogs` | ✅ |
| Vista pública | Sin personas, notas ni presupuesto | `TestPublicPlan_SinPersonas`, `TestEnlace_CompartirVerYUnirse` | ✅ |
| Límite | 30 por minuto por IP en la vista pública | `TestEnlaceSecurity_LimitePorIPYCodigoInventado` (31.ª = 429 con `Retry-After`) | ✅ |
