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

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-21), sin ejecutar.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) éxito parcial al reservar todo; (2) un pago por reserva; (3) permiso por omisión `ver`; (4) «me apunto» entra con `ver`.
- **Detalle:** `specs/nv-89/checklists/gates.md` al ejecutar.
