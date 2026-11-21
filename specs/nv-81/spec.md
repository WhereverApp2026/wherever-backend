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

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-21), sin ejecutar.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) nivel e insignias destacadas visibles para quien ve el perfil, no solo amigos; (2) contadores en caché de 5 min; (3) «planeando ahora» = plan para hoy o votación abierta.
- **Detalle:** `specs/nv-81/checklists/gates.md` al ejecutar.
