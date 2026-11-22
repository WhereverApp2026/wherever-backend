# NV-84 — Inicio social: planes de amigos e historias

## Contexto
El inicio de la maqueta tiene **«Planes de tus amigos»** (`10_Inicio`, `11_InicioAmigos`: «Mariana armó un plan · Sábado de cafés · 3 paradas · $420 por persona · Jorge y 3 más van · Me apunto · Hacer mi versión», «Falta tu voto») e **historias** de amigos (`16_Historia`: «Jorge · Hace 3 h · Fonda Doña Meche · Verificado», con «Hacer mi versión» y «Responde a Jorge…»). Hoy existen «me apunto», clonar plan, votaciones y el feed de momentos, pero no una lista de planes de amigos ni historias. **Origen**: revisión de la maqueta (2026-11-21). **Depende de**: NV-30/NV-68/NV-69 (planes, populares, me apunto, clonar), NV-39 (votaciones), NV-63 (momentos), NV-82 (privacidad), NV-83 (me gusta), NV-87 (respuesta a historia por mensaje directo).

## Requisitos
1. Ver los planes próximos y recientes de mis amigos que ellos compartieron conmigo.
2. Ver historias de amigos de las últimas 24 h agrupadas por persona.
3. Ver en el inicio las votaciones abiertas donde falta mi voto.

