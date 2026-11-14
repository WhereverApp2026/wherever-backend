# gates.md — NV-71 Dispositivos por id y reglas de moderación por filtro

Spec en Borrador (2026-11-13), sin ejecutar. Se llena al implementar; Integration, Security, Database y Contract las corre el CI del PR (`AGENTS.md` §4). Sin firma.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git | — | pendiente |
| critic B2 | Sin tokens de FCM en rutas, respuestas ni logs | — | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados | No aplica | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | — | pendiente |
| Build y vet | Sin errores | — | pendiente |
| Lint | 0 hallazgos | — | pendiente |
| Unit | Verde | — | pendiente |
| Integration | `dispositivos_id_test.go`, `moderacion_reglas_filtro_test.go` | — | pendiente |
| Security | `dispositivos_reglas_security_test.go` | — | pendiente |
| Database y Contract | sin índices nuevos; `dispositivos_reglas_contract_test.go` | — | pendiente |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | — | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | — | pendiente |
