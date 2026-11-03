# Endpoints — vivo (nv-01, nv-04)

> Fuente de contratos. Las specs solo enlazan, no duplican. Constitución §6.
> Las filas de módulos de negocio se agregan con su spec (`nv-04+`), copiadas de `Wherever_Endpoints_CRUD.xlsx` (hojas A1–A7, J1–J9).

| Método | Ruta | Auth | Rate | Códigos | Spec |
|---|---|---|---|---|---|
| GET | `/health` → `{estado:"ok", version}` | no | global | 200 | nv-01 |
| GET | `/ready` → `{estado, dependencias}` (MongoDB, Redis, Neo4j e `indices_mongodb`; 1 s c/u; Redis caído = 200 `degraded`; falta un índice declarado = 503 hasta correr `scripts/indexes`) | no, red interna | global | 200/503 | nv-01, nv-04 |
| GET | `/api/v1/ping` | no | global | 200 | nv-01 |
| GET | `/api/v1/probe/timeout` (no-prod) | no | global | 408/404 | nv-01 |
| GET | `/api/v1/probe/error` (no-prod) | no | global | 500/404 | nv-01 |
| GET | `/api/v1/probe/forbidden` (no-prod) | no | global | 403/404 | nv-01 |
| GET | `/api/v1/probe/conflict` (no-prod) | no | global | 409/404 | nv-01 |
| GET | `/api/v1/probe/invalid` (no-prod) | no | global | 422/404 | nv-01 |
| GET | `/docs` y `/openapi.json` (solo dev) | no | — | 200/404 | nv-01 |
| POST | `/api/v1/auth/login` → `{correo, password, dispositivo{modelo, sistema, app_version}}` → tokens. Argon2id; 5 fallos del mismo correo (exista o no) → 423 con minutos restantes | no | login 5/5 min por IP | 200/400/401/422/423/429/503 | nv-04 (J1) |
| POST | `/api/v1/auth/refresh` → `{refresh_token}` → tokens nuevos; rota el refresh y uno ya rotado revoca la familia | refresh_token | 10/min por IP | 200/400/401/422/429/503 | nv-04 (J1) |
| POST | `/api/v1/auth/logout` → cierra la sesión actual (`revocada_en` + `jwt_revocado:{jti}`) | JWT | global | 204/401/503 | nv-04 (J1) |
| POST | `/api/v1/auth/logout-todos` → cierra todas las sesiones del usuario | JWT | global | 204/401/503 | nv-04 (J1) |
| GET | `/api/v1/auth/sesiones` → `[{id, dispositivo, ultimo_uso, actual}]` (solo abiertas y no vencidas) | JWT | global | 200/401 | nv-04 (J1) |
| DELETE | `/api/v1/auth/sesiones/{id}` → cierra una sesión propia; ajena o inexistente = 404 | JWT, dueño | global | 204/401/404 | nv-04 (J1) |
| POST | `/api/v1/auth/recuperar` → `{correo}`; enlace de 1 h; siempre 202 exista o no el correo; no reenvía antes de 60 s | no | 10/min por IP | 202/400/422/429 | nv-04 (J1) |
| POST | `/api/v1/auth/restablecer` → `{token, password_nuevo, dispositivo?}` → tokens; enlace de un solo uso; cierra las otras sesiones | token del enlace | 10/min por IP | 200/400/422/429 | nv-04 (J1) |
| POST | `/api/v1/auth/verificar-correo` → `{token}` → `{correo_verificado: true}`; enlace de 24 h, un solo uso | token del enlace | 10/min por IP | 200/400/422/429 | nv-04 (J1) |
| POST | `/api/v1/auth/reenviar-verificacion` → reenvía el enlace (máx. 1 cada 60 s, `Retry-After`) | JWT | global | 202/401/429 | nv-04 (J1) |
| PUT | `/api/v1/auth/password` → `{password_actual, password_nuevo}`; cierra las otras sesiones | JWT | global | 204/400/401/422 | nv-04 (J1) |

Convención JSON: `snake_case`. `PageResult{items, siguiente_cursor, limite}` (desde nv-02). Errores `{codigo, mensaje, campos[], detalle?, traza_id}`.

Tokens (nv-04): `{access_token, refresh_token, expira_en, usuario{id, nombre, correo, correo_verificado, roles}}`; `expira_en` = vencimiento del access token (ISO 8601 UTC, 15 min); refresh opaco de 30 días. Contraseña nueva: 8 a 128 caracteres con letra y número (422 si no). Cuerpo ilegible = 400.

## Diferidos (sin ruta en código todavía)
- `POST /api/v1/auth/google` (`{id_token, dispositivo{}}` → 200 con tokens, 201 si es cuenta nueva) — nv-04 T7, espera la decisión D-1 (cuenta sin `fecha_nacimiento`) y la excepción de stack `google.golang.org/api/idtoken`. Hoja J1.
- `GET /metrics` (Prometheus, red interna) — spec de observabilidad. Hoja J9.
- `POST /api/v1/errores` y `GET /api/v1/admin/errores` (colección `errores_app`) — spec de observabilidad. Hoja J9.