## Diseño
- **Visibilidad de un plan:** se usa el campo `visibilidad` de nv-68 (`privado`, `amigos`, `publico`); «Visible para mis amigos» de `CrearPlan` = `amigos`. Solo con `compartir_planes` y `actividad` ≠ `nadie` (nv-82).
- **`GET /planes/amigos`** (cursor, 20) → planes activos de amigos aceptados con fecha de hoy a 30 días o creados en los últimos 7 días, sin silenciados (nv-70) ni bloqueos: `{id, nombre, fecha, autor{id, nombre, nombre_usuario, foto_url}, paradas[{hora, lugar{id, nombre}}], costo_por_persona, apuntados{total, amigos[≤3]}, me_gusta_total, comentarios_total, yo{apuntado, me_gusta}}`. Sin integrantes completos ni grupo.
- **`GET /momentos/historias`** → `{personas[{autor{…}, momentos[{id, imagen_url, negocio{id, nombre}, verificado, creado_en}]}]}` de amigos, momentos de las últimas 24 h visibles para mí, la persona con historia más reciente primero, hasta 30 personas. «Verificado» = el momento viene de un check-in válido. Sin «visto» por ahora.
- **Respuesta a historia:** la app abre el mensaje directo (nv-87) con el `momento_id` adjunto; sin nv-87 el botón no aparece.
- **Falta tu voto:** `GET /votaciones/pendientes` → votaciones abiertas de mis grupos donde no he votado `{id, grupo{id, nombre}, cierra_en}`.
- **Costo:** consultas con `ix_de_estado`/`ix_para_estado` para amigos (≤ 500) y un índice nuevo `ix_amigos_feed` `{dueno_id, visibilidad, estado, fecha}` en planes.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/planes/amigos` | JWT, cuenta activa | 200, 401, 403, 422 |
| GET | `/api/v1/momentos/historias` | JWT, cuenta activa | 200, 401, 403 |
| GET | `/api/v1/votaciones/pendientes` | JWT, cuenta activa | 200, 401, 403 |

## Tests
- Integration: planes por visibilidad, fecha, silencio y bloqueo; apuntados y me gusta; historias de 24 h agrupadas; votaciones pendientes.
- Security: un plan `privado` o de un no amigo nunca sale; `actividad: nadie` y `compartir_planes=false` lo ocultan; sin integrantes ni ubicación de terceros.
- Database: `ix_amigos_feed`. Contract: las tres rutas.

## Criterios
- Nadie ve un plan o una historia que su autor no compartió con él.

## Límites
- Sin «visto por», sin historias de no amigos, sin anuncios en el feed.

## Drift al ejecutar (2026-11-22)
- **Visibilidad `amigos`:** nv-68 solo tenía `privado` y `publico`; nv-84 agrega `amigos` (`planes.VisibilityFriends`) en `POST /planes` (campo nuevo `visibilidad`, por omisión `privado`) y `PATCH /planes/{id}`. La regla vive en un solo lugar, `planes.Service.sharedPlan`: `publico` sin bloqueo o `amigos` con amistad aceptada y dueño con `compartir_planes` y `actividad` ≠ `nadie`. La usan `GET /planes/{id}`, `POST /planes/{id}/clonar` («Hacer mi versión») y `Visible` (me gusta y comentarios de nv-83). `POST /planes/{id}/me-apunto` (nv-69) acepta `amigos` (`Plan.SharedWithFriends`). Populares y la pestaña de planes del perfil siguen solo con `publico`.
- **Planes de amigos:** `internal/planes/amigos.go`. Orden por `fecha` ascendente y `_id` (el cursor firmado usa la pestaña `amigos`); la ventana usa el día de hoy en México: fecha en [hoy, hoy + 31 días) o `creado_en` en los últimos 7 días. Silenciados (`silenciado_por`, nv-70) fuera; los bloqueos ya no son amistad aceptada. Las fichas de autor y de apuntados salen de una sola consulta a `usuarios` (`FindPublicByIDs`) de las amistades; quien no comparte planes u oculta su actividad no entra a la consulta de planes. `apuntados.total` cuenta a los confirmados del grupo sin el dueño (`grupos.Repository.GoingTo`, `ux_plan`), y con nombre solo salen hasta 3 amistades de quien mira que no ocultan su actividad; quien mira va en `yo.apuntado`. `yo.me_gusta` sale de `reacciones.Repository.LikedPlans` (`ux_me_gusta`). Las paradas llevan `hora` y `lugar{id, nombre}` (null si el lugar ya no es visible); sin notas, precios por parada, integrantes, grupo ni `zona`. También responde 503 sin las dependencias.
- **Historias:** `internal/postales/historias.go`, sin colección nueva. Momentos publicados `amigos` o `publica` con postal `lista` de las últimas 24 h (`ix_feed`, hasta 300), amistades sin silenciar con `compartir_postales` (las reglas de `autores` de nv-82). Dentro de cada persona, de la más vieja a la más nueva (orden de reproducción). `autor` es la `AutorView` de momentos (`{id, nombre_usuario, foto_url}`). «Verificado» = check-in válido de la autora en ese negocio el mismo día de México (`resenas.Repository.Validas`, `ux_checkin_dia`); el momento no guarda el check-in.
- **Votaciones pendientes:** `internal/votaciones/pendientes.go`. Revisa los 100 grupos vivos más recientes de quien llama (`ListForMember`); cuenta si está invitada o confirmada (quien declinó no vota) y sin sorpresa activa que le oculte las paradas. Votaciones `abierta` con `cierra_en` futuro y sin su voto (`votos.<id>` ausente), la que cierra antes primero; `grupo.nombre` es el del grupo o el del plan; un plan borrado no sale.
- **Contrato:** las tres rutas piden cuenta activa (403) y `Cache-Control: no-store`; además de lo de la tabla, `GET /planes/amigos` da 422 por cursor y las tres 503 sin dependencias.
- **Índice nuevo:** `ix_amigos_feed` `{dueno_id, visibilidad, estado, fecha}` en `planes`. Hay que crearlo en Atlas (`go run ./scripts/indexes`) antes del despliegue: `/ready` da 503 si falta.
- **Respuesta a historia:** sin cambio en el backend; espera a nv-87.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-22): ejecutada; ver `checklists/gates.md`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) historias = momentos de las últimas 24 h, sin colección propia — tomada así; (2) ventana de planes: hoy a 30 días o creados en 7 días — tomada así; (3) sin «visto» — tomada así.
- **Detalle:** `specs/nv-84/checklists/gates.md` al ejecutar.
