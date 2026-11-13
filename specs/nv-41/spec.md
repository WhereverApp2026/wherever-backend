# NV-41 — Envío de push con Firebase

## Contexto
Los avisos llegan al teléfono respetando preferencias y horario silencioso, sin bloquear nada. **Origen**: US-162 y hoja A6 del Excel. **Depende de**: NV-40.

## Requisitos
1. Todo evento crea la notificación en el centro y, si corresponde, envía push por FCM.
2. El push respeta las preferencias del usuario por tipo y el **horario silencioso**; en silencio la notificación queda en el centro y no suena.
3. Tokens inválidos (`UNREGISTERED`) se borran solos.
4. Envío en segundo plano con reintentos; un fallo de FCM nunca cambia la respuesta de la acción original.
5. Sin contenido sensible en el push: título y texto genéricos.

## Diseño
- **Cliente** `internal/platform/push`: FCM HTTP v1 (`/v1/projects/{id}/messages:send`) con la cuenta de servicio de Firebase. Firma un JWT RS256 (`golang-jwt`, ya en el proyecto: sin librerías nuevas), lo cambia en `token_uri` por un token de acceso y lo guarda en memoria hasta un minuto antes de que venza (`Reset()` con mutex). Respuestas: 404 o `UNREGISTERED` = `ErrUnregistered`; 5xx, 429, 401 o red = `ErrRetryable`; lo demás = `ErrRejected`. Los errores no llevan la respuesta cruda de Google.
- **Cola** `notificaciones.Pusher`: 4 trabajadores, cola de 500, igual que el correo (nv-06). `Enqueue` nunca bloquea; con la cola llena el aviso queda solo en el centro y se cuenta. 3 intentos con espera creciente (1 s, 2 s) solo para fallos temporales. Un token `UNREGISTERED` se borra (`RemoveDeviceByHash`).
- **Mismo origen que el centro:** `notificaciones.Service.Emit` guarda el aviso y, solo si se guardó, lo encola para push. Así el centro y el push cuentan lo mismo.
- **Preferencias:** `invitacion_plan`, `votacion_abierta` y `votacion_cerrada` siguen el interruptor `preferencias.notificaciones.grupos` (nv-05). Los de amistad no tienen interruptor propio en nv-05 y siempre se envían. Cuenta no activa = sin push.
- **Horario silencioso:** `preferencias.horario_silencioso{desde, hasta}` (`HH:MM`) en la hora de México (zona por defecto de la constitución §5); `desde` incluido y `hasta` excluido; si `desde` > `hasta`, cruza la medianoche. En silencio no se envía (el aviso ya está en el centro).
- **Texto genérico:** título «Wherever» y un texto fijo por tipo («Te invitaron a un plan», etc.); `data` lleva `tipo` y los ids del aviso. Nunca nombres ni contenido.
- **Configuración:** `FCM_PROJECT_ID` y `FCM_CREDENCIALES_B64` (el JSON de la cuenta de servicio en base64, una línea, porque `deploy-env.sh` escribe una variable por línea y el JSON tiene varias). Van juntas o ninguna; sin ellas no hay push y todo lo demás funciona. `config.Load` valida la llave al arrancar. `FCM_URL` solo cambia fuera de prod (pruebas).
- **Logs:** tipo y resultado (`push_enviado`, `push_token_baja`, `push_fallido`, `push_descartado`, `push_apagado`); nunca token ni cuerpo.
- **Corrección de drift (2026-11-12):** el parámetro `fcm-service-account` no era un nombre de variable válido y su JSON de varias líneas no cabía en `deploy-env.sh` (hallazgo de la revisión); ahora es `FCM_CREDENCIALES_B64`. `notificaciones.Emitir` ya existía como `Emit` (nv-40).

## Operación (la hace el equipo; las credenciales nunca pasan por el chat)
1. Firebase: proyecto en el plan Spark, app Android con el *package name* de la app y llave de la cuenta de servicio (Configuración del proyecto → Cuentas de servicio → Generar nueva clave privada).
2. Local: en `.env`, `FCM_PROJECT_ID=<id>` y `FCM_CREDENCIALES_B64=<base64 del JSON>`. En PowerShell: `[Convert]::ToBase64String([IO.File]::ReadAllBytes("ruta\llave.json"))`.
3. AWS: `FCM_PROJECT_ID` (String) y `FCM_CREDENCIALES_B64` (SecureString) en `/wherever/dev/`. La siguiente instalación de imagen los toma (`deploy-env.sh`); la salida por 443 ya está abierta.
4. Prueba en vivo, una vez: registrar el teléfono de prueba (`PUT /dispositivos`) y mandarse una solicitud de amistad.

## Contratos
Sin endpoints nuevos. Variables nuevas: `FCM_PROJECT_ID`, `FCM_CREDENCIALES_B64` y `FCM_URL` en `configs/.env.example`.

## Tests
- Unit `internal/notificaciones/push_test.go` (10): tipo apagado, amistad sin interruptor, silencio que cruza la medianoche (noche, madrugada, borde excluido), silencio de tarde, horario mal formado, hora de México; texto para cada tipo.
- Unit `internal/platform/push/fcm_test.go`: envía y reutiliza el token de acceso; `UNREGISTERED`, 503 y FCM caído; credenciales inválidas.
- Integration `tests/integration/push_test.go` contra `internal/testutil/fcmtest` (servidor local que habla FCM v1 y el intercambio de tokens, con llave propia): un push por teléfono con texto genérico y el mismo aviso en el centro; token dado de baja se borra; 503 se reintenta; horario silencioso no suena; FCM caído no frena la solicitud.
- Security: el token no llega a los logs (`TestPush_SolicitudLlegaAlTelefonoConTextoGenerico`, `TestNotificacionesSecurity_*`); el sexto dispositivo = 422 (nv-40).
- En vivo, una vez: push real a un teléfono de prueba (pendiente de las credenciales).

## Criterios
- Un fallo de FCM no afecta la acción que originó el aviso.
- En horario silencioso no suena.

## Límites
- Sin push a iOS.
- Sin notificaciones programadas (recordatorios llegan después).
- La entrega no está garantizada: FCM es de mejor esfuerzo. Al apagar la API, los push en cola se pierden (el aviso sigue en el centro).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-12) con la corrección de drift; implementada en la rama `nv-41`; falta la prueba en vivo con las credenciales del equipo.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** amistad sin interruptor propio; credenciales en base64; 3 intentos; los push en cola se pierden al apagar.
- **Detalle:** la evidencia irá en `specs/nv-41/checklists/gates.md` + `Memoria.md`.
