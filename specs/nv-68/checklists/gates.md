# gates.md — NV-68 Extras de planes

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-14) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `TestPlanesExtrasSecurity_SesionYPopularesSinDatos`: el punto exacto no llega a los logs | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios; índice nuevo `ix_populares` por `scripts/indexes` | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (220 rutas)` | ✅ |
| Pruebas | Cinco suites verdes con servicios reales | Local: build, vet, Unit, `golangci-lint` v2.15.0 (0 issues); Integration (`TestPlanesExtras*`, `TestPlanesAPI*`, `TestSync*`, `TestGrupos*`, `TestVotaciones*`), Security y Database en verde. Suites completas en el CI del PR | pendiente (CI) |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente (CI) |
| Semgrep | 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; sección NV-68 en `Memoria.md` | ✅ |
| Privacidad | Ningún plan popular ni vista pública revela al dueño | `TestPlanesExtras_PopularesYCopiarPublico` (sin dueño ni notas; bloqueado = 404) | ✅ |
| Versión | Cambiar una parada nunca pisa una edición más nueva | `TestPlanesExtras_AlternativasYCambiarParada` (409 con versión vieja, 400 sin If-Match) | ✅ |
