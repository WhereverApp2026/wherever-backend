# Checklist — nv-04 Autenticación, sesiones y recuperación de contraseña (J1)

> Trazabilidad: `spec R1-R8 ↔ plan §3-§5 ↔ task T1-T8`. Estado: Aprobada (2026-11-02, Osmar Reséndiz). T7 (Google) diferida.

- [x] Tokens: JWT HS256 de 15 min con `sub, sid, jti, roles, iss, aud` y refresh opaco de 30 días (R1) — `internal/auth/tokens_test.go`, `TestAuthService_LoginOK`
- [x] Sesiones por dispositivo, cierre individual y total (R2) — `TestAuth_ActiveSessionsMarkCurrent`, `TestAuth_RevokeSessionOnlyOwn`, `TestAuthService_LogoutAllClosesEverySession`
- [x] Reutilización de refresh revoca la familia; `refresh_hash_previo` + índice (R3) — `TestAuthService_ReusedRefreshRevokesFamily`, `TestAuth_ReusedRefreshForcesNewLogin`
- [x] Login con Argon2id, hash falso, bloqueo 5→15 min, sin enumeración (R4) — `password_test.go`, `TestAuthService_FifthFailureLocks`, `TestAuthSecurity_LoginDoesNotRevealUnknownEmail`
- [x] Recuperar/restablecer con enlace de 1 h, un solo uso, reenvío 60 s (R5) — `TestAuth_RecoverThenReset`, `TestAuthService_ExpiredLinkIs400`, `TestAuthSecurity_RecoverRespondsTheSameWithOrWithoutAccount`
- [x] Verificar correo y reenviar (R6) — `TestAuth_VerifyEmail`, `TestAuthService_ResendBefore60sIs429`
- [x] Cambiar contraseña cierra las otras sesiones (R7) — `TestAuthService_ChangePasswordClosesOtherSessions`, `TestAuth_ChangePasswordEndpoint`
- [ ] Google con `id_token` validado (R8) — **diferida**: D-1 sin aprobar; la fila queda en "Diferidos" de `docs/endpoints.md`
- [x] Redis caído: 503 en JWT y login; limitador en memoria — `TestAuth_RedisDownThenRecovered` (contenedor detenido de verdad y vuelto a levantar)
- [x] MongoDB caído: 503 en las rutas de auth — `TestAuth_MongoDownIs503`
- [x] Sin índices obligatorios `/ready` da 503 (la API no los crea) — `TestReady_MissingIndexesIs503UntilScriptRuns`
- [x] Ningún secreto ni token en logs (critic B2) — `TestAuthSecurity_NoTokensOrPasswordsInLogs`, `TestLog_NeverWritesTokenOrFullAddress`
- [x] 11 filas en `docs/endpoints.md` (+ Google en Diferidos); `check_endpoints.sh → OK` (21 rutas)
- [x] `go build`, `go vet`, suites, cobertura ≥45 %, semgrep 0
- [x] Dos instancias: logout en una invalida el JWT en la otra — `TestAuth_LogoutInOneInstanceRevokesInTheOther`
- [x] `Memoria.md` con D-1, excepción `idtoken` (pendiente con T7) y deuda del proveedor de correo
- [x] Aprobación firmada: Osmar Reséndiz, 2026-11-02 (T7 fuera de la aprobación)

## Evidencia (2026-11-02, Go 1.27.3, Docker 29.8.1, Windows + Git Bash)
- `go build ./...` y `go vet ./...` sin errores; `golangci-lint@v2.15.0 run ./...` → 0 issues.
- Unit `go test ./internal/... ./cmd/... -count=1` → 62 PASS (nuevos: tokens 6, password 4, mailer 3, validación 2, `RequireAuth` 1, config 3 casos).
- Integration `-p 1` → 42 PASS (14 del service de auth, 13 HTTP de auth, `/ready` sin índices, 14 de nv-01) con MongoDB 7.0.14, Redis 7.4.1 y Neo4j 5.26.0 reales.
- Security `-p 1` → 16 PASS (8 de auth + 8 de nv-01). Database `-p 1` → 4 PASS. Contract → 17 capturas en `tests/contract/testdata/auth/` validadas sin `CONTRACT_CAPTURE`.
- Cobertura (perfiles de las cinco suites unidos, como hace `ci-pr.yml`; medidos en `golang:1.27.3` porque Control de aplicaciones de Windows bloquea `cover.exe` local): 83,7 % (≥ 45 %). Solo Unit: 36,4 % (no es lo que mide el gate).
- Delta contra `origin/main` (advisory): 81,4 % (726/892 líneas).
- Mutación (límite conocido: go-mutesting solo muta archivos con `_test.go` del mismo nombre; de los 18 de nv-04 mide 7 y nunca `service.go`). Línea base: score 0,62 (171 muertos de 274, 17 duplicados) en 31 min en Windows. Con el alcance de PR (`internal/auth/{service,tokens}.go`, en copia temporal): 0,79 (38/48) en 3 min 43 s; solo cuenta `tokens.go`, porque go-mutesting salta `service.go` al no tener `service_test.go`. La completa queda en `nightly.yml`. Después, `go build ./...` OK y el árbol sin cambios.
- `critic-guardrails` PASS, `check_endpoints` OK (21 rutas), `check_sdd` SDD-OK (6 specs), `semgrep 1.95.0` → 0, `yaml.safe_load` OK.
- `scripts/indexes` contra el compose de desarrollo, dos veces seguidas → `indexes: OK`; `usuarios` (3), `sesiones` (6) y `tokens_un_uso` (4) con los nombres de J1/A1.
- Binario real (staging, compose en puertos alternos): login con correo inexistente 401 ×4 → 423 al 5.º → 429 al 6.º (límite por IP); `recuperar` 202; JSON roto 400; `sesiones` sin token 401; `/ready` 200; el log no contiene la contraseña ni la clave JWT.
