# NV-73 — Mensajes del negocio en una reserva y recordatorio de 2 h

## Contexto
El negocio necesita «Contactar» al cliente de una reserva desde P-48 (llego tarde, falta un dato, cambió la mesa) y el cliente necesita el aviso de 2 h antes. nv-59 dejó las dos piezas fuera porque esperaban notificaciones; nv-40 (centro de avisos) y nv-41 (push con Firebase) ya están en `main`. **Origen**: hoja «J5 Reservas y pagos» del Excel `Wherever_Endpoints_CRUD_v3.xlsx` (fila del endpoint y el índice `ix_recordatorios`, «Recordatorio 2 h antes y marcar no_asistio al terminar el día»). **Depende de**: NV-04 (`RequireAuth`, `RequireRoles`), NV-05 (`RequireActive`, ARCO), NV-11 (presupuesto de Redis), NV-40 (`notificaciones.Notifier`, canal `reservas`), NV-41 (push), NV-56 (propiedad del negocio) y NV-59 (`internal/reservas`, barrido de 5 min, `ix_recordatorios`).

## Requisitos
1. `POST /negocios/{id}/reservas/{rid}/mensajes` deja un mensaje del negocio al cliente de esa reserva y le avisa por el centro de notificaciones y push.
2. El cliente lee los mensajes de su reserva en `GET /reservas/{id}` (campo nuevo `mensajes`).
3. Cada reserva confirmada recibe un solo aviso unas 2 h antes de `fecha_hora`, aunque corran dos instancias.
4. Los mensajes son datos del cliente: se borran con su cuenta y a los 90 días, y entran en su exportación.

## Diseño
### Mensajes
- **Paquete:** `internal/reservas` (archivo `mensajes.go`: modelo, DTO, service y repositorio de mensajes y del recordatorio): usa el repositorio de reservas para validar la reserva y la propiedad, sin módulo nuevo.
- **Colección `mensajes_reserva`:** `reserva_id`, `negocio_id`, `usuario_id` (el cliente, destinatario), `texto` (1 a 500 caracteres, sin HTML, recortado), `autor_id` (el dueño que lo mandó), `creado_en`, `expira_en` (`creado_en` + 90 días). Índices `ix_mensajes_reserva` `{reserva_id: 1, creado_en: -1}`, `ix_mensajes_usuario` `{usuario_id: 1}` (ARCO) y `ttl_mensajes_reserva` `{expira_en: 1}` con `expireAfterSeconds: 0`, en `scripts/indexes` y `/ready`.
- **Quién:** JWT con rol `comerciante` + `RequireActive`, dueño del negocio `{id}` (como `PATCH /negocios/:id/reservas/:rid`). Reserva de otro negocio o inexistente = 404 (no revela si existe).
- **Cuándo:** solo reservas `confirmada`, o `asistio`/`no_asistio` con `fecha_hora` de hace menos de 24 h (dejaron algo olvidado). En `bloqueada`, `pendiente_pago`, `cancelada` o `expirada` = 409 `reserva_no_contactable`.
- **Límite contra abuso, sin Redis:** 5 mensajes por reserva en 24 h, contados en MongoDB con `ix_mensajes_reserva`; el 6.º = 429 uniforme con `Retry-After` hasta que sale el más viejo de la ventana. Una política propia sumaría 3 comandos por mensaje.
- **Aviso:** `Notifier.Emit(cliente, KindReservaMensaje = "reserva_mensaje", {reserva_id, negocio_id, mensaje_id})` en el canal `reservas` (`notificaciones.ChannelOf`) (siempre activo, nv-40). Como pide nv-40, `datos` lleva solo ids; la app pide el texto con `GET /reservas/{id}`. Avisar nunca cambia la respuesta (201 aunque falle el push).
- **Lectura:** `GET /reservas/{id}` (dueño de la reserva) agrega `mensajes: [{id, texto, creado_en}]`, los 20 más nuevos, de más nuevo a más viejo; `[]` si no hay. El negocio no ve el campo (tampoco cuando abre el mismo detalle) y la agenda P-48 no cambia. Las demás respuestas de reserva no lo llevan (`omitempty` sobre un puntero).
- **Respuesta 201:** `{ id, reserva_id, texto, creado_en }`.
- **Moderación:** sin IA ni filtro de palabras (no gasta Gemini); el texto va de un negocio verificado (nv-55) a su propio cliente. El cliente puede reportar al negocio con `reportes_negocio` (ya existe). Decisión para el revisor.
- **Logs:** nunca el `texto`; solo `reserva_id` y el evento.

### Recordatorio de 2 h
- **Dónde:** el barrido de nv-59 (`reservas.Barrer`, cada 5 min, solo MongoDB, sin candado de Redis) gana un tercer paso.
- **Qué:** reservas `confirmada` con `fecha_hora` en `(ahora, ahora + 2 h]` y sin `recordatorio_en` (`Service.Recordar`, tercer paso de `Barrer`). Por cada una, `UpdateOne` condicional (sigue `confirmada`, a la misma `fecha_hora` y con `recordatorio_en` ausente → `$set recordatorio_en: ahora`, sin subir `version`); solo si `ModifiedCount = 1` se emite `reserva_recordatorio {reserva_id, negocio_id}`. Dos instancias no duplican el aviso. Usa `ix_recordatorios` `{estado: 1, fecha_hora: 1}`, que ya existe. El barrido en segundo plano recibe el mismo centro de avisos con push que las votaciones (`backgroundNotifier` en `cmd/api`).
- **Campo nuevo en `reservas`:** `recordatorio_en` (opcional, UTC). Una reserva que se cambia de hora (`PATCH /reservas/{id}`) borra `recordatorio_en` para que el aviso salga otra vez con la hora nueva.
- **Reservas hechas con menos de 2 h:** reciben el aviso en el siguiente barrido (≤ 5 min); es aceptable porque ya acaban de ver la confirmación. Decisión para el revisor: omitirlo si `creado_en` es de hace menos de 15 min.
- **Tope por corrida:** 500 reservas por barrido para no alargarlo; el resto sale en el siguiente.

