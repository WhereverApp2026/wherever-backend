# gates.md — NV-47 Subida de acciones hechas sin conexión

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-12) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS` | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios de infraestructura ni pipeline | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (100 rutas)` | ✅ |
| Pruebas | Cinco suites verdes con servicios reales | Local: build, vet, Unit, `golangci-lint` v2.15.0 (0 issues); Integration `TestPendientes*` y `TestSync*` (50 s). Suites completas en el CI del PR | pendiente (CI) |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente (CI) |
| Semgrep | 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; sección NV-47 en `Memoria.md` | ✅ |
| Idempotencia | Repetir un lote no cambia nada | `TestPendientes_LoteNuevoRepetidoYConflicto` | ✅ |
| Conflicto | Nunca pierde datos | conflicto con la copia del servidor, sin sobrescribir | ✅ |
| Concurrencia | `sync_seq` sin huecos ni repetidos | `TestPendientes_Concurrencia_SinHuecosNiRepetidos` | ✅ |
