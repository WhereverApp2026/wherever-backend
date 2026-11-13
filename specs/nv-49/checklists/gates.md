# gates.md — NV-49 Cifrar los datos sensibles en la base

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-12); la llave de pruebas solo en `internal/testutil` | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; `TestCifrado_AlergiasCifradasYLeidasEnClaro` revisa que las alergias no estén en los logs | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios; el equipo carga `/wherever/dev/DATA_KEYS` y corre la migración | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (100 rutas)`; sin rutas nuevas | ✅ |
| Pruebas | Cinco suites verdes con servicios reales | Local: build, vet, Unit (cifrado 20 corridas seguidas), `golangci-lint` v2.15.0 (0 issues); Integration completa (todas las suites con `DATA_KEYS` de prueba), Security (142 s) y Contract (47 s) en verde | ✅ |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente (CI) |
| Semgrep | 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; sección NV-49 en `Memoria.md` | ✅ |
| En la base | Sin alergias ni fechas en claro | `TestCifrado_RegistroGuardaLaFechaCifrada`, `TestCifrado_AlergiasCifradasYLeidasEnClaro` | ✅ |
| Transparente | El resto del código no cambia | todas las suites existentes pasan con el cifrado encendido | ✅ |
| Migración | Idempotente, con modo de prueba | `TestCifrado_MigracionIdempotente` | ✅ |
| Entorno | `DATA_KEYS` cargada y migración corrida en `/wherever/dev/` | pendiente del equipo | pendiente |
