# gates.md — NV-94 Restablecer la contraseña con código

Evidencia local del 2026-11-26, en Docker `golang:1.27.3`. Las suites completas las corre el CI del PR.

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1–B4 | Sin secretos; rutas en `docs/endpoints.md` | `critic-guardrails: PASS`; `check_endpoints: OK (286 rutas)` | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | `go build ./...` y `go vet ./...` sin salida; golangci-lint v2.15.0: `0 issues.` | ✅ |
| Unit | Validación, tope por tipo y plantilla | `go test ./internal/... ./cmd/...` verde: `TestResetRequest_CorreoYCodigo`, `TestCodesPerDay` (auth), `TestRender_PasswordResetShowsTheCodeWithoutLink` (mailer) | ✅ |
| Integration | Feliz, malo, correo sin cuenta, un solo uso, 423, vencido, tope diario; Mailpit | `-run 'TestAuth_RecoverThenReset\|TestAuth_ResetCodeLocksAfterFiveAttempts\|TestAuthService_ExpiredResetCodeIs400\|TestAuthService_ResetCodesPerDay\|TestAuthService_ResendBefore60sIs429\|TestAuth_VerifyEmail\|TestAuthService_VerificationCode\|TestMailer_Recover\|TestMailer_RegisterSendsVerification'` ok (64,3 s) | ✅ |
| Security | Sin código, contraseña ni enlace en logs; el correo trae el código | `-run 'TestAuthSecurity\|TestMailerSecurity'` ok (68,5 s) | ✅ |
| Database | Índices nuevos (si los hay) | Sin índices nuevos | ✅ N/A |
| Contract | Forma de `restablecer` | `TestContract_Auth` ok (31,5 s): `auth/restablecer_200` con el cuerpo nuevo, misma respuesta, sin recaptura | ✅ |
| SDD | `check_sdd` OK y `Memoria.md` al día | `SDD-OK`; `Memoria.md` con la sección NV-94 | ✅ |
