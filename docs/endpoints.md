# Endpoints — vivo (nv-01, nv-04, nv-05)

> Fuente de contratos. Las specs solo enlazan, no duplican. Constitución §6.
> Las filas de módulos de negocio se agregan con su spec (`nv-04+`), copiadas de `Wherever_Endpoints_CRUD.xlsx` (hojas A1–A7, J1–J9).

| Método | Ruta | Auth | Rate | Códigos | Spec |
|---|---|---|---|---|---|
| GET | `/health` → `{estado:"ok", version}` | no | — | 200 | nv-01, nv-11 |
| GET | `/ready` → `{estado, dependencias}` (MongoDB, Redis, Neo4j e `indices_mongodb`; 1 s c/u; Redis caído = 200 `degraded`; falta un índice declarado = 503 hasta correr `scripts/indexes`) | no; también público en staging, sin direcciones | global | 200/503 | nv-01, nv-04 |
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
| POST | `/api/v1/usuarios` → `{nombre, fecha_nacimiento (AAAA-MM-DD), correo, password, acepta_terminos, version_terminos, promociones_correo, dispositivo?}` → 201 con tokens; 18+ por fecha UTC, rol `consumidor`, 50 puntos, manda verificación | no | registro 5/h por IP | 201/400/409/422/429 | nv-05 (A1) |
| GET | `/api/v1/usuarios/correo-disponible` con `?correo=` → `{disponible}` (usa `ux_correo`) | no | 20/min por IP | 200/400/422/429 | nv-05 (A1) |
| GET | `/api/v1/usuarios/yo` → perfil completo (sin `password_hash`, `telefono_hash` ni `google_id`) con `version` y `ETag` | JWT | global | 200/401 | nv-05 (A1) |
| PATCH | `/api/v1/usuarios/yo` → `{nombre?, foto_url? (https), ciudad_id?}` con `If-Match: "<version>"`; sin `If-Match` 400, versión vieja 409 | JWT, dueño | global | 200/400/401/409/422 | nv-05 (A1) |
| PUT | `/api/v1/usuarios/yo/correo` → `{correo_nuevo, password}`; queda sin verificar y manda verificación | JWT, dueño | global | 200/400/401/409/422 | nv-05 (A1) |
| PUT | `/api/v1/usuarios/yo/gustos` → `{categorias[3..10], presupuesto_min, presupuesto_max, restricciones[], alergias}`; `alergias` no vacío exige `privacidad.datos_sensibles` (422 si no, US-010); no suma puntos (se dan al registrarse) | JWT consumidor | global | 200/400/401/403/422 | nv-05 (A1) |
| PATCH | `/api/v1/usuarios/yo/preferencias` → parcial `{movilidad_default, tema, unidades, horario_silencioso{desde, hasta}, notificaciones{}}`; `reservas: false` = 422 | JWT, dueño | global | 200/400/401/422 | nv-05 (A1) |
| PATCH | `/api/v1/usuarios/yo/privacidad` → parcial `{ubicacion, aprendizaje_personalizado, estado_animo, datos_sensibles, datos_uso, buscable_por_telefono, promociones_correo, version_terminos}` | JWT, dueño | global | 200/400/401/422 | nv-05 (A1) |
| PUT | `/api/v1/usuarios/yo/telefono` → `{telefono}`; se quitan espacios, guiones, puntos y paréntesis y debe quedar E.164 (`+` y lada del país); se guarda solo su HMAC-SHA-256; repetido = 409 | JWT, dueño | global | 200/400/401/409/422 | nv-05 (A1) |
| POST | `/api/v1/usuarios/yo/modo` → `{modo: explorar|negocio}`; negocio sin rol `comerciante` = 403 | JWT | global | 200/400/401/403/422 | nv-05 (A1) |
| POST | `/api/v1/usuarios/yo/exportar` → 202; una por día (429 con `Retry-After`); la arma el trabajo nocturno y manda un enlace de 24 h (D-2) | JWT, dueño | global | 202/401/429 | nv-05 (A1) |
| POST | `/api/v1/usuarios/exportaciones/descargar` → `{token}` → el JSON de "mis datos"; enlace de un solo uso (D-2); al descargarlo el archivo se borra de la base | token del enlace | 10/min por IP | 200/400/410/422/429 | nv-05 (A1) |
| DELETE | `/api/v1/usuarios/yo/historial` → borra el historial de todos los módulos registrados; conserva la cuenta | JWT, dueño | global | 204/401/503 | nv-05 (A1) |
| DELETE | `/api/v1/usuarios/yo` → `{confirmacion: "ELIMINAR"}` → 202 `{eliminar_en}` (hoy + 30 días); cierra todas las sesiones | JWT, dueño | global | 202/400/401/422 | nv-05 (A1) |
| POST | `/api/v1/usuarios/yo/cancelar-eliminacion` → perfil; fuera de plazo o sin eliminación pendiente = 409 | JWT, dueño | global | 200/401/409 | nv-05 (A1) |
| GET | `/api/v1/ciudades/autocompletar` con `?q=&limite=` → `[{id, nombre, estado}]`; `q` de 2 letras o más, sin acentos; `limite` 8 por defecto, máx. 50; primero las de cobertura | no | global | 200/400/422 | nv-05 (A1) |

Convención JSON: `snake_case`. `PageResult{items, siguiente_cursor, limite}` (desde nv-02). Errores `{codigo, mensaje, campos[], detalle?, traza_id}`.

Tokens (nv-04): `{access_token, refresh_token, expira_en, usuario{id, nombre, correo, correo_verificado, roles}}`; `expira_en` = vencimiento del access token (ISO 8601 UTC, 15 min); refresh opaco de 30 días. Contraseña nueva: 8 a 128 caracteres con letra y número (422 si no). Cuerpo ilegible = 400.

Sondas y `detalle` (nv-15): solo fuera de prod y sin `HIDE_INTERNALS=true`. El servidor público de AWS los oculta: `/probe/*` da 404 y los errores no traen `detalle`.

Consentimiento (nv-15): al retirar `datos_sensibles` se borran las alergias guardadas.

Concurrencia (nv-05): `GET /usuarios/yo` y cada cambio devuelven `version` y `ETag`. `PATCH /usuarios/yo` exige `If-Match: "<version>"`: sin la cabecera = **400**; con una versión vieja = **409**, que la app debe mostrar como "tus datos cambiaron en otro dispositivo, recarga" y volver a pedir `GET /usuarios/yo` (la constitución §5 no usa 412 ni 428).

Cuenta en eliminación pendiente (nv-05): puede iniciar sesión, pero con sesión solo responden `GET /usuarios/yo`, `POST /usuarios/yo/cancelar-eliminacion`, `POST /auth/logout` y `POST /auth/logout-todos`; las demás rutas con JWT dan **403** hasta cancelar.

## Diferidos (sin ruta en código todavía)
- `POST /api/v1/auth/google` (`{id_token, dispositivo{}}` → 200 con tokens, 201 si es cuenta nueva) — nv-04 T7, espera la decisión D-1 (cuenta sin `fecha_nacimiento`) y la excepción de stack `google.golang.org/api/idtoken`. Hoja J1.
- `GET /metrics` (Prometheus, red interna) — spec de observabilidad. Hoja J9.
- `POST /api/v1/errores` y `GET /api/v1/admin/errores` (colección `errores_app`) — spec de observabilidad. Hoja J9.
