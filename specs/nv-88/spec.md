# NV-88 — Cupo por hora y día del negocio

## Contexto
`55_NegHoras` deja al dueño fijar **mesas disponibles por hora para cada día de la semana** y las marca como «Tranquilo · Moderado · Hora pico» («Wherever usa esto para sugerir tus horas tranquilas»). `38_CambiarHora` y `22_Reserva` muestran horas sin cupo tachadas y las horas poco concurridas. Hoy `reservas-config` tiene una sola `capacidad_por_horario` para todo el horario y bloqueos puntuales (nv-59). **Origen**: revisión de la maqueta (2026-11-21). **Depende de**: NV-56 (configuración de reservas), NV-59 (disponibilidad y bloqueos), NV-66 (afluencia).

## Requisitos
1. El dueño fija el cupo por hora y día de la semana; donde no lo fija, aplica la capacidad general.
2. La disponibilidad usa ese cupo.
3. Cada hora trae su nivel de afluencia para sugerir horas tranquilas.

## Diseño
- **Modelo:** `reservas_config.cupo_por_hora` = lista de `{dia (0=domingo…6), hora ("HH:00"), lugares (0–500)}`, hasta 7 × 24. `lugares: 0` cierra esa hora para reservas. Sin entrada = `capacidad_por_horario`.
- **Rutas:** `GET /negocios/{id}/cupo` → `{dias[{dia, horas[{hora, lugares, afluencia}]}], capacidad_por_horario, version}` (solo horas dentro de `horarios`); `PUT /negocios/{id}/cupo` con `If-Match` y la lista completa (reemplaza; horas fuera de horario = 422; duplicados = 422). Dueño, como el resto del panel.
- **Disponibilidad (nv-59):** `lugares_libres` = cupo de esa hora y día − personas en reservas vigentes; los turnos que caen en dos horas usan el menor cupo. Agrega `afluencia` (`poco`, `moderado`, `pico`) por horario, de la curva de nv-66 para ese día.
- **Sugerencia:** la disponibilidad marca `sugerida: true` en la hora libre con menor afluencia cercana a la pedida (como «Te sugerimos 17:00»).
- **Mesas y máximo de personas (`45_NegOper`):** el alta (nv-55) no guarda «Mesas» ni «Máx. personas». Se agregan a `reservas_config` como `mesas` (1–200) y `max_personas` (1–2000), editables en `PUT /negocios/{id}/reservas-config`; el cupo de una hora no puede superar `max_personas` (422).

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/negocios/{id}/cupo` | JWT dueño | 200, 401, 403, 404 |
| PUT | `/api/v1/negocios/{id}/cupo` | JWT dueño | 200, 400, 401, 403, 404, 409, 422 |
| GET | `/api/v1/lugares/{id}/disponibilidad` | no | agrega `afluencia` y `sugerida` |

## Tests
- Unit: resolución de cupo (entrada, por omisión, 0 = cerrado, turno en dos horas).
- Integration: disponibilidad con cupos distintos por día; cambio con `If-Match` y 409; horas fuera de horario 422; sugerencia.
- Security: dueño ajeno 404; consumidor 403.
- Contract: cupo y disponibilidad nueva.

## Criterios
- Ninguna reserva nueva supera el cupo de su hora y día.

## Límites
- Sin cupos por fecha específica (para eso los bloqueos de nv-59) ni por zona (interior/terraza).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-21), sin ejecutar.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) cupo en lugares (personas), no en mesas, como el resto de reservas; (2) `PUT` reemplaza la tabla completa; (3) la sugerencia usa la afluencia histórica.
- **Detalle:** `specs/nv-88/checklists/gates.md` al ejecutar.
