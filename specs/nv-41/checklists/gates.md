# gates.md — NV-41 Envío de push con Firebase

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-12); `fcmtest` genera su llave en cada prueba; `.env.example` sin valores | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; el log del push solo lleva tipo y resultado; `TestPush_SolicitudLlegaAlTelefonoConTextoGenerico` revisa que el token no esté en los logs | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios de infraestructura ni pipeline; dos parámetros nuevos en SSM que carga el equipo | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (98 rutas)`; sin rutas nuevas | ✅ |
| Pruebas | Cinco suites verdes con servicios reales | Local: build, vet, Unit, `golangci-lint` v2.15.0 (0 issues); Integration `TestPush*` y `TestNotificaciones*` (56 s) en verde. Suites completas en el CI del PR | pendiente (CI) |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente (CI) |
| Semgrep | 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; sección NV-41 en `Memoria.md` | ✅ |
| FCM caído | No afecta la acción | `TestPush_FCMCaidoNoRompeLaAccion` (la solicitud responde en menos de 2 s) | ✅ |
| Silencio | En horario silencioso no suena | `TestPush_HorarioSilenciosoYPreferencias`, `TestWanted_PreferenciasYHorarioSilencioso` | ✅ |
| Tokens | `UNREGISTERED` se borra; 503 se reintenta | `TestPush_TokenNoRegistradoSeBorraYReintentos`, `TestFCM_ClasificaLosErrores` | ✅ |
| En vivo | Push real a un teléfono de prueba | pendiente de las credenciales de Firebase del equipo | pendiente |
