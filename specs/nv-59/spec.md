# NV-59 — Reservas y disponibilidad

## Contexto
La persona elige día, hora y personas; el servidor aparta el cupo sin que dos personas ganen el mismo horario. El negocio ve su agenda, confirma, marca asistencia, escanea el código y bloquea horas. La cancelación calcula el reembolso según la política. **Origen**: parte de reservas de la hoja «J5 Reservas y pagos» (roles C5 y C13, backend B10/B11; pantallas P-21, P-23, P-24 y P-48) y `reservas_config` de A2. **Depende de**: NV-20 (negocios y horarios), NV-55 (`PUT /negocios/{id}/reservas-config`, panel del negocio), NV-30 (planes y paradas). **Opcional**: NV-37 (`grupo_id`). **La completa**: NV-60 (anticipo con Stripe y reembolso real).

## Requisitos
1. `GET /lugares/{id}/disponibilidad?fecha=&personas=` da los horarios del día con cupo y marca los llenos.
2. `POST /reservas` (con `Idempotency-Key`) aparta el cupo con un candado en Redis; sin anticipo, queda confirmada (o pendiente si el negocio confirma a mano); con anticipo, queda `bloqueada` 10 minutos mientras se paga (nv-60).
3. `GET /reservas` y `GET /reservas/{id}`: mis reservas próximas o pasadas, y el detalle con código, reembolso posible y si se puede cambiar.
4. `PATCH /reservas/{id}` cambia hora o personas si el negocio lo permite, revisando cupo.
5. `GET /reservas/{id}/reembolso` calcula el reembolso antes de cancelar; `POST /reservas/{id}/cancelar` cancela y libera el cupo.
6. El negocio ve su agenda (`GET /negocios/{id}/reservas`), confirma o marca asistencia (`PATCH /negocios/{id}/reservas/{rid}`), escanea el código (`POST .../escanear`) y bloquea horas (`POST`/`DELETE /negocios/{id}/bloqueos`).
7. Los apartados no pagados a tiempo se liberan y las confirmadas sin llegada quedan `no_asistio` al terminar el día.

