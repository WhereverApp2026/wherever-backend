# NV-72 — Entrar con Google

## Contexto
`POST /auth/google` es la única ruta de la hoja «J1 Autenticación» sin código: nv-04 la dejó en T7 esperando dos cosas que esta spec pone frente al revisor: la decisión **D-1** (Google no entrega `fecha_nacimiento` y la cuenta exige 18 años o más) y la **excepción de stack** `google.golang.org/api/idtoken`. Hoy está en «Diferidos» de `docs/endpoints.md` y en `Memoria.md` (nv-04 y nv-05). **Origen**: fila `POST /api/v1/auth/google` de «J1 Autenticación» y el campo `google_id` de «A1 Usuarios y perfil». **Depende de**: NV-04 (tokens, `sesiones`, `DeviceInput`, rate limit por IP, `ux_google_id` parcial), NV-05 (registro: `adult`, `acepta_terminos`, `version_terminos`, `codigo_amigo`, estados de cuenta), NV-19 (versión del aviso de privacidad) y NV-49 (`fecha_nacimiento` cifrada y `mayor_de_edad`).

## Requisitos
1. `POST /auth/google` con `{id_token, dispositivo}` valida el token con las llaves públicas de Google y entra: 200 con tokens si la cuenta ya existe.
2. Si el correo no tiene cuenta, la crea con la mayoría de edad comprobada y los términos aceptados, y responde 201 con tokens.
3. Una cuenta existente con el mismo correo se vincula a Google sin abrir la puerta a una cuenta preparada por otra persona.
4. Ningún `id_token`, correo ni `google_id` llega a los logs.

## Diseño
- **Paquete:** en `internal/auth` (handler, service y DTO de la ruta; `google.go` y `google_service.go`) más un `GoogleVerifier` (interfaz) cuya implementación real envuelve a `idtoken`. En las pruebas corre el verificador **real** contra un servidor local de llaves (`internal/testutil/googletest`): su cliente HTTP redirige la descarga de llaves de Google a ese servidor, que firma con un par RSA propio. Es la única frontera externa simulada, como pidió nv-04. `usuarios` suma `FindByGoogleID`, `LinkGoogle` y `NewConsumer` (la cuenta inicial, compartida con el registro de nv-05).
- **Validación del `id_token`:** firma con las llaves de Google (en caché según `Cache-Control` de Google, lo hace `idtoken`), `aud` igual a `GOOGLE_CLIENT_ID`, `iss` `accounts.google.com` o `https://accounts.google.com`, `exp` vigente, `email_verified = true` y `sub` presente. Cualquier falla = 401 uniforme (`no_autenticado`, mensaje «credenciales inválidas»), sin decir cuál. `idtoken` revisa firma, `aud` y `exp`; `iss`, `sub`, `email` y `email_verified` los revisa `identityFromClaims`. Google caído al bajar las llaves (error de red o respuesta distinta de 200, marcada por `keysTransport`) = 503. La descarga tiene plazo de 5 s porque `idtoken` no se lo pone.
- **Búsqueda de la cuenta:** primero por `google_id = sub` (`ux_google_id`); si no, por `correo` normalizado (`ux_correo`).
  - **Encontrada por `google_id`:** entra (200).
  - **Encontrada por correo, sin `google_id`:** se vincula (`google_id = sub`, `correo_verificado = true`) y entra (200). Si la cuenta **no** tenía el correo verificado, además se borra `password_hash` y se revocan sus sesiones: quien la registró con un correo ajeno pierde el acceso (ataque de cuenta preparada). La persona puede fijar contraseña después con `/auth/recuperar`.
  - **Encontrada por correo con otro `google_id`:** 409 `cuenta_vinculada` (no se cambia el vínculo).
  - **Estado de la cuenta:** mismas reglas que `POST /auth/login` (bloqueada o eliminada = 401 uniforme; en eliminación pendiente, lo mismo que login).
- **Cuenta nueva y D-1 — resuelta: camino B (revisor, 2026-11-18).** Se dejan los dos caminos como registro:
  - **A (propuesta de nv-04):** la cuenta nace sin `fecha_nacimiento` con `perfil_completo = false`, y nv-05 responde 403 `perfil_incompleto` en lo que exige mayoría de edad hasta que `PATCH /usuarios/yo` la complete. Pega en todos los módulos (cada uno debe revisar la marca) y deja cuentas a medias con datos de un posible menor.
  - **B (recomendado):** no se crea la cuenta sin los datos. Si el correo no existe y el cuerpo no trae `{fecha_nacimiento, acepta_terminos, version_terminos}`, responde 422 `registro_incompleto` con `faltantes[]` y **no guarda nada**. Para no romper el cuerpo uniforme de error (nv-01), `registro_incompleto` va en `mensaje` y los faltantes en `campos[]` (`{campo, mensaje: "falta"}`). La app pide la fecha y la aceptación y repite la llamada con el mismo `id_token` (vive 1 hora). Con los datos, se valida igual que el registro (`adult`: menor de 18 = 422; `acepta_terminos` debe ser `true`) y se crea la cuenta con `mayor_de_edad`, fecha cifrada (nv-49), `codigo_amigo`, preferencias por defecto y `google_id`; 201 con tokens. Ninguna cuenta existe sin la regla de 18 años, y no hay marca nueva que revisar.
