# gates.md — NV-69 Extras de grupos: editar y disolver, punto de encuentro, modo sorpresa y «Me apunto»

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-14) | ✅ |
| critic B2 | Sin tokens ni coordenadas de integrantes en logs ni respuestas | `TestGruposExtrasSecurity_UbicacionYSorpresaNoSeFiltran` (respuestas y `env.Logs`) | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados | sin cambios | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (228 rutas)` | ✅ |
| Build y vet | Sin errores | local | ✅ |
| Lint | 0 hallazgos | `golangci-lint` v2.15.0: 0 issues | ✅ |
| Unit | Verde | `internal/grupos`, `internal/planes` y el resto | ✅ |
| Integration | `grupos_extras_test.go` y presupuesto de Redis | local: `TestGruposExtras*`, `TestPresupuestoRedis_UbicacionDelGrupoNV69`, `TestGrupos_*`, `TestVotaciones*`, `TestVaca*`, `TestChat*`, `TestPlanes*`, `TestSync*` | ✅ local, CI pendiente |
| Security | `grupos_extras_security_test.go` | local en verde | ✅ local, CI pendiente |
| Database y Contract | sin índices nuevos; `grupos_extras_contract_test.go` | contrato capturado y validado en local | ✅ local, CI pendiente |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; sección NV-69 en `Memoria.md` | ✅ |
| Privacidad | Nadie sabe dónde está otra persona con más precisión que unos km; la sorpresa no se filtra | `TestGruposExtras_SugeridoYTraslados`, `TestGruposExtras_Sorpresa` | ✅ |
