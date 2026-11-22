# NV-83 — Me gusta y comentarios

## Contexto
La maqueta tiene **me gusta** y **comentarios** en publicaciones y planes: `17_PostDetalle` (comentarios con respuestas y me gusta), `12_InicioMas` («24 · 6»), `13_Reacciones` («Todos · 11 · Me gusta · 8 · Van · 4»), `29_PlanAmigo` («Ver los 4 comentarios») y avisos `18_Notif` («le dio me gusta a tu reseña», «comentó tu postal»). Los momentos tienen `reacciones_total` en 0 «hasta el módulo social B2»; no hay comentarios. Las reseñas solo tienen «Útil». **Origen**: revisión de la maqueta (2026-11-21). **Depende de**: NV-63 (momentos), NV-30/NV-69 (planes y «me apunto»), NV-62 (reseñas), NV-42/NV-71 (moderación), NV-40 (avisos), NV-82 (privacidad).

## Requisitos
1. Dar y quitar me gusta a momentos y planes visibles para mí.
2. Comentar momentos y planes; responder un comentario (un nivel); borrar los propios.
3. Ver quién reaccionó, con pestañas Todos, Me gusta y Van (planes).
4. El autor recibe un aviso; los textos pasan por moderación.

## Diseño
- **Me gusta:** colección `me_gusta` `{objeto_tipo (momento|plan), objeto_id, usuario_id, creado_en}` con índice único `ux_me_gusta`. `PUT` y `DELETE /momentos/{id}/me-gusta` y `/planes/{id}/me-gusta` (idempotentes, 204). `reacciones_total` del objeto con `$inc` solo si la inserción no es duplicada.
- **Comentarios:** colección `comentarios` `{objeto_tipo, objeto_id, autor_id, padre_id?, texto (1–300), estado, me_gusta_total, creado_en, eliminado_en?}`. `GET /momentos/{id}/comentarios` y `/planes/{id}/comentarios` (cursor, más viejo primero, con respuestas anidadas de un nivel), `POST` (moderación: rechazado = 422; dudoso = `en_moderacion`, solo lo ve su autor hasta decidir en la cola de nv-43), `DELETE /comentarios/{id}` (autor o dueño del objeto; queda «Comentario eliminado»).
- **Quién reaccionó:** `GET /momentos/{id}/reacciones` y `/planes/{id}/reacciones?pestana=todos|me_gusta|van` → personas con `relacion` (como la búsqueda) y amigos en común; «van» = apuntados del plan (nv-69).
- **Visibilidad:** solo se reacciona o comenta lo que puedo ver (momento por su visibilidad, plan por nv-84/nv-82); un bloqueo oculta y prohíbe en los dos sentidos. Límite `social_escrituras` 60 por hora por usuario.
- **Reseñas:** «me gusta a tu reseña» de la maqueta = «Útil» (nv-62); se agrega el aviso al autor (`resena_util`).
- **Avisos (nv-86):** `me_gusta` y `comentario` al autor, agrupados por objeto cada 10 min (no uno por toque).
- **ARCO:** exporta y borra me gusta y comentarios propios; al borrar la cuenta, los comentarios quedan como «Comentario eliminado» sin autor.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| PUT/DELETE | `/api/v1/momentos/{id}/me-gusta` | JWT, cuenta activa | 204, 401, 403, 404, 429 |
| PUT/DELETE | `/api/v1/planes/{id}/me-gusta` | JWT, cuenta activa | 204, 401, 403, 404, 429 |
| GET/POST | `/api/v1/momentos/{id}/comentarios` | JWT, cuenta activa | 200/201, 401, 403, 404, 422, 429 |
| GET/POST | `/api/v1/planes/{id}/comentarios` | JWT, cuenta activa | 200/201, 401, 403, 404, 422, 429 |
| DELETE | `/api/v1/comentarios/{id}` | JWT, autor o dueño | 204, 401, 403, 404 |
| GET | `/api/v1/momentos/{id}/reacciones`, `/api/v1/planes/{id}/reacciones` | JWT | 200, 401, 404 |

## Tests
- Integration: me gusta idempotente y concurrente; contador exacto; comentarios con respuesta, moderación (rechazado, dudoso), borrado por autor y por dueño; pestañas de reacciones; avisos agrupados; ARCO.
- Security: sin visibilidad o con bloqueo = 404; no se borra comentario ajeno de objeto ajeno; límite 429; sin correo ni teléfono en las listas.
- Database: `ux_me_gusta` y el índice de comentarios por objeto. Contract: las seis rutas.

## Criterios
- Un me gusta cuenta una vez por persona; nadie interactúa con lo que no puede ver.

## Límites
- Sin emojis variados en momentos (solo me gusta), sin menciones con aviso, sin comentarios en reseñas.

