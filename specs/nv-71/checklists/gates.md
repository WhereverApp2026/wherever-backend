# gates.md — NV-71 Dispositivos por id y reglas de moderación por filtro

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git | `critic-guardrails: PASS` (2026-11-14) | ✅ |
| critic B2 | Sin tokens de FCM en rutas, respuestas ni logs | `TestDispositivosReglasSecurity_TokenNuncaSale` | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados | sin cambios | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (232 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `golangci-lint` v2.15.0: 0 issues | ✅ |
| Unit | Verde | `umbrales_test.go` (umbral propio, heredado, inactivo), `dto_test.go` (canales y semver) | ✅ |
| Integration | `dispositivos_id_test.go`, `moderacion_reglas_filtro_test.go` | local en verde (con `TestNotificaciones*`, `TestPush*`, `TestModeracion*`) | ✅ local, CI pendiente |
| Security | `dispositivos_reglas_security_test.go` | local en verde | ✅ local, CI pendiente |
| Contract | `dispositivos_reglas_contract_test.go` | capturado y validado en local | ✅ local, CI pendiente |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; sección NV-71 en `Memoria.md` | ✅ |
| Criterio | Nadie apaga `datos_personales` | 422 en `PATCH {activa:false}` y `DELETE` | ✅ |
