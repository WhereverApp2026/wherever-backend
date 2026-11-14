# NV-71 — Dispositivos por id y reglas de moderación por filtro

## Contexto
Cuatro rutas del Excel con `{id}` que el código no tiene porque nv-40 y nv-43 decidieron otra forma: (a) la hoja «A6» pone el token de FCM en la ruta (`/dispositivos/{fcm_token}`) y nv-40 lo movió al cuerpo para que no quede en el log de acceso; (b) la hoja «A7» modela una regla por filtro (`reglas_moderacion` con `filtro`, `umbral_cuarentena`, `umbral_rechazo`, `activa`) y nv-43 guardó un solo documento con umbrales globales. Esta spec concilia las dos sin romper lo que ya existe. **Origen**: filas `PUT`/`DELETE /dispositivos/{fcm_token}` de «A6 Notificaciones y sync» y `PATCH`/`DELETE /admin/moderacion/reglas/{id}` de «A7 Moderación». **Depende de**: NV-04 (`RequireRoles`, rol `administrador`), NV-05 (`RequireActive`, `preferencias.notificaciones`), NV-40 (`dispositivos`, `PUT`/`DELETE /dispositivos` con `{token}`), NV-41 (envío de push) y NV-43 (`moderacion_reglas` de un solo documento `_id: "vigentes"`, `Moderador`, `If-Match`, política `admin`).

## Requisitos
1. `PUT /dispositivos/{id}` ajusta los datos de un teléfono ya registrado (versión de la app y canales activos) sin mandar el token.
2. `DELETE /dispositivos/{id}` quita un teléfono de mi cuenta por su id.
3. `PATCH /admin/moderacion/reglas/{id}` ajusta los umbrales de un filtro o lo activa; `DELETE` lo desactiva.
4. Las rutas de nv-40 (`PUT`/`DELETE /dispositivos` con `{token}`) y de nv-43 (`GET`/`POST`/`PATCH /admin/moderacion/reglas`) siguen iguales.

## Diseño
### Dispositivos por id (`internal/notificaciones`)
- **Conciliación:** `{id}` es el `_id` (ObjectId) del documento de `dispositivos`, nunca el token: se mantiene la decisión de nv-40 (el token no va en la ruta). El Excel queda corregido de `{fcm_token}` a `{id}`.
- **Cómo conoce la app el id:** `PUT /dispositivos` (nv-40) sigue respondiendo 204 y agrega la cabecera `Location: /api/v1/dispositivos/{id}`. No cambia el cuerpo ni el código de estado, así que la app actual no se rompe.
- **Campos nuevos** del documento: `app_version` (SemVer de hasta 20, validator `semver` de nv-64) y `canales_activos` (subconjunto sin repetidos de `reservas`, `cupones`, `grupos`, `resenas`, `promociones`; ausente = todos). Son los canales de notificación que el sistema del teléfono deja encendidos; la preferencia de la cuenta (`preferencias.notificaciones`, nv-05) sigue mandando.
- **`PUT /dispositivos/:id`** con `{app_version?, canales_activos?}` (al menos uno; campo de más = 400, formato malo = 422): actualiza y renueva `ultimo_uso`; responde 200 `{id, plataforma, app_version, canales_activos, ultimo_uso}` (nunca el token ni su hash). Sin `If-Match`: es un ajuste del propio teléfono, el último gana (excepción a `AGENTS.md` §5, para el revisor).
- **`DELETE /dispositivos/:id`**: borra el documento; 204. Repetirlo = 404.
- **Propiedad:** filtro por `_id` y `usuario_id` en la misma consulta; id de otra cuenta o inexistente = 404 igual. `id` no ObjectId = 422.
- **Envío (nv-41):** el push de un tipo se salta en el teléfono cuyo `canales_activos` no lo incluye; las demás reglas (horario silencioso, tope de marketing) no cambian. `reservas` siempre se envía aunque falte en la lista: la hoja A1 dice que no se puede apagar del todo.
- **ARCO:** la exportación de `dispositivos` agrega `app_version` y `canales_activos`; el `Purger` ya los borra con la cuenta.

