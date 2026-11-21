# NV-86 — Notificaciones por categoría y avisos nuevos

## Contexto
La bandeja de la maqueta (`18_Notif`, `87_PushBloqueo`, `88_PushBanner`) tiene pestañas **Todo · Social · Reservas** y avisos que hoy no existen: «le dio me gusta a tu reseña», «comentó tu postal», «Cuéntanos cómo te fue» (reseña pendiente), «Tu cupón 2x1 vence en 3 días», «Mañana tienes reservación» y «Es hora de salir al Jardín del Cerro». `GET /notificaciones` no filtra y sus tipos son de amistad, planes, votaciones, negocio, mensajes, momentos y reservas. **Origen**: revisión de la maqueta (2026-11-21). **Depende de**: NV-40 (notificaciones y push), NV-61 (cupones), NV-62 (check-ins y reseñas), NV-59 (reservas), NV-83 (me gusta y comentarios), NV-05 (preferencias de notificaciones).

## Requisitos
1. Filtrar la bandeja por categoría.
2. Avisos nuevos: me gusta, comentario, reseña útil, reseña pendiente, cupón por vencer y hora de salir a la siguiente parada.
3. Respetar las preferencias y el horario silencioso de cada persona.

## Diseño
- **Categoría:** cada tipo pertenece a `social` (amistad, me gusta, comentario, etiqueta, momento agradecido/destacado, invitación a plan, mensajes, reseña útil), `reservas` (recordatorio, mensaje de reserva, pago, cambio) o `planes` (votaciones, hora de salir). `GET /notificaciones?categoria=social|reservas|planes` (sin parámetro = todo) y `GET /notificaciones/no-leidas` agrega `por_categoria`. Índice `ix_notif_categoria` `{usuario_id, categoria, _id}`.
- **Tipos nuevos:** `me_gusta` y `comentario` (de nv-83, agrupados por objeto cada 10 min: «Mariana y 3 más»), `resena_util` (nv-62), `resena_pendiente` (check-in válido sin reseña 2 h después, uno por check-in; el trabajo nocturno de nv-62 ya conoce los pendientes), `cupon_por_vencer` (cupón guardado sin canjear que vence en 3 días, una vez por cupón), `hora_de_salir` (plan del día con traslado: aviso cuando falta el tiempo de traslado + 5 min para la siguiente parada; solo con preferencia activa y sin pasar el horario silencioso).
- **Preferencias:** `preferencias.notificaciones` agrega `social`, `cupones`, `recordatorios_plan` (todas true por omisión); un tipo apagado no se guarda ni se envía.
- **Trabajos:** `resena_pendiente` y `cupon_por_vencer` en el trabajo nocturno existente (a las 10:00 locales, no a medianoche); `hora_de_salir` en una tarea cada 5 min que solo mira planes de hoy (índice `ix_dueno_estado_fecha`).
- **Datos:** como hoy, `datos` solo lleva ids; el texto lo arma la app.

## Contratos
| Método | Ruta | Cambio |
|---|---|---|
| GET | `/api/v1/notificaciones` | `?categoria=`; tipos nuevos |
| GET | `/api/v1/notificaciones/no-leidas` | agrega `por_categoria` |
| PATCH | `/api/v1/usuarios/yo/preferencias` | `notificaciones.social`, `cupones`, `recordatorios_plan` |

## Tests
- Unit: tipo → categoría; agrupado de me gusta; cálculo de la hora de salir.
- Integration: filtro por categoría y conteos; cada aviso nuevo una sola vez; preferencias apagadas; horario silencioso.
- Security: solo mis notificaciones; `datos` sin texto de terceros ni ubicación.
- Database: `ix_notif_categoria`. Contract: lista con categoría.

## Criterios
- Cada aviso nuevo llega una vez y solo si la persona lo tiene encendido.

## Límites
- Sin avisos por correo de estos tipos; sin resumen semanal.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-21), sin ejecutar.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) tres categorías; (2) «hora de salir» con una tarea cada 5 min; (3) reseña pendiente a las 2 h del check-in.
- **Detalle:** `specs/nv-86/checklists/gates.md` al ejecutar.
