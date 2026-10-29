# ASVS L2 Checklist — vivo

> Constitución §6. Trazable a `nv-01`.

- [ ] V2 Auth: JWT HS256 con clave ≥32B por env, leeway 0, métodos `[HS256]`, claims `sub/jti/roles`
- [ ] V2 Auth: access 15 min + refresh 30 días con rotación, hash SHA-256 y revocación por familia + `jwt_revocado:{jti}` en Redis (fail-closed)
- [ ] V2 Login: hash falso anti-enumeración, bloqueo 5→15 min (`423`) fail-closed, contador también para correos inexistentes
- [ ] V3 Sesión: stateless multi-instancia, sin sesión en proceso
- [ ] V4 Acceso: denegar por defecto, mínimo privilegio, 403/404 correctos
- [ ] V5 Validación: validator en la frontera, sin secretos en código/logs/errores
- [ ] V7 Logs: `slog` JSON + `request_id`, sin datos personales, filtro de secretos
- [ ] V9 Comunicaciones: HSTS 365d no-dev, CORS 1 origen, CSP, nosniff/DENY
- [ ] V10 Código: `gosec` y `errcheck` como error, `gocyclo ≤ 10`, `detalle` solo no-prod
- [ ] V14 Configuración: secretos fuera del repo, body 10MB, timeout 60s
