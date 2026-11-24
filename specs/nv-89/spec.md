# NV-89 — Reservar todo el plan y permisos al compartir

## Contexto
En el plan de la maqueta (`39_Plan`) el botón **«Reservar todo»** aparta todas las paradas de una vez, y al compartir (`42_Compartir`) se elige **«Quién puede hacer cambios: Solo ver · Puede editar»**. Hoy cada parada se reserva aparte con `POST /reservas` (nv-59, con `plan_id` y `parada_id` opcionales) y solo el dueño edita su plan; compartir (nv-38) y «me apunto» (nv-69) meten a la persona al grupo del plan sin permiso de edición. **Origen**: revisión de la maqueta (2026-11-21). **Depende de**: NV-30 (planes y paradas), NV-37/NV-38 (grupo del plan y enlace), NV-59/NV-60 (reservas y anticipo), NV-46 (sincronización).

## Requisitos
1. Reservar en una sola petición todas las paradas reservables del plan, con resultado por parada.
2. Al compartir, el dueño elige si los integrantes solo ven el plan o también lo editan, y puede cambiarlo por persona.
3. Un integrante con permiso de edición cambia nombre, fecha y paradas con las mismas reglas que el dueño.

## Diseño
### Reservar todo
- **`POST /planes/{id}/reservas`** con `Idempotency-Key` y `{personas? (por omisión las del plan), nota?}` → 200 con el resultado de cada parada: `{resultados[{parada_id, negocio_id, estado, reserva?, motivo?}], anticipo_total, requieren_pago}`. `estado`: `reservada` (o `pendiente` si pide anticipo), `no_acepta_reservas`, `sin_cupo`, `ya_reservada` (la parada ya tiene una reserva vigente del plan), `fuera_de_horario`.
- Cada parada usa el mismo servicio de `POST /reservas` (mismo candado por negocio y turno, mismas reglas de cupo, apartado y anticipo) con `plan_id`, `parada_id` y la hora `llega` de la parada; la llave de idempotencia por parada es `{llave}:{parada_id}`. Éxito parcial permitido: una parada sin cupo no deshace las otras.
- **Pago:** cada reserva con anticipo se paga como hoy (`POST /reservas/{id}/pago`); «Pagar todo» en un solo cargo queda fuera (Stripe Connect cobra por negocio).
- Solo el dueño o un integrante con permiso `editar`; plan borrado o pasado = 404/422. Límite: cuenta como una reserva por parada en la política `reservas`.

