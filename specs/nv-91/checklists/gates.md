# gates.md — NV-91 Verificación de correo con código

Evidencia local del 2026-11-25, en Docker `golang:1.27.3`. Las suites completas las corre el CI del PR.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1–B4 | Sin secretos; rutas en `docs/endpoints.md` | `critic-guardrails: PASS`; `check_endpoints: OK (285 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `go build ./...` y `go vet ./...` sin salida; golangci-lint v2.15.0: `0 issues.` | ✅ |
| Unit | Código, HMAC, validación y plantilla | `go test ./internal/... ./cmd/...` verde: `codigo_test.go`, `dto_test.go` (auth), `TestRender_VerifyEmailShowsTheCodeWithoutLink` (mailer) | ✅ |
| Integration | Feliz, malo, vencido, 423, reenvío y tope diario; Mailpit | `-run 'TestAuth_VerifyEmail\|TestAuth_ResendVerification\|TestAuthService_VerificationCode\|TestMailer_RegisterSendsVerification\|TestMailer_ChangeEmail'` ok (65,0 s) | ✅ |
| Security | 401/403 y código ajeno | `-run 'TestAuthSecurity\|TestMailerSecurity'` ok (51,3 s) | ✅ |
| Database | Índices nuevos (si los hay) | Sin índices nuevos | ✅ N/A |
| Contract | Formas de `verificar-correo` | `TestContract_Auth` ok (25,3 s): `auth/verificar_correo_200` y `_400` con el cuerpo nuevo, mismas formas, sin recaptura | ✅ |
| SDD | `check_sdd` OK y `Memoria.md` al día | `SDD-OK (92 specs)`; `Memoria.md` con la sección NV-91 | ✅ |