### Reglas de moderación por filtro (`internal/moderacion`)
- **Conciliación con nv-43:** se conserva el documento único `moderacion_reglas` (`_id: "vigentes"`, una sola `version`, una sola lectura por evaluación) y se le agrega un mapa `filtros` con una entrada por filtro: `{cuarentena?, rechazo?, activa}`. `{id}` es el nombre del filtro, no un ObjectId: `desnudez`, `violencia`, `drogas`, `odio`, `sexual`, `acoso`, `spam` o `datos_personales` (las constantes de `moderacion.go`). Otro valor = 404. `palabra_bloqueada` no es un filtro editable: las palabras se ajustan con `PATCH /reglas` (nv-43).
- **Por qué no una colección por regla (Excel):** el `Moderador` necesita todas las reglas juntas en cada evaluación; un solo documento evita leer N y mantiene un solo `If-Match` para todo el juego de reglas. La colección del Excel `reglas_moderacion` con `ux_filtro` no se crea.
- **Cómo decide el `Moderador`:** por cada filtro con puntaje, usa sus umbrales si existen y si no los globales (`cuarentena`, `rechazo`); un filtro con `activa: false` no cuenta. Sin entrada en `filtros`, todo sigue como en nv-43.
- **`PATCH /admin/moderacion/reglas/:id`** con `If-Match` (la `version` del documento) y cualquiera de `{cuarentena, rechazo, activa}`: los umbrales del filtro deben cumplir 40 ≤ cuarentena < rechazo ≤ 100 (con el valor global para el que falte); responde 200 con las reglas completas (como `GET /reglas`). Sin reglas guardadas = 404 (como `PATCH /reglas`); versión vieja = 409; sin `If-Match` = 400.
- **`DELETE /admin/moderacion/reglas/:id`** con `If-Match`: deja el filtro en `activa: false` (Excel: «Desactiva una regla»), conserva sus umbrales y responde 204. Repetirlo no cambia nada salvo la versión vieja (409).
- **`datos_personales` no se puede desactivar:** `DELETE` o `activa: false` sobre él = 422. Desactivarlo dejaría pasar teléfonos y correos en reseñas y chat (decisión para el revisor).
- **`GET /admin/moderacion/reglas`** agrega `filtros` con los ocho, cada uno `{cuarentena, rechazo, activa, propio}` (`propio: false` = hereda los globales). Cada cambio guarda `actualizado_por` y `actualizado_en` como nv-43.
- **Límite:** las dos rutas nuevas comparten la política `admin` (200 por minuto por IP), antes de `RequireRoles`, como nv-43.

- **Variables:** ninguna nueva.

## Contratos
Filas del Excel copiadas y corregidas:

| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| PUT | `/api/v1/dispositivos/{id}` | JWT, dueño | 200, 400, 401, 403, 404, 422 |
| DELETE | `/api/v1/dispositivos/{id}` | JWT, dueño | 204, 401, 403, 404, 422 |
| PATCH | `/api/v1/admin/moderacion/reglas/{id}` | JWT administrador | 200, 400, 401, 403, 404, 409, 422, 429 |
| DELETE | `/api/v1/admin/moderacion/reglas/{id}` | JWT administrador | 204, 400, 401, 403, 404, 409, 422, 429 |

- Excel A6: `PUT /dispositivos/{fcm_token}` con `{app_version, canales_activos}` → 200 y `DELETE /dispositivos/{fcm_token}` → 204. Corregido: `{id}` del documento; registrar y quitar por token siguen en `PUT`/`DELETE /dispositivos` (nv-40), que ahora devuelve `Location`.
- Excel A7: `PATCH /admin/moderacion/reglas/{id}` «Ajusta umbrales o palabras» → 200 y `DELETE` «Desactiva una regla» → 204. Corregido: `{id}` es el nombre del filtro, las palabras siguen en `PATCH /reglas`.
- 403 en dispositivos = cuenta en eliminación pendiente; en reglas = rol distinto de administrador. Las cuatro filas pasan a `docs/endpoints.md` al ejecutar la spec (no en este PR).

## Tests
- `tests/integration/dispositivos_id_test.go`: `Location` al registrar, ajustar `app_version` y `canales_activos`, borrar por id, id de otra cuenta e inexistente = 404 igual, id malo = 422, cuerpo vacío = 400, y el push de un canal apagado no se envía a ese teléfono (salvo `reservas`).
- `tests/integration/moderacion_reglas_filtro_test.go`: umbral propio de un filtro cambia el veredicto, desactivar un filtro lo ignora, reactivar con `PATCH {activa: true}`, `datos_personales` = 422, filtro desconocido = 404, sin reglas guardadas = 404, versión vieja = 409, sin `If-Match` = 400, y `GET /reglas` con `filtros`.
- `tests/security/dispositivos_reglas_security_test.go`: 401 en las cuatro rutas, 403 de consumidor y comerciante en reglas, 403 en dispositivos con cuenta en eliminación, ni token ni `token_hash` en las respuestas, y ningún token en la ruta ni en los logs.
- `tests/contract/dispositivos_reglas_contract_test.go`: 200 y 204 de dispositivos, 200 y 204 de reglas.
- Unit: validators de `canales_activos` y `app_version`; decisión del `Moderador` con umbral propio, heredado e inactivo (`internal/moderacion/umbrales_test.go`).

## Criterios
- El token de FCM nunca aparece en una ruta, una respuesta ni un log, y ningún administrador puede apagar el filtro de datos personales.

## Límites
- Sin `GET /dispositivos` (la app guarda el id del `Location`); se agrega si el panel de sesiones lo pide.
- Sin colección `reglas_moderacion` ni palabras por filtro.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-13), ola por decidir
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** `{id}` del dispositivo es el `_id`, nunca el token (se mantiene nv-40); el id llega por `Location` sin cambiar el 204; `PUT /dispositivos/{id}` sin `If-Match`; reglas por filtro dentro del documento único de nv-43 (`{id}` = nombre del filtro, sin colección `reglas_moderacion`); `DELETE` desactiva y no borra; `datos_personales` no se puede desactivar.
- **Detalle:** evidencia en `specs/nv-71/checklists/gates.md` + `Memoria.md` al ejecutar.
