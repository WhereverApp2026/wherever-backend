# gates.md — NV-72 Entrar con Google

Ejecutada el 2026-11-18. En local, Docker `golang:1.27.3` (AGENTS.md §4); Integration, Security, Database y Contract las corre el CI del PR. Sin firma.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| D-1 | Decisión tomada por el revisor | Camino B (Osmar Reséndiz, 2026-11-18); visto bueno del responsable de A1 pendiente en el PR | ✅ |
| Excepción de stack | `idtoken` aprobada y en `Memoria.md` | `google.golang.org/api` v0.316.0, Apache-2.0, mantenida por Google | ✅ |
| critic B1 | Sin secretos nuevos en Git; `GOOGLE_CLIENT_ID` por config | `critic-guardrails: PASS`; `.env.example` vacío, Parameter Store | ✅ |
| critic B2 | Sin `id_token`, correo ni `sub` en logs | El service no registra nada del token; `TestAuthGoogleSecurity_NothingFromTheTokenInLogsOrResponse` | ✅ local / CI |
| critic B3 | Cambios de infraestructura o pipeline aprobados | Sin cambios de pipeline; `deploy-env.sh` intacto (variable opcional) | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (254 rutas)` | ✅ |
| Build, vet y govulncheck | Sin errores ni vulnerabilidades nuevas | Build y vet limpios. `govulncheck` v1.9.0: 0 avisos por la dependencia nueva; los 3 que reporta ya estaban (stdlib 1.27.3 → el CI usa 1.27.x más reciente, fasthttp y x/crypto sin llamada) | ✅ |
| Lint | 0 hallazgos | golangci-lint v2.15.0: `0 issues` (gocyclo de `buildModules` resuelto con `buildAuth`) | ✅ |
| Unit | Verde | `go test ./internal/... ./cmd/... -count=1` verde, incluido `internal/auth/google_test.go` (verificador real contra llaves locales) | ✅ |
| Integration | `auth_google_test.go` | Compila; corre en el CI del PR | CI |
| Security | `auth_google_security_test.go` | Compila; corre en el CI del PR | CI |
| Database y Contract | `ux_google_id` sin cambios; `auth_google_contract_test.go` | Capturas `auth/google_200`, `google_201`, `google_422` escritas a mano con la forma de login; corre en el CI | CI |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | CI del PR | CI |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (76 specs)` | ✅ |
