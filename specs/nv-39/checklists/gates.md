# gates.md — NV-39 Votaciones

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-12) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; el log del `Closer` solo lleva el id de la votación | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios de infraestructura ni pipeline; trabajador nuevo dentro de la API | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (91 rutas)` | ✅ |
| Pruebas | Cinco suites verdes con servicios reales | Local: build, vet, Unit, `golangci-lint` v2.15.0 (0 issues); Integration de votaciones 3 de 3 corridas en verde, Security (72 s), Database (53 s) y Contract (43 s) en verde. Suites completas en el CI del PR | pendiente (CI) |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente (CI) |
| Semgrep | 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; sección NV-39 en `Memoria.md` | ✅ |
| Una sola vez | Dos instancias cierran y aplican una vez | `TestVotaciones_TrabajoCierraUnaSolaVezConDosInstancias` (el plan sube una versión). Encontró que se aplicaba dos veces; se corrigió con `ClaimApply` | ✅ |
| Constancia | Cerrar cambia el plan y deja ganadora, quién cerró y si se aplicó | `TestVotaciones_VotarCambiarYCerrarAplicaLaGanadora`, `TestVotaciones_GanadoraQueNoCabeQuedaSinAplicar` | ✅ |
| Redis | Sin comandos nuevos | el cierre solo usa MongoDB; sin `lock:` | ✅ |