## Drift al ejecutar (2026-11-22)
- **Módulo propio:** `internal/reacciones` (modelo, repositorio, service, handler, destino de la cola y ARCO). No decide permisos: pregunta a `postales.Service.Visible` (las reglas del detalle del momento con sesión: visibilidad, bloqueo, moderación y `compartir_postales`/`actividad` de la autora) y a `planes.Service.Visible` (dueño, integrante del grupo o `publico` sin bloqueo cuyo dueño comparte sus planes, nv-82). Va antes que grupos y planes en el armado (el grupo `/planes` de planes pide sesión para todo su prefijo).
- **Planes por nv-84:** nv-84 aún no existe; la visibilidad `amigos` de los planes llega con ella. Hoy un plan se ve si es propio, del grupo o `publico`.
- **Contadores:** los momentos ya tenían `reacciones_total` (me gusta) y ganan `comentarios_total`; los planes ganan `me_gusta_total` y `comentarios_total` (los nombres de nv-84) en su respuesta. Solo los mueve `reacciones` con `$inc`: el me gusta solo si la inserción no es duplicada y el comentario solo cuando está (o pasa a) `publicado`. Las ediciones del plan y del momento hacen `$set` de sus campos y no los tocan.
- **`dueno_id` en `me_gusta` y `comentarios`:** el dueño del objeto al reaccionar. Sirve para que la dueña borre comentarios, para avisarle cuando la cola aprueba un comentario y para que ARCO borre lo que recibieron sus objetos al eliminar su cuenta.
- **Sin `me_gusta_total` en comentarios:** el contrato no tiene ruta para dar me gusta a un comentario; queda fuera (como emojis y menciones).
- **Avisos agrupados:** `notificaciones.Agrupador` reserva en MongoDB (`notificaciones_agrupadas`, `_id` = persona:tipo:objeto, TTL en `hasta`) el aviso de cada persona, tipo y objeto por 10 min: el primer toque avisa y los demás del lapso no (no hay resumen «Ana y 3 más»). Sin Redis, así que no cambia el costo de `TestPresupuestoRedis_*`. Tipos nuevos `me_gusta`, `comentario` y `resena_util` con texto de push genérico; el aviso solo lleva `momento_id`, `plan_id` o `resena_id`. Nunca avisa a sí misma.
- **`actividad: nadie` (nv-82):** quien la tiene no sale en «quién reaccionó» ni sus comentarios para terceros; sí para la dueña del objeto y para sí misma. Los totales sí la cuentan (son números).
- **Quién reaccionó:** sin cursor; trae los 100 me gusta más recientes y luego a quienes van. `relacion` agrega `yo`; `amigos_en_comun` sale de una sola consulta a `amistades` (`CommonWith`) sobre los amigos de quien mira, sin Neo4j. `todos` = me gusta + van − los que hicieron las dos cosas. En momentos `van` siempre vacía. Pestaña desconocida = 422. Las tres rutas de lectura también responden 403 (cuenta no activa) y 422 (cursor o pestaña).
- **Comentario eliminado:** queda sin texto y sin autor en la respuesta; sale mientras tenga respuestas visibles y deja de salir cuando no (ocupa lugar en la página: una página puede traer menos que `limite`). Una respuesta eliminada no sale. Rechazado por la cola = solo lo ve su autora con `estado: rechazado`.
- **Borrar:** inexistente, ya eliminado o de un objeto que no veo = 404; lo veo y no soy autora ni dueña = 403. Borrar no lleva límite propio (no estaba en el contrato con 429).
- **Índices nuevos:** `me_gusta`: `ux_me_gusta`, `ix_me_gusta_objeto`, `ix_me_gusta_usuario`, `ix_me_gusta_dueno`; `comentarios`: `ix_comentarios_objeto` `{objeto_tipo, objeto_id, padre_id, creado_en, _id}`, `ix_comentarios_autor`, `ix_comentarios_dueno`; `notificaciones_agrupadas`: `ix_agrupada_usuario`, `ttl_agrupada`. Hay que crearlos en Atlas con `go run ./scripts/indexes` antes del despliegue (`/ready` da 503 si falta uno).
- **ARCO:** exporta `reacciones{me_gusta[], comentarios[]}` (sin eliminados). Al borrar la cuenta quita sus me gusta (bajando cada contador), deja sus comentarios como «Comentario eliminado» sin `autor_id` y borra los me gusta y comentarios que recibieron sus objetos; también sus reservas de avisos agrupados.
- **Fuera por ahora:** saber en el feed si ya di me gusta (`yo.me_gusta` llega con nv-84 en sus listas); la app lo sabe por el 204 y por `reacciones`. 271 rutas (11 nuevas).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-22): ejecutada; ver `checklists/gates.md`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) un solo tipo de reacción (me gusta) — tomada así; (2) respuestas de un nivel — tomada así; (3) «me gusta a tu reseña» = «Útil», que ahora avisa `resena_util` — tomada así; (4) avisos agrupados cada 10 min (el primero avisa, los demás del lapso no) — tomada así.
- **Detalle:** `specs/nv-83/checklists/gates.md` al ejecutar.
