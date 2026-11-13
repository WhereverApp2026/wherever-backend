# gates.md — NV-40 Dispositivos y centro de notificaciones

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-12) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; `TestNotificacionesSecurity_SesionYTokenFueraDeLogs`; `Emit` solo registra el tipo | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios de infraestructura ni pipeline | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (98 rutas)` | ✅ |
| Pruebas | Cinco suites verdes con servicios reales | Local: build, vet, Unit, `golangci-lint` v2.15.0 (0 issues); Integration (133 s), Security (55 s), Database (44 s) y Contract (37 s) en verde. Suites completas en el CI del PR | pendiente (CI) |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente (CI) |
| Semgrep | 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; sección NV-40 en `Memoria.md` | ✅ |
| Centro | Listar, no leídas, marcar, leer todas y borrar; la de otro = 404 | `TestNotificaciones_CentroListarMarcarYBorrar` | ✅ |
| Eventos | Amistad, invitación y votación avisan | `TestNotificaciones_EventosDeAmistadGrupoYVotacion` | ✅ |
| Dispositivos | Máximo 5; token fuera de la ruta, logs y exportación | `TestNotificaciones_Dispositivos`, `TestNotificaciones_ExportarYBorrarCuenta` | ✅ |