### Permisos al compartir
- **`POST /planes/{id}/compartir`** acepta `{permiso: ver | editar}` (por omisión `ver`); el enlace guarda el permiso y quien entra por él (`grupos/unirse`) recibe ese `permiso_plan` en `grupos.integrantes[]`. «Me apunto» (nv-69) entra con `ver`.
- **`PATCH /grupos/{id}/integrantes/{usuario_id}`** `{permiso_plan}` — solo el organizador (dueño del plan).
- **Edición por integrantes:** `PATCH /planes/{id}`, paradas (`POST`, `PATCH`, `DELETE`, `orden`, `cambiar`) aceptan a un integrante con `editar` y `If-Match`; borrar el plan, compartir, cambiar visibilidad o permisos sigue siendo solo del dueño. El número de sincronización sigue siendo el del dueño (nv-46) y la copia del integrante se actualiza por el grupo.
- **`GET /planes/{id}`** para integrantes agrega `mi_permiso`.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/planes/{id}/reservas` | JWT dueño o integrante con `editar` | 200, 400, 401, 403, 404, 409, 422, 429 |
| POST | `/api/v1/planes/{id}/compartir` | JWT dueño | agrega `permiso` |
| PATCH | `/api/v1/grupos/{id}/integrantes/{usuario_id}` | JWT organizador | 200, 400, 401, 403, 404, 422 |
| PATCH | `/api/v1/planes/{id}` y rutas de paradas | JWT dueño o integrante con `editar` | igual que hoy |

## Tests
- Integration: reservar todo con paradas reservables, sin reservas, sin cupo y ya reservadas; idempotencia por llave; concurrencia con otra reserva del mismo turno; permisos `ver`/`editar` por enlace y por organizador; edición por integrante con `If-Match`.
- Security: integrante con `ver` no edita (403); no integrante = 404; un integrante no cambia permisos ni borra el plan; la llave de idempotencia de otra persona no devuelve sus reservas.
- Contract: resultado de reservar todo y `mi_permiso`.

## Criterios
- Reservar todo nunca deja una parada reservada dos veces ni supera el cupo.
- Solo quien tiene permiso `editar` cambia el plan, y nunca sus permisos.

## Límites
- Sin un solo cargo para todos los anticipos; sin roles más allá de ver y editar.

## Drift al ejecutar (2026-11-23)
- **Código:** reservar todo en `internal/reservas/plan.go` (`Service.ReservarPlan`, `TurnoDe`, `MismaParada`; ruta en el handler de reservas con `WithPlanes(planSvc)` en `main.go` y `authtest`). Permisos en `internal/grupos` (`Member.Permission` → `integrantes[].permiso_plan`, `Group.CodePermission` → `codigo_permiso`, `Repository.SetPermission`, `Service.SetPermission`, `ShareRequest`, `PermissionRequest`). Edición por integrantes en `internal/planes` (`Service.Editable`, `editAs`); `planes.Members` cambió `CanRead` por `PlanPermission` (una sola lectura del grupo para leer y para editar). Constantes `planes.PermissionView`/`PermissionEdit` (grupos importa planes, no al revés).
- **Éxito parcial (decisión 1):** cada parada es independiente; sin cupo, fuera de horario o sin reservas no deshace las demás. Solo se corta toda la petición por un error del servicio (MongoDB o Redis caídos = 503, la misma llave en curso = 409): lo ya reservado queda y el reintento con la misma `Idempotency-Key` lo devuelve tal cual.
- **Reintentos:** llave por parada `idem:reserva:{usuario}:{llave}:{parada_id}` (la spec decía `{llave}:{parada_id}`; se antepone el usuario como en `POST /reservas`, así la llave de otra persona no devuelve sus reservas). Orden por parada: idempotencia → reserva activa del plan (`ya_reservada`) → candado y, dentro de él, otra vez la revisión de la parada (`MismaParada`) y el cupo. Dos «reservar todo» a la vez con llaves distintas dejan una sola reserva por parada.
- **Hora de la reserva:** el turno que contiene la llegada de la parada (`TurnoDe`: llegar 10:30 a un negocio con turnos de 60 min reserva las 10:00); sin turno = `fuera_de_horario`. La spec decía «la hora `llega`»: con la hora exacta casi ninguna parada caería en un turno.
- **Estados:** además de los de la spec, `no_reservada` con `motivo` para lo demás que rechaza `POST /reservas` (`demasiados_apartados`, `anticipo_no_disponible`, o el estado de una reserva de la misma llave que ya se canceló o venció); un negocio no visible es `no_acepta_reservas` con `motivo: negocio_no_disponible`. `ya_reservada` no trae `reserva` (puede ser de otro integrante y llevaría su código). `pendiente` = la reserva está `bloqueada` esperando el anticipo; `reservada` = `pendiente` (confirmación manual) o `confirmada`.
- **Respuesta:** `anticipo_total` en centavos (suma de los anticipos por pagar) y `requieren_pago` es cuántas reservas esperan el pago (la spec no fijaba el tipo). Un pago por reserva (decisión 2).
- **Límite:** la política `reservas` (10/h por usuario) cuenta una por parada que se va a intentar (acepta reservas y no tiene reserva activa del plan), de una vez y antes de reservar: si no alcanza, 429 sin reservar nada; el reintento ya no cuenta las paradas hechas. Nuevo `middleware.Limiter.Take` (`INCRBY` en la misma tubería que la ventana; mismo costo que `INCR`). La ruta pide rol consumidor y cuenta activa, como `POST /reservas`.
- **Plan pasado:** 422 si ninguna parada está por llegar; una parada pasada de un plan vigente sale `fuera_de_horario`.
- **Permisos (decisiones 3 y 4):** enlace `ver` por omisión (cuerpo vacío = `ver`); `POST /grupos/unirse` da el permiso del enlace a quien entra; un enlace de `editar` sube a quien ya estaba y uno de `ver` no baja a nadie. «Me apunto» entra con `ver`. Un invitado que aún no confirma solo ve. `PATCH /grupos/{id}/integrantes/{usuario_id}` sin `If-Match` y sin subir la `version` del grupo (no la pedía la spec; cambiarla invalidaría el `If-Match` de renombrar o disolver).
- **Edición por integrantes:** `PATCH /planes/{id}`, las cuatro rutas de paradas, `cambiar` y `alternativas` aceptan al integrante confirmado con `editar` (alternativas también, porque sirve a `cambiar`). Integrante con `ver` = 403 (antes 404); `visibilidad` en el PATCH = 403 para el integrante; borrar, clonar, compartir y el paquete sin conexión siguen solo del dueño y dan 404 al integrante como antes. Quien tiene la sorpresa del grupo activa (nv-69) no edita (403): vería las paradas. El número de sincronización, el grafo y los contadores siguen siendo del dueño.
- **`mi_permiso`:** sale en `GET /planes/{id}` y en las ediciones hechas por un integrante; el dueño no lo lleva. `integrantes[].permiso_plan` sale en toda vista del grupo (el organizador, `editar`).
- **Índices y variables:** ninguno nuevo. La consulta de reservas activas del plan usa `ix_agenda_negocio` (negocios de las paradas y rango de horas).
- **Pruebas:** Unit `internal/reservas/plan_test.go`, `internal/grupos/permiso_test.go` y `TestRateLimit_TakeCuentaVariasDeUnaVez`; Integration `tests/integration/planes_reservar_test.go` (éxito parcial e idempotencia, rechazos, límite por parada, concurrencia, permisos por enlace y organizador, integrante con `editar`, `TestPresupuestoRedis_ReservarTodoNV89`) y `TestGrupos_IntegranteLeeElPlanYNoLoEdita` ahora espera 403; Security `tests/security/planes_permisos_security_test.go`; Contract `tests/contract/planes_reservar_contract_test.go` (`planes/reservar_todo_200`, `planes/compartir_201`, `planes/detalle_integrante_200`, `grupos/permiso_integrante_200`) y `grupos/punto_encuentro_200` con `permiso_plan`.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-23), ejecutada; ver `checklists/gates.md`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) éxito parcial al reservar todo; (2) un pago por reserva; (3) permiso por omisión `ver`; (4) «me apunto» entra con `ver`.
- **Detalle:** `specs/nv-89/checklists/gates.md` al ejecutar.
