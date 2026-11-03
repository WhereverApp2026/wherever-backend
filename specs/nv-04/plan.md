# plan.md — NV-04 Autenticación, sesiones y recuperación de contraseña (J1)

1. Mapeo
- Crear (real): `internal/auth/{dto,handler,routes,service,repository,tokens,password}.go`, `internal/usuarios/repository.go` (mínimo), `internal/platform/mailer/{mailer,memory,log}.go`, `internal/platform/mongo/indexes.go`, `internal/indexes/indexes.go`, `scripts/indexes/main.go`, `internal/testutil/authtest/authtest.go`, `tests/integration/auth_{service,}_test.go`, `tests/security/auth_security_test.go`, `tests/database/indexes_test.go`, `tests/contract/{auth_contract,shape}_test.go` + `testdata/auth/*.json`.
- Modificar (real): `internal/http/app.go` (interfaz `Module`; `/api/v1/auth` se registra desde `cmd/api/main.go`), `internal/http/middleware/{jwt,roles}.go` (`AccessParser`, `RequireAuth`), `internal/platform/config` (`JWT_ACCESS_TTL`, `JWT_REFRESH_TTL`), `configs/.env.example`, `docs/endpoints.md`, `docs/rate-limit-matrix.md`, `docs/asvs-l2-checklist.md`, `.github/workflows/ci-pr.yml` (pasos `database` y `contract`), `Makefile`. Sin reordenar el pipeline. `GOOGLE_CLIENT_ID` llega con T7 y `MAIL_FROM` con el proveedor de correo.

2. Guardarraíles
- Ningún handler importa `mongo-driver` (test de arquitectura de `nv-02`).
- Claims (`jti`, `exp`) se extraen en el servicio; `exp` malformado → 401.
- Contraseña, `refresh_token` y tokens de enlace nunca en logs, errores ni caché.
- Redis caído: fail-closed para JWT y login (503).
- `critic B2` sigue vigente sin excepciones: la llave de revocados es `jwt_revocado:{jti}` (constitución §6).

3. Pruebas
- Las de la spec §Tests. El servicio se prueba con MongoDB y Redis reales (Testcontainers), sin simular la base, en `tests/integration/auth_service_test.go` para que Unit siga sin Docker. Solo se simula el `Mailer` (las llaves de Google, con T7).

4. Secuencia
1. Config, `Mailer`, `usuarios` mínimo.
2. Tokens (JWT, refresh) y contraseñas.
3. `sesiones` + login + refresh + logout + sesiones.
4. Bloqueo por intentos.
5. Recuperar/restablecer y verificar correo.
6. Cambio de contraseña.
7. Google (tras D-1).
8. Filas de docs, gates y `Memoria.md`.

5. Verificación
- `go build ./... && go vet ./...`
- `go test ./internal/... ./tests/... -count=1 → verdes`
- `scripts/critic-guardrails.sh → PASS` · `scripts/check_endpoints.sh → OK` · `scripts/check_sdd.sh → SDD-OK`
- `scripts/check_coverage.sh --min 45 → OK` · `semgrep scan --config=auto --config=.semgrep/semgrep.yaml --error → 0`
