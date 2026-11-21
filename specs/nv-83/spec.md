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

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-21), sin ejecutar.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) un solo tipo de reacción (me gusta); (2) respuestas de un nivel; (3) «me gusta a tu reseña» = «Útil»; (4) avisos agrupados cada 10 min.
- **Detalle:** `specs/nv-83/checklists/gates.md` al ejecutar.
