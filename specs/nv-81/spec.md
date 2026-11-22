# NV-81 — Perfil con contadores y pestañas

## Contexto
La maqueta muestra en el perfil propio (`81_Perfil`) y en el de otra persona (`26_PerfilAmigo`) **contadores** («14 planes · 31 amigos · 9 reseñas»), el nivel, las insignias y tres **pestañas**: Publicaciones, Planes y Reseñas. En la lista de amigos (`22_AmigosLista`) cada amigo trae «14 planes» o «Planeando ahora». Hoy `GET /personas/{id}` solo da datos básicos y, entre amigos, nivel y comunes; no hay listas por persona. **Origen**: revisión de la maqueta (2026-11-21). **Depende de**: NV-70 (perfil de persona y visibilidad), NV-77 (descripción), NV-78/NV-79 (insignias y nivel), NV-82 (privacidad social), NV-30 (planes), NV-62 (reseñas), NV-63 (momentos).

## Requisitos
1. El perfil propio y el de otra persona traen contadores de planes, amigos y reseñas.
2. Las tres pestañas del perfil de otra persona muestran lo que su privacidad permite.
3. La lista de amigos trae cuántos planes tiene cada uno y si está planeando ahora.

## Diseño
- **Contadores:** `planes` (planes activos propios), `amigos` (amistades aceptadas), `resenas` (publicadas). En `GET /usuarios/yo` y en `GET /personas/{id}`; en el perfil ajeno se muestran según nv-82 (con «actividad: nadie», contadores en `null`).
- **Nivel en el perfil ajeno:** `nivel{numero, nombre}` y las 3 primeras insignias ganadas (`insignias_destacadas`, como «Explorador · Taquero · Reseñista · +3») para quien puede ver el perfil; deja de ser solo de amigos (la maqueta lo muestra a todos). Decisión para el revisor.
- **Pestañas (paginadas con cursor firmado, 20 por página, máx. 50):**
  - `GET /personas/{id}/momentos` — momentos del autor que yo puedo ver (visibilidad del momento y nv-82), del más nuevo al más viejo.
  - `GET /personas/{id}/planes` — planes `publico`, y los de `amigos` si somos amigos y su privacidad lo permite; sin integrantes ni grupo.
  - `GET /personas/{id}/resenas` — reseñas publicadas con el negocio (nombre, slug) y la respuesta del negocio.
  - Mismas reglas de nv-70: propia, inexistente, inactiva o con bloqueo = 404; límite de la política `personas_perfil`.