## Diseño
- **Paquete `internal/reservas`:** modelo, repositorio, service, handler, DTO, `disponibilidad.go` y `reembolso.go` (puros), barrido y `Privacy`. Lee `negocios` con el repositorio de `internal/negocios` (horarios, `propietario_id`, `acepta_reservas`, `reservas_config`).
- **`negocios.reservas_config`** (A2; lo escribe nv-55): `capacidad_por_horario` (personas), `duracion_turno_min` (15 a 240; también es el intervalo entre horarios), `anticipo {tipo: fijo|porcentaje|por_persona, valor}`, `politica_cancelacion` (`flexible`, `moderada`, `estricta`). Esta spec propone a nv-55 dos campos más: `confirmacion` (`automatica`, `manual`) y `permite_cambios` (bool). Sin `acepta_reservas` o sin `reservas_config`: disponibilidad vacía y `POST` = 422 `no_acepta_reservas`.
- **Colección `reservas`** (J5 §1): `codigo`, `usuario_id`, `negocio_id`, `plan_id?`, `parada_id?` (ObjectId, como `planes.paradas.id`), `grupo_id?`, `fecha_hora` (UTC), `personas` (1 a 20), `nota?` (≤ 140), `estado`, `bloqueo_expira_en?`, `anticipo {monto_centavos, tipo}`, `politica_cancelacion` (copia al reservar: `tipo`, `horas_gratis`, `porcentaje_reembolso`), `pago_id?` (lo llena nv-60), `historial[{estado, fecha, por}]`, `version`, `creado_en`, `actualizado_en`. Índices de J5: `ux_codigo`, `ix_mis_reservas`, `ix_agenda_negocio`, `ix_bloqueos` (parcial `estado = bloqueada`) e `ix_recordatorios`.
- **Estados:** `bloqueada` (espera el pago, 10 min) → `confirmada`; `pendiente` (espera que el negocio confirme) → `confirmada`; `confirmada` → `asistio` | `no_asistio`; `bloqueada` → `expirada`; `bloqueada`, `pendiente` o `confirmada` → `cancelada`. Cada cambio va a `historial` con filtro condicional sobre el estado esperado.
- **Colección `bloqueos_horario`** (J5 §3): `negocio_id`, `desde`, `hasta` (`desde < hasta`, máximo 31 días), `motivo` (`evento_privado`, `lleno`, `otro`), `creado_por`, `creado_en`. Índice `ix_bloqueo_negocio`.
- **Disponibilidad:** horarios del día (zona `America/Mexico_City`, como el resto del proyecto) dentro de `horarios` del negocio, cada `duracion_turno_min`. Cupo = `capacidad_por_horario` − personas en reservas `confirmada`, `pendiente` o `bloqueada` vigente (`bloqueo_expira_en > ahora`) de ese horario; 0 si un bloqueo del negocio lo cubre o si ya pasó. Respuesta `[{hora, disponible, lugares_libres}]`; `disponible` = `lugares_libres ≥ personas`. Fecha hasta 60 días adelante.
- **Crear sin choques:** `fecha_hora` debe caer en un horario de la disponibilidad. Dentro de `lock:reserva:{negocio_id}:{fecha_hora}` (TTL 5 s) se cuenta el cupo y se inserta; si no alcanza, 409 `horario_ocupado` ("Alguien ganó ese horario"). Redis caído = 503 (fail-closed). El candado es solo para contar; el apartado de 10 minutos vive en Mongo (`bloqueo_expira_en`).
- **Idempotencia:** `Idempotency-Key` (UUID, obligatoria) en `POST /reservas` → `idem:reserva:{usuario_id}:{llave}` (TTL 24 h) guarda el id de la reserva creada; repetir devuelve la misma reserva con 200. Llave en uso por otra petición en curso = 409. Redis caído = 503.
- **Anticipo:** se calcula en el servidor al reservar (`fijo` = valor; `porcentaje` = valor % de `precio_promedio × personas`; `por_persona` = valor × personas). Mientras nv-60 no esté en `main`, un negocio con anticipo > 0 responde 422 `anticipo_no_disponible`.
- **Código:** 6 caracteres de `ABCDEFGHJKLMNPQRSTUVWXYZ23456789` (sin O, I, 0, 1), generado con `crypto/rand`; choque en `ux_codigo` = se genera otro. El QR lo dibuja la app con el mismo código; el backend no genera imágenes.
- **Quién:** crear y cambiar = rol `consumidor`, cuenta activa (`usuarios.RequireActive`) y `correo_verificado` (si no, 403 `correo_no_verificado`). Dueño del negocio = rol `comerciante` y `negocios.propietario_id` igual (se valida en el service). Reserva ajena = 404. Hasta 3 reservas `bloqueada` vigentes por persona (4.ª = 409).
- **Cambiar:** solo `confirmada` o `pendiente`, con `permite_cambios`, al menos 2 h antes y con `If-Match`; revisa cupo con el mismo candado. Con anticipo, cambiar `personas` = 422 (el cobro ya está hecho; lo resuelve nv-60).
- **Reembolso (`reembolso.go`):** flexible = 100 % hasta 2 h antes, 0 % después; moderada = 100 % hasta 24 h, 50 % después; estricta = 50 % hasta 48 h, 0 % después. Si cancela el negocio, siempre 100 %. Sin pago, el reembolso es 0. Cancelar llama al puerto `Refunder` que entrega nv-60; hasta entonces no hay pagos que devolver.
- **Agenda del negocio:** por día, `estado` y `q` (nombre del cliente sin acentos, o inicio del código), ordenada por hora; muestra nombre, personas, nota, anticipo y estado. Sin correo ni teléfono del cliente.
- **Escanear:** busca por `codigo` exacto; debe ser de ese negocio y del día de hoy; `confirmada` → `asistio`; otro negocio, otro día o código inexistente = 404 igual; ya usado = 409. El check-in de J7 se agrega cuando exista su spec.
- **Barrido:** cada 5 minutos en cada instancia, solo con Mongo y sin candado de Redis (las actualizaciones son condicionales e idempotentes, así que correr en dos instancias no hace daño y no gasta Upstash): `bloqueada` vencida → `expirada`; `confirmada` del día anterior o antes → `no_asistio`. El cupo no depende del barrido: el conteo ya ignora apartados vencidos.
- **Límites de uso:** `POST /reservas` 10 por hora por usuario (`rate:reservas:{usuario}`, ventana fija; nueva fila en `docs/rate-limit-matrix.md`). `GET /lugares/{id}/disponibilidad` usa solo el límite global.
- **ARCO:** al borrar la cuenta se cancelan las reservas futuras (el negocio deja de verlas) y las pasadas se conservan sin nota, porque los pagos que cuelgan de ellas se guardan 5 años (nv-60). "Borrar historial" no las toca. La exportación incluye `reservas` (negocio, fecha, personas, estado, código).
- **Variables:** ninguna nueva.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/lugares/{id}/disponibilidad?fecha=&personas=` | Público | 200, 404, 422 |
| POST | `/api/v1/reservas` → `{negocio_id, fecha_hora, personas, nota?, plan_id?, parada_id?, grupo_id?}`, `Idempotency-Key` | JWT consumidor | 200, 201, 400, 401, 403, 404, 409, 422, 429, 503 |
| GET | `/api/v1/reservas?cuando=proximas\|pasadas&estado=&cursor=` | JWT | 200, 401, 403, 422 |
| GET | `/api/v1/reservas/{id}` | JWT dueño o negocio | 200, 401, 403, 404 |
| PATCH | `/api/v1/reservas/{id}` → `{fecha_hora?, personas?}`, `If-Match` | JWT dueño | 200, 400, 401, 403, 404, 409, 422, 503 |
| GET | `/api/v1/reservas/{id}/reembolso` → `{reembolso_centavos, politica}` | JWT dueño | 200, 401, 403, 404 |
| POST | `/api/v1/reservas/{id}/cancelar` → `{motivo?}` | JWT dueño o negocio | 200, 401, 403, 404, 409 |
| GET | `/api/v1/negocios/{id}/reservas?fecha=&estado=&q=` | JWT dueño del negocio | 200, 401, 403, 404, 422 |
| PATCH | `/api/v1/negocios/{id}/reservas/{rid}` → `{estado: confirmada\|asistio\|no_asistio}` | JWT dueño del negocio | 200, 400, 401, 403, 404, 409, 422 |
| POST | `/api/v1/negocios/{id}/reservas/escanear` → `{codigo}` | JWT dueño del negocio | 200, 400, 401, 403, 404, 409 |
| POST | `/api/v1/negocios/{id}/bloqueos` → `{desde, hasta, motivo?}` | JWT dueño del negocio | 201, 400, 401, 403, 404, 422 |
| DELETE | `/api/v1/negocios/{id}/bloqueos/{bid}` | JWT dueño del negocio | 204, 401, 403, 404 |

403 = cuenta en eliminación pendiente, rol equivocado o correo sin verificar. Sin `If-Match` = 400; versión vieja = 409. Filas para `docs/endpoints.md` al implementar.

**Drift corregido contra el Excel:**
- J5 trae `bloqueada` y `pendiente_pago` para lo mismo; queda `bloqueada`, y `pendiente` pasa a ser "espera confirmación del negocio", que J5 pide ("Confirmar si es manual") sin estado propio.
- J5 pone `parada_id` como string; en `planes` es ObjectId.
- Ejemplo de código `WH4821` (10 000 combinaciones con índice único global): se cambia a 6 caracteres de 32 símbolos.
- J5 pide un trabajo cada minuto con candado; aquí es cada 5 minutos, solo Mongo, sin gastar Upstash, y el cupo no depende de él.
- `reservas_config` (A2) no dice si confirmar es manual ni si se permiten cambios; se proponen `confirmacion` y `permite_cambios` a nv-55.
- `PATCH /reservas/{id}` no lleva versión en J5; se agrega `If-Match`.
- Pasan a nv-60: `POST /reservas/{id}/pago`, `POST /webhooks/stripe`, `GET /pagos/{id}/estado` y `GET /negocios/{id}/reservas/{rid}/cobro`.
- Fuera: `cupon_usuario_id` (J6), `checkin_id` (J7), `POST /negocios/{id}/reservas/{rid}/mensajes` y el recordatorio de 2 h (esperan nv-40 y nv-41).

## Tests
- Unit `internal/reservas`: `disponibilidad_test.go` (horarios, bloqueos, apartados vencidos, día cerrado, cambio de horario de verano no aplica en CDMX), `reembolso_test.go` (las tres políticas en sus bordes, cancelación del negocio), código (alfabeto y longitud), DTOs e entradas inválidas.
- `tests/integration/reservas_test.go` con MongoDB y Redis reales: disponibilidad, reservar confirmada y pendiente, `Idempotency-Key` repetida (200, un documento), sin llave (400), correo sin verificar (403), negocio sin reservas (422), con anticipo antes de nv-60 (422), cambiar con y sin cupo, cancelar y reembolso, agenda con `q`, confirmar y marcar asistencia, escanear (hoy, otro día, otro negocio, ya usado), bloqueo de horas, barrido, borrar cuenta y exportar, límite 10/h (11.ª = 429).
- `TestReservas_Concurrencia_UnSoloGanador`: 20 personas piden el último lugar a la vez en 3 rondas; gana exactamente una por ronda y el cupo nunca queda negativo.
- `tests/security/reservas_security_test.go`: las doce rutas con 401 sin token (menos disponibilidad), 403 con rol equivocado o cuenta en eliminación pendiente, 404 en negocio ajeno; Redis caído en `POST /reservas` = 503.
- `tests/database/indexes_test.go`: índices de `reservas` y `bloqueos_horario`. `tests/contract`: `reservas: []` en la exportación.

## Criterios
- Nunca hay más personas reservadas que la capacidad de un horario.
- Reenviar la misma petición nunca crea dos reservas.

## Límites
- Sin pago con tarjeta (nv-60), cupones (J6), check-ins (J7), mensajes ni recordatorios (nv-40, nv-41).
- Sin lista de espera ni reservas recurrentes.
- El QR lo dibuja la app.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-12), ola por decidir; requiere nv-55 (`reservas_config`) en `main`
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** estados `bloqueada` y `pendiente` en lugar de `pendiente_pago`; `confirmacion` y `permite_cambios` en `reservas_config` (a cargo de nv-55); barrido cada 5 min solo con Mongo; código de 32 símbolos; tope de 3 apartados y 10 reservas por hora; reservas pasadas conservadas al borrar la cuenta; anticipo 422 hasta nv-60.
- **Detalle:** la evidencia irá en `specs/nv-59/checklists/gates.md` + `Memoria.md`.
