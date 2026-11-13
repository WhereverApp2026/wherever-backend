# gates.md — NV-46 Descarga de cambios

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-12) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; la ruta no registra nada propio | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios de infraestructura ni pipeline | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (99 rutas)` | ✅ |
| Pruebas | Cinco suites verdes con servicios reales | Local: build, vet, Unit, `golangci-lint` v2.15.0 (0 issues); Integration (`TestSync*`, `TestPlanesAPI*`, `TestGrupos*`, `TestVotaciones*`, 70 s) y Database (31 s) en verde. Suites completas en el CI del PR | pendiente (CI) |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente (CI) |
| Semgrep | 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; sección NV-46 en `Memoria.md` | ✅ |
| Reloj | El reloj del teléfono no influye | el cursor guarda un contador y la hora del servidor; `TestSync_CompletaEIncrementalConBorrados` | ✅ |
| Propiedad | Nunca datos de otra persona | `TestSync_SoloLoPropio` (también siendo integrante del grupo; cursor ajeno = 422) | ✅ |
| Completa | Más de 30 días = 410 | `TestSync_CursorDeHaceMasDe30DiasPideDescargaCompleta` | ✅ |
