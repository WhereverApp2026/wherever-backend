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

## Drift al ejecutar (2026-11-23)
- **Código:** modelo y reglas puras en `internal/negocios/cupo.go` (`CupoHora`, `HoraEntera`, `FormatoHora`, `MinutosHorario`, `ReservasConfig.CupoEn`, `HorasAbiertas`, `CrowdAt`; `CrowdLevel` de nv-85 usa los mismos umbrales); `ReservasConfig` ganó `mesas`, `max_personas` y `cupo_por_hora` (`omitempty`). Rutas en el panel (`internal/negocios/panel/cupo.go`, `Service.Cupo` y `Service.CambiarCupo`), sin cableado nuevo en `main.go`, `authtest` ni `scripts/routes` (el handler del panel ya estaba registrado). Disponibilidad en `internal/reservas/disponibilidad.go` (`CupoTurno`, `Disponibilidad` con el negocio y la configuración, `Sugerir`).
- **Días y medianoche:** `dia` es el día de la semana en que de verdad cae la hora (America/Mexico_City). Un tramo que cruza la medianoche (viernes 22:00–01:00) pone la hora 00:00 en el sábado, tanto en la tabla como en la disponibilidad. GET devuelve siempre los 7 días; un día cerrado va con `horas: []`.
- **Horas fuera de horario:** el PUT las rechaza (422). Si después el dueño cambia `horarios`, las entradas que quedaron fuera no se borran: no salen en GET ni se usan, porque no hay turnos en esas horas.
- **Sin «Horas y cupo»:** GET responde con todo en 0 y PUT da 422 (`cupo_por_hora`: primero guarda reservas-config). `PUT /reservas-config` conserva `cupo_por_hora` y, si no vienen, `mesas` y `max_personas`; con `max_personas` fijado, `capacidad_por_horario` o una hora del cupo por encima = 422 (la spec solo pedía el tope en el cupo; la capacidad general es el cupo por omisión y se trata igual).
- **Reservar:** crear, cambiar y confirmar un pago tardío (nv-60) usan el cupo del turno (`CupoTurno`, el menor de las horas que toca) dentro del mismo candado `lock:reserva:{negocio}:{hora}`; la idempotencia y el 503 con Redis caído no cambian. Sin comandos de Redis nuevos.
- **Afluencia y sugerencia:** `afluencia` es la de la hora local en que empieza el turno, con los umbrales de Explorar (0–33 `poco`, 34–66 `moderado`, 67–100 `pico`); null si el negocio no tiene curva o el día está en ceros. La disponibilidad aceptó `hora=HH:MM` opcional (la hora pedida): `sugerida` marca a lo más un horario, el disponible con menor afluencia a ±2 h de la pedida; sin `hora`, en todo el día; empate, el más cercano a la pedida y luego el más temprano. Sin curva no hay sugerencia.
- **Índices y variables:** ninguno nuevo.
- **Pruebas:** Unit `internal/reservas/disponibilidad_test.go` (`TestCupoTurno_*`, `TestDisponibilidad_CupoPorHoraYAfluencia`, `TestSugerir_*`) e `internal/negocios/panel/cupo_test.go`; Integration `tests/integration/reservas_cupo_test.go`; Security `tests/security/reservas_cupo_security_test.go`; Contract `tests/contract/reservas_cupo_contract_test.go` (`reservas/disponibilidad_200`) y en `TestContract_NegociosPanel` (`negocios/cupo_200`, `negocios/cupo_put_200`, `negocios/reservas_config_200` con `mesas` y `max_personas`).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-23), ejecutada; ver `checklists/gates.md`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) cupo en lugares (personas), no en mesas, como el resto de reservas; (2) `PUT` reemplaza la tabla completa; (3) la sugerencia usa la afluencia histórica. Se ejecutó con las tres como están.
- **Detalle:** `specs/nv-88/checklists/gates.md`.