- **Nombre y foto de la cuenta nueva:** `nombre` del claim `name` (recortado a 40; menos de 2 caracteres = la parte local del correo; si tampoco alcanza, «Viajero») y `foto_url` del claim `picture` solo si es `https://`. Sin `password_hash` (se puede fijar con `/auth/recuperar`).
- **Cuerpo:** `{id_token (requerido, hasta 4096), dispositivo{modelo, sistema, app_version} (requerido, el `DeviceInput` de nv-04), fecha_nacimiento?, acepta_terminos?, version_terminos?, promociones_correo?}`. Campo de más = 400; formato malo = 422.
- **Respuesta:** la misma de `POST /auth/login` (tokens y sesión con el dispositivo). 200 si entró, 201 si creó la cuenta.
- **Límite:** política `auth_google`, 10 por minuto por IP (`rate:auth_google:{ip}`), la misma familia «Auth enlaces» de `docs/rate-limit-matrix.md` que nv-04 ya anticipó.
- **Compresión y logs:** `/auth` ya está fuera de la compresión (lleva tokens). El logger no registra el cuerpo; el `id_token` tiene forma de JWT y el filtro de nv-64 lo oculta si llega a un texto. Nunca se registra `sub` ni el correo.
- **Excepción de stack — aprobada: `idtoken` (revisor, 2026-11-18).** `google.golang.org/api/idtoken` (Apache-2.0, mantenida por Google). Trae el módulo `google.golang.org/api` y sus dependencias (más tamaño del binario y más avisos de `govulncheck`). Alternativa: `github.com/coreos/go-oidc/v3` (Apache-2.0, más chica, también requiere excepción) o validar con `golang-jwt/jwt/v5` (ya está) y las llaves JWKS de Google con caché propia (sin dependencia nueva, pero más código de seguridad propio). Se eligió `idtoken` v0.316.0, registrada en `Memoria.md`. `govulncheck` no suma avisos por ella (los tres que reporta ya estaban: librería estándar 1.27.3, fasthttp y x/crypto), así que no hizo falta cambiar a `go-oidc`.
- **Variables:** `GOOGLE_CLIENT_ID` nueva en `configs/.env.example`, Parameter Store (`/wherever/dev/GOOGLE_CLIENT_ID`, String) y `config.go` (resuelta por constructor). Es el client ID del cliente OAuth **Web** del proyecto de Firebase: la audiencia de los `id_token` que piden Android e iOS. Opcional fuera de prod: sin ella el arranque avisa una vez y la ruta responde 503; en prod es obligatoria. **No** se agrega a las obligatorias de `deploy-env.sh`, para que su falta no tumbe el servidor (como pasó con `MAIL_FROM`). No es secreto (va en la app), pero no se hardcodea.

## Contratos
Fila del Excel (hoja «J1 Autenticación») copiada y corregida:

| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/auth/google` | no | 200, 201, 400, 401, 409, 422, 429, 503 |

- Excel: `{id_token, dispositivo{}}` → «200 con tokens o 201 si fue cuenta nueva». Corregido: con el camino B, la cuenta nueva necesita además `fecha_nacimiento`, `acepta_terminos` y `version_terminos` (422 `registro_incompleto` si faltan); 409 si el correo ya está vinculado a otra cuenta de Google.
- La fila salió de «Diferidos» y está en la tabla viva de `docs/endpoints.md` (nv-72).

## Tests
- `tests/integration/auth_google_test.go` (verificador con llaves locales): cuenta nueva 201 con datos completos, sin datos = 422 `registro_incompleto` y ningún documento creado, menor de 18 = 422, cuenta existente por `google_id` 200, vínculo por correo verificado 200 (conserva contraseña y sesiones), vínculo por correo sin verificar borra `password_hash` y revoca sesiones previas, otro `google_id` = 409, cuenta bloqueada = 401 sin vincular, Google caído = 503, 429 en la 11.ª por minuto.
- `tests/security/auth_google_security_test.go`: `id_token` con firma ajena, `aud` ajeno, `iss` ajeno, vencido, `email_verified=false`, sin `sub` y basura = 401 igual y sin cuenta creada; ningún `id_token`, correo ni `sub` en logs capturados; respuesta sin `google_id`; campo de más = 400.
- `tests/database/indexes_test.go`: sin cambios; ya verifica `ux_google_id` único parcial.
- `tests/contract/auth_google_contract_test.go`: 200, 201 y 422 `registro_incompleto`.
- Unit: `internal/auth/google_test.go` (verificador real contra llaves locales, cada falla = `ErrGoogleToken`, llaves caídas = `ErrGoogleUnavailable`, validators del cuerpo, nombre, foto, faltantes y 503 sin verificador); `TestLoad_GoogleClientID` en config; `/auth/google` sin compresión en `compress_test.go`.

## Criterios
- Ninguna cuenta existe sin mayoría de edad comprobada y términos aceptados (si se aprueba B), y una cuenta creada con un correo ajeno no conserva acceso tras el vínculo.

## Límites
- Sin Sign in with Apple ni otros proveedores.
- Sin desvincular Google desde la app (se agrega con su fila del Excel si aparece).
- Sin `perfil_completo` si se elige B.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (ejecutada 2026-11-18); se firma al fusionar con el CI verde
- **Revisores:** Osmar Reséndiz
- **Fecha:** 2026-11-18 (decisiones)
- **Decisiones del revisor (2026-11-18):** D-1 camino B; excepción de stack `google.golang.org/api/idtoken` (Apache-2.0); vincular por correo y, si no estaba verificado, borrar contraseña y revocar sesiones; 409 `cuenta_vinculada` si el correo ya tiene otro `google_id`; política `auth_google` de 10 por minuto por IP; `GOOGLE_CLIENT_ID` opcional fuera de prod (503 sin ella) y fuera de las obligatorias de `deploy-env.sh`.
- **Detalle:** evidencia en `specs/nv-72/checklists/gates.md` + `Memoria.md`.
