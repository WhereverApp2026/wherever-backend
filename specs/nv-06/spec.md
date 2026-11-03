# NV-06 — Correo saliente: SMTP, envío en segundo plano y plantillas

## Contexto
Deuda de `nv-04` y `nv-05`: hoy el `Mailer` solo tiene un buzón en memoria (pruebas) y un `LogMailer`, así que ningún entorno manda correos. Sin esto no salen la verificación de correo, la recuperación de contraseña ni el enlace de "Descargar mis datos". **Depende de**: NV-04, NV-05.

## Requisitos
1. Adaptador SMTP genérico para la interfaz `Mailer` existente, con la librería estándar de Go. El proveedor (SES, Resend, Brevo, otro) es solo configuración.
2. Envío en segundo plano: ninguna petición espera al servidor de correo. Un fallo de envío nunca cambia la respuesta HTTP.
3. Tres plantillas en español, texto plano y HTML, con el nombre de la persona: verificación de correo, recuperación de contraseña y descarga de datos.
4. Enlaces armados con `APP_LINK_BASE` y el token en el **fragmento** (`#token=`), nunca en la ruta ni en la consulta; en producción solo `https://`.
5. Mailpit en el `docker-compose` de desarrollo, para ver los correos sin cuenta en ningún proveedor.
6. La exportación no se pierde si el correo falla: el aviso se reintenta en la misma corrida y en las siguientes mientras la exportación no venza (tope de 72 h desde que se armó).
7. Ningún token, enlace ni cuerpo de correo en logs ni en errores.

## Diseño
- **Paquete** `internal/platform/mailer`: `smtp.go` (adaptador), `async.go` (cola acotada con trabajadores), `templates/*.tmpl` (`html/template` y `text/template`), `link.go` (arma enlaces). `memory.go` y `log.go` de `nv-04` se quedan para las pruebas automáticas y para dev/staging sin servidor.
- **Mensaje**: `mailer.Message` agrega `Name` (nombre de pila de la persona) para las plantillas; quien envía (auth) lo llena desde `usuarios`. El enlace no viaja en el mensaje: lo arma el adaptador con `link.go` a partir del tipo y el token.
- **Configuración nueva** (solo por entorno): `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_TLS` (`starttls` por defecto, `tls` para el puerto 465, `none` solo fuera de producción), `MAIL_FROM` (remitente; obligatorio si hay `SMTP_HOST`), `MAIL_TIMEOUT` (5 s), `MAIL_WORKERS` (4), `MAIL_QUEUE_SIZE` (100), `APP_LINK_BASE`. Todas entran a `configs/.env.example` sin secretos.
- **Validación al arrancar**: en producción fallan el arranque `SMTP_TLS=none`, un `APP_LINK_BASE` que no sea `https://` y la **falta de `SMTP_HOST`** (sin correo nadie verifica su cuenta ni recupera su contraseña). En dev y staging, sin `SMTP_HOST` se usa el `LogMailer` y se avisa en el log.
- **Autenticación**: solo si hay `SMTP_USER`; con `smtp.PlainAuth`, que no manda la contraseña sin TLS. Mailpit (sin usuario) no autentica.
- **Cola**: canal con capacidad `MAIL_QUEUE_SIZE`; `Enqueue` nunca bloquea la petición. Si la cola está llena, se descarta el envío y se cuenta un contador (sin contenido). Cada envío tiene `MAIL_TIMEOUT`, hasta 2 intentos con espera de 1 s. Al apagar la API se vacía la cola con un plazo máximo.
- **Anti-enumeración**: `/auth/recuperar` encola y responde 202 siempre; `/auth/reenviar-verificacion` y el registro también. El tiempo de respuesta no depende de que el correo exista ni de que el servidor de correo responda.
- **Seguridad del mensaje**: destinatario, asunto y remitente se validan contra saltos de línea (CR/LF) antes de armar los encabezados; los datos del usuario (por ejemplo el nombre) entran a las plantillas escapados.
- **Registro en logs**: destinatario enmascarado, tipo de correo, resultado y duración. Nunca cuerpo, enlace, token ni la respuesta cruda del servidor SMTP.
- **Exportación** (D-2 de `nv-05`, corregida):
  - El trabajo nocturno manda el aviso de forma **síncrona** con el adaptador SMTP directo (no con la cola), con los mismos 2 intentos, porque necesita saber si salió.
  - Cada intento de aviso **invalida el token de descarga anterior y crea uno nuevo**: solo guardamos el hash, así que no se puede volver a mandar el mismo.
  - Se agrega `aviso_enviado_en` (date, opcional) a `exportaciones`. Si el aviso falla, la exportación queda `lista` sin `aviso_enviado_en` y las corridas siguientes lo reintentan.
  - **Tope de 72 h** desde que se armó (`lista_en`): `expira_en = lista_en + 72 h`, y el TTL la borra al vencer aunque el aviso no haya salido; la persona puede pedir otra (una por día).
  - **Las 24 h de descarga cuentan desde `aviso_enviado_en`**: al enviarse el aviso, `expira_en = min(aviso_enviado_en + 24 h, lista_en + 72 h)` y el token del enlace vence a las 24 h.
