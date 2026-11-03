# ASVS L2 Checklist — vivo

> Constitución §6. Trazable a `nv-01`, `nv-04`, `nv-05` y `nv-06`.

- [x] V2 Auth: JWT HS256 con clave ≥32B por env, leeway 0, métodos `[HS256]`, claims `sub/sid/jti/roles/iss/aud` (nv-04: `internal/auth/tokens.go`, `middleware.AccessParser`, `internal/auth/tokens_test.go`)
- [x] V2 Auth: access 15 min + refresh 30 días con rotación, hash SHA-256 y revocación por familia + `jwt_revocado:{jti}` en Redis (fail-closed) (nv-04: `tests/integration/auth_*_test.go`)
- [x] V2 Login: hash falso anti-enumeración, bloqueo 5→15 min (`423`) fail-closed, contador también para correos inexistentes (nv-04: `tests/security/auth_security_test.go`)
- [x] V2 Credenciales: Argon2id 64 MB / 3 iteraciones, política 8+ con letra y número, enlaces de un solo uso (1 h / 24 h) guardados como SHA-256, recuperar sin revelar cuentas (nv-04)
- [x] V3 Sesión: stateless multi-instancia, sin sesión en proceso (nv-01: solo el respaldo del rate limit vive en memoria); cierre de una, de todas y al cambiar contraseña revoca también el access vigente de cada sesión (nv-04)
- [x] V4 Acceso: denegar por defecto (`RequireAuth`), mínimo privilegio, 403/404 correctos; los datos propios solo por `/yo` con el `sub` firmado, sin ids de otra persona en la ruta (nv-04, nv-05: `tests/security/usuarios_security_test.go`)
- [x] V5 Validación: validator en la frontera (registro único de reglas), objetos en lugar de texto → 400, búsqueda por prefijo escapada, sin secretos en código/logs/errores (nv-02, nv-05)
- [x] V8 Datos: teléfono solo como HMAC-SHA-256 con llave por entorno, respuestas sin hashes, "mis datos" por enlace de un solo uso de 24 h y no por correo (D-2), borrado a 30 días (nv-05)
- [x] V7 Logs: `slog` JSON + `request_id`, sin datos personales, filtro de secretos (nv-01: `internal/platform/logger`)
- [x] V9 Comunicaciones: HSTS 365d no-dev, CORS 1 origen, CSP, nosniff/DENY (nv-01: `tests/security`); correo saliente con STARTTLS o TLS (prod no arranca con `SMTP_TLS=none`), sin autenticación SMTP por canal sin cifrar (nv-06)
- [x] V2/V3 Enlaces de un solo uso: token en el fragmento (`#token=`), nunca en la ruta ni en la consulta; plantillas con `html/template`; CR/LF rechazados en encabezados; ningún token, enlace ni cuerpo en logs (nv-06: `tests/security/mailer_security_test.go`)
- [x] V10 Código: `gosec` y `errcheck` como error, `gocyclo ≤ 10`, `detalle` solo no-prod (nv-01)
- [x] V14 Configuración: secretos fuera del repo, body 10MB, timeout 60s (nv-01: `config.Load`)
