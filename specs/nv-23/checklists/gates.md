# gates.md — NV-23 Filtros duros y puntuación de recomendaciones

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-11) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; `por_que` sin alergias, dieta ni cifras (`TestExplicador_SinDatosSensibles`) | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios de infraestructura ni pipeline | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (43 rutas)`; sin endpoints nuevos | ✅ |
| Pruebas | Cinco suites verdes con servicios reales | Unit, Integration (145 s), Security, Database y Contract `ok` con `-p 1` en Docker; build, vet y `golangci-lint` v2.15.0 con 0 issues | ✅ |
| Cobertura | Mínimo 45 % | `check_coverage: OK (48.6% >= 45%)` (Unit) | ✅ |
| Semgrep | 0 hallazgos | no corrido en local; lo corre CI en el PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)`; sección nv-23 en `Memoria.md` | ✅ |
| Alergias | Ningún caso de prueba incumple una alergia | `TestRecomendador_NingunaAlergiaSeCuela` (catálogo, combinadas y fuera de catálogo) y `TestFiltro_Alergia*` | ✅ |
| Determinismo | Dos corridas iguales dan el mismo orden | `TestRecomendador_OrdenEsperadoConDiezNegocios` compara dos corridas; `TestOrdenar_EmpatesYFiltroEnGo` | ✅ |
