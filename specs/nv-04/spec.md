# NV-04 — Autenticación, sesiones y recuperación de contraseña (J1)

## Contexto
Primer módulo de negocio. Controla quién entra y con qué permisos. Origen: US-158 y hoja J1 del Excel de endpoints. Responsable de la hoja: Jorge Alberto Rodríguez Enriquez. Pantallas: P-07, P-08, P-63 y verificación de correo. **Depende de**: NV-00, NV-01, NV-02.

## Requisitos
1. Emitir y renovar tokens: `access_token` JWT HS256 de 15 min (`sub, sid, jti, roles, iss, aud`) y `refresh_token` opaco de 30 días con rotación en cada uso.
2. Sesiones por dispositivo en `sesiones`; listar, cerrar una, cerrar la actual y cerrar todas.
3. Detección de reutilización: un refresh ya rotado revoca toda la `familia_id` y obliga a iniciar sesión de nuevo.
4. Login con correo y contraseña (Argon2id), con bloqueo de 15 min tras 5 fallos del mismo correo, **exista o no la cuenta** (anti-enumeración).
5. Recuperar contraseña (enlace de 1 h, un solo uso, reenvío cada 60 s, respuesta idéntica exista o no el correo) y restablecer.
6. Verificar correo con enlace de un solo uso y reenviar la verificación (1 cada 60 s).
7. Cambiar contraseña conociendo la actual; cierra las demás sesiones.
8. Entrar con Google (`id_token`), creando la cuenta si el correo no existe (ver D-1).

## Diseño
- Módulo `internal/auth/{handler,service,repository,dto}` y `internal/usuarios/repository` mínimo (solo lo que auth necesita: buscar por correo o id, crear desde Google, fijar contraseña, marcar correo verificado). El resto de `usuarios` llega en `nv-05`.
- **Colección `sesiones`** (Excel J1 §1) con dos cambios: se agrega `refresh_hash_previo` (string, no obligatorio) y el índice `ix_refresh_previo { refresh_hash_previo: 1 }`. Sin él no se puede distinguir un token reutilizado de uno inventado, porque `refresh_hash` solo guarda el vigente. También se agrega `jti_vigente` (ya en la hoja J1 v3): al cerrar una sesión desde otro dispositivo (o todas, o por reutilización) se revoca su access token vigente en `jwt_revocado:{jti}`, no solo el del que pide. Índices: `ux_refresh_hash`, `ix_refresh_previo`, `ix_usuario_sesiones`, `ix_familia`, `ttl_sesion`.
- **Colección `tokens_un_uso`** (Excel J1 §2): índices `ux_token_hash`, `ix_usuario_tipo`, `ttl_token`. Tipos: `recuperar_password` (1 h) y `verificar_correo` (24 h).
- **Colección `usuarios`**: se crea con los campos de la hoja A1 que usa auth (más `version`, constitución §5) y solo estos índices: `ux_correo`, `ux_google_id` (parcial). El resto del esquema y los demás índices los crea `nv-05`.
- **Índices**: los declara cada módulo y los crea solo `scripts/indexes` (`make indexes`), idempotente (constitución §5); la API no crea índices al arrancar.
- **Tokens**: `refresh_token` = 32 bytes aleatorios (`crypto/rand`) en base64url; en base de datos solo su SHA-256. Los tokens de enlace igual. La contraseña se compara con Argon2id 64 MB / 3 iteraciones; con correo inexistente se compara contra un hash falso para igualar tiempos.
- **Redis** (nombres de la constitución §6, iguales a los del Excel J1): `jwt_revocado:{jti}` (TTL = vida remanente del JWT) y `login_fallos:{correo}` (TTL 15 min; con 5 fallos la llave misma es el bloqueo y su TTL da los minutos restantes). Si Redis no responde: el JWT se rechaza con 503 y el login falla cerrado con 503; el limitador sigue en memoria por instancia.
- **Rate limit** (antes de Auth): login 5 por 5 min por IP, como en `docs/rate-limit-matrix.md`; `google`, `refresh`, `recuperar`, `restablecer` y `verificar-correo` 10 por min por IP (valor provisional, se mide antes de endurecer).
- **Correo**: interfaz `Mailer` en `internal/platform/mailer`. En pruebas, un buzón en memoria. En desarrollo, un `LogMailer` que registra solo destinatario y tipo, nunca el enlace ni el token.
- **Google**: validación del `id_token` contra las llaves públicas de Google (`aud` = `GOOGLE_CLIENT_ID`, `iss` de Google, `email_verified = true`).
- Errores con `ErrorResponse` de `nv-01`: 400, 401, 403, 404, 422, 423, 429, 503.

## Contratos
Doce endpoints. Detalle por fila en `docs/endpoints.md` (única fuente); esta tabla solo los lista.

| Método | Ruta | Auth | Códigos |
|---|---|---|---|
| POST | `/api/v1/auth/login` | no | 200, 401, 422, 423, 429, 503 |
| POST | `/api/v1/auth/google` | no | 200, 201, 401, 422, 429 |
| POST | `/api/v1/auth/refresh` | refresh_token | 200, 401, 422, 429 |
| POST | `/api/v1/auth/logout` | JWT | 204, 401 |
| POST | `/api/v1/auth/logout-todos` | JWT | 204, 401 |
| GET | `/api/v1/auth/sesiones` | JWT | 200, 401 |
| DELETE | `/api/v1/auth/sesiones/{id}` | JWT, dueño | 204, 401, 404 |
| POST | `/api/v1/auth/recuperar` | no | 202, 422, 429 |
| POST | `/api/v1/auth/restablecer` | token del enlace | 200, 400, 422, 429 |
| POST | `/api/v1/auth/verificar-correo` | token del enlace | 200, 400, 422, 429 |
| POST | `/api/v1/auth/reenviar-verificacion` | JWT | 202, 401, 429 |
| PUT | `/api/v1/auth/password` | JWT | 204, 401, 422 |

