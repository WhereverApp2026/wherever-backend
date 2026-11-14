# gates.md — NV-72 Entrar con Google

Spec en Borrador (2026-11-13), sin ejecutar. Se llena al implementar; Integration, Security, Database y Contract las corre el CI del PR (`AGENTS.md` §4). Sin firma.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| D-1 | Decisión tomada por el revisor y el responsable de A1 | — | pendiente |
| Excepción de stack | `idtoken` (o alternativa) aprobada y en `Memoria.md` | — | pendiente |
| critic B1 | Sin secretos nuevos en Git; `GOOGLE_CLIENT_ID` por config | — | pendiente |
| critic B2 | Sin `id_token`, correo ni `sub` en logs | — | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados (Parameter Store) | — | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | — | pendiente |
| Build, vet y govulncheck | Sin errores ni vulnerabilidades nuevas | — | pendiente |
| Lint | 0 hallazgos | — | pendiente |
| Unit | Verde | — | pendiente |
| Integration | `auth_google_test.go` | — | pendiente |
| Security | `auth_google_security_test.go` | — | pendiente |
| Database y Contract | `ux_google_id` sin cambios; `auth_google_contract_test.go` | — | pendiente |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | — | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | — | pendiente |
