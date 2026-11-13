# gates.md — NV-48 Capa de cifrado de campos

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-12); `.env.example` con `DATA_KEYS=` vacío; la llave de pruebas solo en `internal/testutil` | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | los errores del paquete no llevan el valor | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios; el equipo carga `/wherever/dev/DATA_KEYS` | ✅ (no aplica) |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | sin rutas nuevas | ✅ |
| Pruebas | Cinco suites verdes con servicios reales | Local: build, vet, Unit (`cifrado`, `config`), fuzz con semillas y `golangci-lint`. Suites completas en el CI del PR | pendiente (CI) |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente (CI) |
| Semgrep | 0 hallazgos | lo corre el CI del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK`; sección NV-48 en `Memoria.md` | ✅ |
| Rotación | Rotar no pierde datos | `TestCifrado_Rotacion` | ✅ |
| Arranque | Sin llave no arranca en prod | `TestLoad_DataKeys` | ✅ |
