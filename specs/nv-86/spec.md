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

## Drift al ejecutar (2026-11-23)
- **Código:** `internal/notificaciones/categoria.go` (tipo → categoría, `KindsOf`, interruptores `Storable`), `barrido.go` (tarea cada 5 min, interfaz `Unico`), `Agrupador.Once`; tareas en su módulo: `resenas.AvisoPendiente` (`internal/resenas/avisos.go`), `cupones.AvisoPorVencer` (`internal/cupones/avisos.go`) y `planes.AvisoSalida` (`internal/planes/salida.go`), conectadas en `cmd/api/avisos.go`. Preferencias en `usuarios.Notifications` (`social`, `recordatorios_plan`; `cupones` ya existía).
- **Categoría sin campo guardado:** la pestaña se resuelve a tipos con `tipo $in` y el índice es `ix_notif_categoria` = `{usuario_id, tipo, creada_en: -1, _id: -1}` (no `{usuario_id, categoria, _id}`): los avisos de antes entran sin migrar y el mapa se puede cambiar en código. El orden sigue siendo `creada_en` y el cursor queda atado a la categoría (otro = 422).
- **Categorías:** `social` = amistad, invitación a plan, mensajes de grupo, momentos, me gusta, comentario y útil; `reservas` = mensaje y recordatorio de reserva (los tipos de pago y cambio aún no existen); `planes` = votaciones y hora de salir. `negocio_revisado`, `resena_pendiente` y `cupon_por_vencer` no tienen pestaña: salen solo en «todo» con `categoria: null` y cuentan solo en `total`.
- **Agrupado de me gusta:** ya lo hacía nv-83 (`Agrupador`, 10 min por objeto); el texto «Mariana y 3 más» lo arma la app con `GET /{objeto}/{id}/reacciones`. Sin cambio aquí.
- **Interruptores:** `social` apaga me gusta, comentario, útil y los avisos de momentos (no las solicitudes de amistad, invitaciones ni mensajes); `cupones` apaga `cupon_por_vencer`; `resenas` (existente) apaga `resena_pendiente`; `recordatorios_plan` apaga `hora_de_salir`. Apagado = no se guarda ni se envía (el centro lee a la persona solo para esos tipos; si no puede leerla, no guarda). `grupos` sigue callando solo el push. `social` y `recordatorios_plan` ausentes = encendidos.
- **Trabajos:** los tres avisos van en una tarea cada 5 min por instancia (no en el trabajo nocturno): la reseña a las 2 h y la hora de salir no caben en una corrida diaria. Cada aviso se reserva una vez en `notificaciones_agrupadas` (30 días, TTL), así que las dos instancias no lo repiten y no se usa Redis. Cupón por vencer solo de 10:00 a 21:59 de México; reseña pendiente para check-ins de hace 2 a 24 h; hora de salir solo a quien hizo el plan, con planes activos de hoy (`ix_estado_fecha`, no `ix_dueno_estado_fecha`, porque mira a todas las personas).
- **Canal del teléfono (nv-71):** `cupon_por_vencer` → `cupones`, `resena_pendiente` → `resenas`, `hora_de_salir` → `grupos`.
- **Índices nuevos:** `ix_notif_categoria` (notificaciones) e `ix_checkins_pendientes` = `{creado_en: 1}` parcial con `valido: true` (checkins). Hay que crearlos en Atlas (`go run ./scripts/indexes`) antes del despliegue.
- **Pruebas:** Unit `internal/notificaciones/categoria_test.go`, `internal/planes/salida_test.go`, `internal/cupones/avisos_test.go`; Integration `tests/integration/notificaciones_categorias_test.go`; Security `TestNotificacionesSecurity_CategoriasSoloMiasYSoloIDs`; Contract `tests/contract/notificaciones_contract_test.go` (`notificaciones/lista_social_200`, `notificaciones/no_leidas_200`) y las preferencias de `usuarios/*`; Database los dos índices en `tests/database/indexes_test.go`. Sin Redis nuevo.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-23), ejecutada; ver `checklists/gates.md`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) tres categorías, con reseña pendiente, cupón por vencer y negocio revisado solo en «todo»; (2) «hora de salir» con una tarea cada 5 min; (3) reseña pendiente a las 2 h del check-in; (4) qué apaga `social`.
- **Detalle:** `specs/nv-86/checklists/gates.md`.
