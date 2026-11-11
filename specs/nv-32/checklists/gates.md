# gates.md — NV-32 Itinerario con IA y verificación contra el catálogo

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-11) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; el log del descarte lleva solo `motivo` | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | No aplica: sin cambios de infraestructura ni pipeline | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (44 rutas)`; sin rutas nuevas | ✅ |
| Build y lint | `go build`, `go vet`, `golangci-lint` | local en `golang:1.27.3`: verdes, `0 issues` | ✅ |
| Pruebas | Unit verde local; Integration, Security, Database y Contract con servicios reales | Unit local verde; las demás en el CI del PR | ⏳ CI |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | ⏳ CI |
| Semgrep | 0 hallazgos | Semgrep en el CI del PR | ⏳ CI |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)`; sección nv-32 en `Memoria.md` | ✅ |
| Alucinación | Ningún plan devuelto contiene un negocio fuera del catálogo | `Unit/Verificar` (id inventado, no ObjectID, cerrado, presupuesto, orden, repetido) e `Integration/Asistente` (alérgeno e id inventado descartados y en `ia_descartes`) | ✅ Unit · ⏳ Integration |