- **Mailpit**: servicio `mailpit` en el compose con perfil `dev`, SMTP en `127.0.0.1:1025` y vista web en `127.0.0.1:8025`, sin exponerse fuera de la máquina.

## Contratos
Sin endpoints nuevos ni cambios en las respuestas de los existentes. Correos que se envían:

| Tipo | Disparador | Enlace |
|---|---|---|
| `verificar_correo` | registro, cambio de correo, `reenviar-verificacion` | `{APP_LINK_BASE}/verificar#token=…` |
| `recuperar_password` | `POST /auth/recuperar` (solo si el correo existe) | `{APP_LINK_BASE}/restablecer#token=…` |
| `descargar_exportacion` | trabajo nocturno de exportaciones | `{APP_LINK_BASE}/exportacion#token=…` |

El token va en el fragmento: el navegador no lo manda al servidor que sirve la página, así que no queda en sus logs ni en el `Referer`. La página o la app lo lee y lo envía por `POST` a la API, como ya hacen `restablecer`, `verificar-correo` y `exportaciones/descargar`.

Cambio de datos: `exportaciones.aviso_enviado_en` (date, no obligatorio). Sin índice nuevo (el TTL sigue en `expira_en`).

## Tests
- `Unit/Mailer 14`: tres plantillas se renderizan en español con y sin nombre raro (`<script>`), enlace con `APP_LINK_BASE` y el token en el fragmento, CR/LF en destinatario o asunto rechazado, configuración (`none`, `http://` o sin `SMTP_HOST` en producción falla; sin `SMTP_HOST` fuera de producción usa `LogMailer`), cola llena descarta sin bloquear, tiempo máximo respetado, segundo intento tras fallo, apagado vacía la cola, la autenticación solo se arma con usuario.
- `Integration/Mailer 6` con Mailpit real (Testcontainers) y la API de Mailpit para leer los correos: registro, recuperación para correo existente, recuperación para correo inexistente (no llega nada y la respuesta es idéntica), cambio de correo, exportación (aviso enviado y 24 h desde el aviso), exportación con servidor caído y reintento en la corrida siguiente con token nuevo (el anterior ya no descarga).
- `Integration/Exportación 2`: sin aviso, la exportación vence a las 72 h de armarse; con aviso, a las 24 h del aviso sin pasar de las 72 h.
- `Integration/Respuesta con servidor lento 2`: un servidor SMTP que tarda 5 s o está caído no cambia el 202 de `/auth/recuperar` ni el 201 del registro, y la respuesta tarda menos de 500 ms.
- `Security/Mailer 4`: ningún token, enlace ni cuerpo en los logs (ni con el servidor caído), sin autenticación ni contraseña por canal sin TLS, inyección de encabezados rechazada, plantillas escapan HTML.
- `Config/.env.example`: las claves nuevas están y no hay secretos (el test de `nv-00` sigue en verde).

## Criterios
- `go build ./...`, `go vet ./...` y todas las suites en verde; `critic B2 → PASS`.
- Con `docker compose --profile dev up`, registrar una cuenta deja el correo visible en `http://127.0.0.1:8025`.
- Apagar el servidor de correo no cambia ninguna respuesta HTTP.
- Producción no arranca sin `SMTP_HOST`, con `SMTP_TLS=none` o con `APP_LINK_BASE` sin `https://`.
- `scripts/check_endpoints.sh → OK` sin filas nuevas; cobertura ≥ 45 %.

## Límites
- Sin proveedor fijado: la elección (SES, Resend, Brevo u otro) se toma al desplegar. Los precios y los límites de cada uno se confirman en su página oficial.
- Sin dominio propio: SPF, DKIM y DMARC quedan como deuda del despliegue; sin ellos los correos reales pueden caer en spam.
- Sin cola persistente: si una instancia se cae con envíos en vuelo, se pierden. Es aceptable porque la verificación y la recuperación se pueden pedir otra vez, y la exportación se reintenta por `aviso_enviado_en`.
- Sin enlaces de aplicación (Android App Links): mientras no haya dominio, `APP_LINK_BASE` apunta a una página web o a localhost en desarrollo. No se usa un esquema propio (`wherever://`) en producción porque otra app podría interceptar el token. La página o la app debe leer el token del fragmento (`#token=`) y mandarlo por `POST`; queda como requisito de la spec móvil.
- Sin plantillas con diseño de marca; sin pruebas de entrega a bandejas reales; sin rebotes ni quejas (webhooks del proveedor).
- Aviso de privacidad: al elegir proveedor debe aparecer como encargado del tratamiento de los correos (LFPDPPP).

## Aprobación y Control de Cambios
- **Estado:** ✅ Aprobada (T1–T8)
- **Revisores:** Osmar Reséndiz
- **Fecha:** 2026-11-03 (ejecución y aprobación)
- [x] CHECKLIST_PR.md OK
- [x] Sin conflicto con constitución
- **Detalle:** evidencia en `specs/nv-06/checklists/gates.md` + `Memoria.md`. Ajustes del revisor antes de ejecutar (2026-11-03): `MAIL_FROM` nuevo, `Name` en el mensaje, token en el fragmento, producción falla sin `SMTP_HOST`, aviso síncrono con reintento que invalida el token anterior, tope de 72 h y 24 h desde el aviso.
