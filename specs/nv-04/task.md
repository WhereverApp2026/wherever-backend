# NV-04 — Autenticación, sesiones y recuperación de contraseña (J1)

## T1 — Base del módulo
- **Crear**: `internal/platform/mailer/*`, `internal/usuarios/repository.go`, índices `ux_correo` y `ux_google_id` (vía `scripts/indexes`), claves nuevas en config y `.env.example`.
- **Verificar**: build, `Config` actualizado (tests de `nv-00` siguen verdes).
- **No hacer**: endpoints, proveedor real de correo.

## T2 — Tokens y contraseñas
- **Crear**: emisión y validación de JWT, generador y hash de refresh, Argon2id con hash falso.
- **Verificar**: `Unit/Tokens 6`, `Unit/Password 4`.

## T3 — Sesiones, login, refresh, logout
- **Crear**: colección `sesiones` con sus 5 índices, `login`, `refresh` con rotación y reutilización, `logout`, `logout-todos`, `sesiones` (GET y DELETE), `jwt_revocado:{jti}`.
- **Verificar**: `Integration/Auth` (flujo, reutilización, sesiones).

## T4 — Bloqueo por intentos
- **Crear**: `login_fallos:{correo}` (contador y bloqueo en la misma llave), 423 con minutos restantes, fail-closed.
- **Verificar**: bloqueo al 5.º fallo, Redis caído → 503.

## T5 — Recuperar y verificar
- **Crear**: `tokens_un_uso` con 3 índices, `recuperar`, `restablecer`, `verificar-correo`, `reenviar-verificacion`.
- **Verificar**: un solo uso, vencimiento, reenvío a 60 s, respuesta idéntica.

## T6 — Cambio de contraseña
- **Crear**: `PUT /auth/password`, cierre de otras sesiones.
- **Verificar**: sesiones ajenas revocadas, la actual sigue.

## T7 — Google (condicionada a D-1)
- **Crear**: validación de `id_token`, alta de cuenta, `Integration/Google 3`.
- **Verificar**: 201 y 200, `id_token` inválido 401.

## T8 — Docs y gates
- **Modificar**: 12 filas en `docs/endpoints.md`, política de login en `rate-limit-matrix.md`, ítems V2 en `asvs-l2-checklist.md`.
- **Verificar**: `check_endpoints.sh`, `check_sdd.sh`, critic, cobertura, semgrep.
- **Actualizar**: `Memoria.md` con D-1, excepción de stack y mediciones.