Respuesta de tokens: `{access_token, refresh_token, expira_en, usuario}`. Cuerpos en `snake_case`.

## Tests
- `Unit/Auth service 14`: login ok, contraseña mala, correo inexistente (mismo tiempo y mismo 401), bloqueo al 5.º fallo, desbloqueo al vencer, rotación, reutilización revoca familia, refresh vencido, logout agrega `jti`, logout-todos, cambio de contraseña cierra otras sesiones, token de enlace usado, token de enlace vencido, reenvío antes de 60 s.
- `Unit/Tokens 6`: JWT con claims correctos, `alg:none` rechazado, firma ajena rechazada, vencido rechazado, hash de refresh determinista, token aleatorio sin repetición.
- `Unit/Password 4`: Argon2id verifica, hash distinto por sal, política 8 caracteres con letra y número, hash falso con costo igual.
- `Integration/Auth 12` con MongoDB y Redis reales: flujo login→refresh→logout, reutilización de refresh, sesiones activas con `actual`, cierre de sesión ajena (404 si no es suya), recuperar→restablecer, verificar correo, TTL de `sesiones` y `tokens_un_uso`, bloqueo 423 con minutos restantes, Redis caído (503 en login y en JWT), Redis recuperado.
- `Integration/Google 3`: cuenta nueva 201, cuenta existente 200, `id_token` inválido 401 (con las llaves de Google sustituidas por un par de prueba firmado localmente; es la única frontera externa simulada).
- `Security/Auth 8`: `recuperar` responde igual con y sin cuenta, `login` no distingue correo inexistente, ningún token ni contraseña en logs (critic B2), enlace de un solo uso, cabeceras de seguridad en `/auth/*`, 429 con `Retry-After`, JWT revocado → 401, `Authorization` mal formado → 401.
- `Contract/Auth`: respuestas coinciden con la tabla de Contratos.

## Criterios
- `go build ./...` y `go vet ./...` sin errores; todas las suites de la spec en verde.
- Cada endpoint tiene su fila en `docs/endpoints.md` (`scripts/check_endpoints.sh → OK`).
- `scripts/critic-guardrails.sh → PASS` (sin tokens ni contraseñas en logs).
- Cobertura global ≥ 45 %; puerta advisory de `nv-03` informa.
- Con las dos instancias levantadas, un logout en una invalida el JWT en la otra.

## Límites
- **Decisión D-1 (Google sin fecha de nacimiento).** `usuarios.fecha_nacimiento` es obligatoria y Google no la entrega. Propuesta: la cuenta nueva por Google nace con `fecha_nacimiento` vacía y `perfil_completo = false`, y `nv-05` bloquea (403 `perfil_incompleto`) las acciones que exigen mayoría de edad hasta que `PATCH /usuarios/yo` la complete. Requiere el visto bueno del responsable de A1 y un ajuste a la hoja A1. Hasta que se decida, `/auth/google` se implementa en la última tarea.
- **Excepciones de stack** (constitución §3, regla de excepción de `nv-00`): `google.golang.org/api/idtoken` para validar Google. Se registra en `Memoria.md` con licencia (Apache-2.0) y mantenimiento.
- El proveedor real de correo (SMTP o servicio) queda como deuda: `nv-04` entrega la interfaz, el buzón de pruebas y el `LogMailer`.
- Sin roles de comerciante ni administrador usados todavía (llegan con sus módulos), sin 2FA, sin Apple/Facebook, sin captcha.
- Un refresh enviado dos veces en paralelo desde el mismo teléfono se leería como reutilización. La app móvil debe serializar la renovación (un solo `Authenticator` en vuelo); queda como requisito de la spec móvil, no se resuelve aquí.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (T1–T6 y T8 ejecutadas; T7 Google diferida por D-1; sin firma)
- **Revisores:** (pendiente de nombre; responsable de la hoja J1: Jorge Alberto Rodríguez Enriquez)
- **Fecha:** 2026-11-02 (ejecución)
- **Detalle:** evidencia en `specs/nv-04/checklists/gates.md` + `Memoria.md`. Desvíos:
  - Llaves de Redis `jwt_revocado:{jti}` y `login_fallos:{correo}` (constitución §6), no `blacklist:/attempts:/lockout:`.
  - Índices solo por `scripts/indexes` (constitución §5), no al arrancar.
  - `sesiones` agrega `jti_vigente` (como la hoja J1 v3; su vencimiento sale de `ultimo_uso` + vida del access) para revocar el access de las sesiones que se cierran desde otro dispositivo; al rotar el refresh también se revoca el access anterior.
  - Las 14 pruebas del service viven en `tests/integration/auth_service_test.go` (bases reales); Integration HTTP tiene 13 (se suma MongoDB caído → 503).
  - El TTL se prueba por la existencia de `ttl_sesion`/`ttl_token` con `expireAfterSeconds: 0` y porque el service rechaza lo vencido; no se espera el barrido de MongoDB (60 s).
  - `202` sin cuerpo; cuerpo ilegible = 400 en todas las rutas con cuerpo; `logout` puede dar 503 si Redis cae.
  - Sin `MAIL_FROM` ni `GOOGLE_CLIENT_ID` todavía (llegan con el proveedor de correo y con T7).
  - Para `nv-05`: el método que abre sesión se llama `auth.Service.CreateSession` (la spec de nv-05 dice `CrearSesion`) y el envío de verificación, `auth.Service.SendVerification`.
