# Checklist — nv-06 Correo saliente

> Trazabilidad: `spec R1-R7 ↔ plan §3-§5 ↔ task T1-T8`. Estado: Aprobada (2026-11-03, Osmar Reséndiz).

- [x] Adaptador SMTP con la librería estándar; proveedor solo por configuración (R1) — `internal/platform/mailer/smtp.go`, `TestMailer_*` con Mailpit
- [x] Ninguna petición espera al servidor de correo; un fallo no cambia la respuesta (R2) — `TestMailer_SlowOrDownServerDoesNotChangeResponses`, `TestAsync_FullQueueDropsWithoutBlocking`
- [x] Tres plantillas en español, texto y HTML, con el nombre escapado (R3) — `TestRender_*`, `TestMailerSecurity_TemplatesEscapeHTML`
- [x] `APP_LINK_BASE` obligatorio `https://` en producción; token en el fragmento `#token=`; sin esquema propio (R4) — `TestLinks_TokenGoesInTheFragment`, `TestLoad_ProdMailRules`
- [x] Mailpit en el compose con perfil `dev`, solo en `127.0.0.1` (R5) — `make mail-dev`
- [x] Exportación con `aviso_enviado_en`, reintento con token nuevo (el anterior se invalida), tope de 72 h y 24 h desde el aviso (R6) — `TestMailer_ExportNotice*`, `TestExport_*`
- [x] Sin token, enlace ni cuerpo en logs, tampoco con el servidor caído (R7) — `TestMailerSecurity_NoTokenLinkOrBodyInLogs`
- [x] Respuesta idéntica y rápida para correo existente e inexistente — `TestMailer_RecoverForUnknownEmailSendsNothing`
- [x] CR/LF rechazados en encabezados — `TestBuildMIME_RejectsHeaderInjection`, `TestMailerSecurity_HeaderInjectionRejected`
- [x] Producción falla al arrancar sin `SMTP_HOST`, con `SMTP_TLS=none` o con `APP_LINK_BASE` sin `https://` — `TestLoad_ProdMailRules`; `SMTP_USER` con `SMTP_TLS=none` falla en todo entorno
- [x] `.env.example` con las claves nuevas (incluido `MAIL_FROM`) y sin secretos — `TestEnvExample_*`
- [x] `go build`, `go vet`, suites, cobertura ≥45 %, semgrep 0, `critic → PASS`
- [x] `Memoria.md` con la deuda: proveedor, dominio (SPF/DKIM/DMARC), App Links, token en fragmento para la spec móvil, aviso de privacidad
- [x] Aprobación firmada: Osmar Reséndiz, 2026-11-03

## Evidencia (2026-11-03, Go 1.27.3, Docker 29.8.1, Windows + Git Bash; suites en `golang:1.27.3`)
- `go build ./...` y `go vet ./...` sin errores; `golangci-lint@v2.15.0 run ./...` → 0 issues.
- Unit → 90 PASS (nuevos: mailer 10, config 3).
- Integration → 83 PASS (nuevos: `Integration/Mailer` 7 con Mailpit real, `Integration/Exportación` 2 y servidor lento o caído con sus dos casos). Security → 30 PASS (nuevos: `Security/Mailer` 4). Database → 4. Contract → 3 (sin cambios de contrato).
- Cobertura unida de las cinco suites: 82,3 % (≥ 45 %).
- `critic-guardrails` PASS, `check_endpoints` OK (37 rutas, sin filas nuevas), `check_sdd` SDD-OK, `semgrep 1.95.0` → 0 (con una excepción anotada: `text/template` solo para la parte `text/plain`; el HTML usa `html/template`). Delta (advisory): 79,6 % (277/348 líneas).
- Servidor lento (acepta y no saluda) y servidor caído: registro 201 y `recuperar` 202, cada uno en menos de 500 ms.
- Manual (binario real, `docker compose --profile dev up` en puertos alternos): `/ready` 200; registro 201 en 0,08 s; Mailpit recibió "Confirma tu correo en Wherever" para la cuenta nueva; el log tiene `correo enviado` y no contiene el token, la contraseña ni el correo completo.