- **Lista de amigos:** `GET /amistades` agrega `planes` (conteo) y `planeando_ahora` (tiene un plan activo para hoy o un grupo con votación abierta). Un `$lookup` agrupado sobre los hasta 500 amigos, con los índices existentes.
- **Costo:** contadores con `CountDocuments` sobre índices por persona (`ix_dueno_estado_fecha`, `ix_mis_resenas`, `ix_de_estado`/`ix_para_estado`); se cachean 5 min en Redis por persona (`cache:perfil_contadores:{usuario}`) y se invalidan al crear o borrar plan, reseña o amistad.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/personas/{id}/momentos` | JWT, cuenta activa | 200, 401, 403, 404, 422, 429 |
| GET | `/api/v1/personas/{id}/planes` | JWT, cuenta activa | 200, 401, 403, 404, 422, 429 |
| GET | `/api/v1/personas/{id}/resenas` | JWT, cuenta activa | 200, 401, 403, 404, 422, 429 |
| GET | `/api/v1/personas/{id}` | JWT | agrega `contadores`, `nivel`, `insignias_destacadas` |
| GET | `/api/v1/usuarios/yo` | JWT | agrega `contadores` |
| GET | `/api/v1/amistades` | JWT | agrega `planes` y `planeando_ahora` por amigo |

## Tests
- Integration: contadores correctos (planes borrados y reseñas ocultas no cuentan); pestañas con visibilidad de momentos y planes; paginación; invalidación de la caché; lista de amigos con conteos.
- Security: bloqueo, propia, inactiva e inexistente = 404 en las tres pestañas; un plan `privado` o de grupo ajeno nunca aparece; sin integrantes, correo ni ubicación; límite 429.
- Contract: perfil con contadores y las tres pestañas.

## Criterios
- Nadie ve en una pestaña algo que la visibilidad del contenido o la privacidad de su dueño no permitan.
- Los contadores coinciden con lo que la persona tiene publicado.

## Límites
- Sin seguidores (solo amistad), sin perfil público sin sesión.

## Drift al ejecutar (2026-11-21)
- **Reglas de nv-82 aplicadas:** `nadie` = pestañas vacías, contadores en `null`, sin `nivel` e `insignias_destacadas` vacía, aun entre amigos. `amigos` (por omisión) = solo los amigos ven pestañas y contadores; a otra persona le salen vacías y en `null`. `todos` = cualquiera con sesión ve las pestañas. Además cada interruptor cierra lo suyo: `compartir_postales` la de momentos, `compartir_planes` la de planes y su contador, `compartir_resenas` la de reseñas y su contador. Una pestaña cerrada responde 200 con la página vacía (no 404), para no distinguirla de una persona sin contenido.
- **Momentos:** entre amigos salen `publica` y `amigos`; a los demás (con `todos`), solo `publica`. Es la diferencia entre `amigos` y `todos` que nv-82 dejó para aquí.
- **Planes:** `planes` no tiene visibilidad `amigos` (solo `privado` y `publico`, nv-68): la pestaña muestra solo los `publico` activos, con la forma de los populares (sin dueño, integrantes, notas ni grupo), por fecha descendente.
- **Contadores:** en `GET /usuarios/yo`, `planes` cuenta todos los activos propios; en el perfil ajeno y en la lista de amigos, solo los `publico` (lo que muestra la pestaña), para que «coincidan con lo que la persona tiene publicado». Se suma `insignias_total` para el «+3» de la maqueta.
- **Caché:** `cache:perfil_contadores:{usuario}` 5 min (`internal/contadores`). Se invalida al crear, editar (cambia la visibilidad) o borrar un plan, al crear o borrar una reseña y al aceptar, quitar o bloquear una amistad (las dos personas). Lo que cambia por moderación (una reseña que se publica u oculta) se ve al vencer la llave.
- **`planeando_ahora`:** un plan activo para hoy en America/Mexico_City, aunque sea privado (solo dice «planeando ahora», no lo muestra), o un grupo donde está confirmado con una votación abierta. Con `compartir_planes: false` o `nadie`, `planes` y `planeando_ahora` van en `null`; también en las listas de solicitudes.
- **Dónde viven las rutas:** las pestañas están en `postales`, `planes` y `resenas` (personas no puede importarlos: `postales` → `amistades` → `personas` sería ciclo); piden permiso con `personas.Service.Pestana` y cuentan en la política `personas_perfil` (120/h compartida con el perfil y sus insignias). `internal/contadores` lee las colecciones por nombre; `TestNombres_IgualesALosDeCadaModulo` lo vigila.
- **Nivel:** `nivel` pasa de cadena (solo amigos) a `{numero, nombre}` de la tabla de nv-79 para quien ve el perfil; una cuenta sin `nivel_numero` va en el 1. Reseña de un negocio que ya no está visible no sale en la pestaña.
- **Feed de reseñas:** no existe; `compartir_resenas` aplica a la pestaña y al contador. La lista pública de reseñas del negocio no cambia.
- **Sin índices nuevos:** los conteos y pestañas usan `ix_dueno_estado_fecha`, `ix_mis_resenas`, `ix_feed`, `ix_de_estado`, `ix_para_estado` e `ix_estado_cierra`. 260 rutas.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-21): ejecutada; ver `checklists/gates.md`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) nivel e insignias destacadas visibles para quien ve el perfil, no solo amigos — tomada así; (2) contadores en caché de 5 min — tomada así; (3) «planeando ahora» = plan para hoy o votación abierta — tomada así.
- **Detalle:** `specs/nv-81/checklists/gates.md`.
