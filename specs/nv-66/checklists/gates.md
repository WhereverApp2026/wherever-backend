# gates.md — NV-66 Actividad de lugares: conteo, afluencia y vistos recientemente

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git | `critic-guardrails: PASS` (2026-11-15) | ✅ |
| critic B2 | Ningún log junta persona y lugar de una vista; el conteo no registra coordenadas | `TestVistasSecurity_SesionPropiedadYLogs` | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados | sin cambios; índices nuevos por `scripts/indexes` | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (240 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `golangci-lint` v2.15.0: 0 issues | ✅ |
| Unit | Verde | `afluencia_test.go`, `vistas/service_test.go` | ✅ |
| Integration | `lugares_actividad_test.go` | local en verde | ✅ local, CI pendiente |
| Security | `vistas_security_test.go` | local en verde | ✅ local, CI pendiente |
| Database | `ux_vista`, `ix_recientes`, `ttl_vistas` | `tests/database` local en verde | ✅ local, CI pendiente |
| Contract | `lugares_actividad_contract_test.go` | capturado y validado en local | ✅ local, CI pendiente |
| Redis | Vista = escritura sin política; conteo = lista pública | `TestPresupuestoRedis_VistasNV66` | ✅ |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; sección NV-66 en `Memoria.md` | ✅ |
