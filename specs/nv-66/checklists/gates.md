# gates.md — NV-66 Actividad de lugares: conteo, afluencia y vistos recientemente

Sin ejecutar: se llena al implementar. Integration, Security, Database y Contract las corre el CI del PR (`AGENTS.md` §4).

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | — | pendiente |
| critic B2 | Sin tokens ni datos personales en logs; vistas sin `usuario_id` + `negocio_id` en logs | — | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados | No aplica | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | — | pendiente |
| Build y vet | Sin errores | — | pendiente |
| Lint | 0 hallazgos | — | pendiente |
| Unit | Verde | — | pendiente |
| Integration | `lugares_conteo_test.go`, `lugares_afluencia_test.go`, `vistas_test.go`, `privacidad_vistas_test.go`, `TestPresupuestoRedis_VistasNV66` | — | pendiente |
| Security | `vistas_security_test.go` | — | pendiente |
| Database y Contract | índices y TTL de `historial_vistas`; `lugares_actividad_contract_test.go` | — | pendiente |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | — | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | — | pendiente |