### Privacidad (ARCO)
- `reservas.Privacy` (ya registrado) amplía: `PurgeAccount` borra `mensajes_reserva` con `usuario_id` = la persona; `Export` agrega `mensajes[{texto, negocio, creado_en}]` a cada reserva (el nombre sale de `Privacy.WithNegocios`). `PurgeHistory` no los toca (no son historial de uso). No es una llave nueva en la exportación: `exportacion_descargar_200.json` no cambia.
- Baja del negocio (`negocios.Observador.DadoDeBaja`): no se borran; el cliente los conserva hasta el TTL.

### Costos
- **Redis:** el `POST` cuesta lo mismo que cualquier escritura con sesión (política global 3 + `Writes` 3 + revocación 1 ≈ 7); el límite va en MongoDB. El recordatorio corre sin Redis. Push: FCM no usa Upstash. Se fija con `TestPresupuestoRedis_MensajesReservaNV73`.
- **Gemini:** ninguno.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/negocios/{id}/reservas/{rid}/mensajes` | JWT dueño (comerciante) | 201, 400, 401, 403, 404, 409, 422, 429 |
| GET | `/api/v1/reservas/{id}` (cambio aditivo: `mensajes`) | JWT | 200, 401, 403, 404 |

Fila copiada del Excel (J5): `POST /api/v1/negocios/{id}/reservas/{rid}/mensajes` «"Contactar": mensaje del negocio al cliente dentro de la app; le llega como notificación.» (Body: texto; 201; Dueño del negocio). Índice (J5 §1): `ix_recordatorios` `{ estado: 1, fecha_hora: 1 }` «Recordatorio 2 h antes y marcar no_asistio al terminar el día». Se copian a `docs/endpoints.md` al ejecutar.

**Drift con el Excel:** el Excel no dice dónde ve el cliente el mensaje ni lo guarda; aquí se guarda en `mensajes_reserva` y se lee en el detalle de la reserva (los avisos de nv-40 no llevan texto). La respuesta 201 trae el mensaje. 403 = rol que no es comerciante o cuenta en eliminación; 409 = estado de la reserva.

## Tests
- `tests/integration/reservas_mensajes_test.go`: mensaje en reserva confirmada (201, aviso en `notificaciones` con solo ids), reserva cancelada = 409, reserva de otro negocio = 404, 6.º mensaje en 24 h = 429, `GET /reservas/{id}` trae los mensajes en orden, ARCO (borrar cuenta los quita; exportación los trae).
- `tests/integration/reservas_recordatorio_test.go`: confirmada a 1 h 50 min recibe un aviso; dos `Barrer` en paralelo = un solo aviso; a 3 h nada; cancelada nada; cambio de hora vuelve a avisar.
- `tests/security/reservas_mensajes_security_test.go`: 403 con consumidor, ningún log con `texto`, el aviso solo lleva ids y nadie más lo recibe, detalle ajeno = 404. 401, cuenta en eliminación y dueño de otro negocio = 404 entran en la tabla de `reservas_security_test.go`.
- `tests/integration/presupuesto_redis_test.go` (`TestPresupuestoRedis_MensajesReservaNV73`).
- `tests/database/indexes_test.go`: `ix_mensajes_reserva`, `ix_mensajes_usuario`, TTL. `tests/contract/reservas_mensajes_contract_test.go`: 201 del mensaje y el detalle con `mensajes`.
- Unit `internal/reservas/mensajes_test.go`: DTO (vacío, solo espacios, 501 caracteres, HTML), estados contactables, ventana del recordatorio y `Retry-After`; `internal/notificaciones`: canal y texto de push de los dos tipos nuevos.

## Criterios
- Un mensaje llega al cliente correcto y a nadie más; el negocio ajeno no sabe si la reserva existe.
- Una reserva confirmada recibe un recordatorio y solo uno, con varias instancias.
- Ningún aviso, log ni push lleva el texto.

## Límites
- Sin respuesta del cliente al negocio (chat de ida y vuelta) ni adjuntos.
- Sin mensajes masivos a todas las reservas del día.
- Sin recordatorios configurables (solo 2 h) ni por correo.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-16), ola 12; implementada en la rama `claude/nv-73-87ba71`, sin firma.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** mensajes guardados en `mensajes_reserva` (90 días) y leídos en `GET /reservas/{id}`; solo de ida (negocio → cliente); 5 por reserva en 24 h contados en MongoDB; sin moderación con IA; recordatorio dentro del barrido de nv-59 con `UpdateOne` condicional, sin Redis; cambio de hora reinicia el recordatorio. **Aplicadas tal como se proponen (2026-11-16)**; las reservas hechas con menos de 2 h sí reciben el aviso (no se aplicó la excepción de 15 min).
- **Detalle:** evidencia en `specs/nv-73/checklists/gates.md` + `Memoria.md` al ejecutar.
