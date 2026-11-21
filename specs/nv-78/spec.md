# NV-78 — Insignias de las personas

## Contexto
La maqueta pide insignias en el perfil (`81_Perfil`) y una pantalla de «Insignias ganadas y bloqueadas» (`83_Insignias`, `/me/badges`). La spec nv-70 dejó las insignias fuera hasta tener su módulo (hoja J8, gamificación). Hoy existen `puntos`, `nivel` y `racha_semanas` en el perfil, pero nada los mueve tras el registro. **Origen**: inventario de Figma (§4.10) y pedido del revisor (2026-11-21). **Depende de**: NV-05 (perfil y ARCO), NV-30 (planes), NV-36 (amistades), NV-62 (check-ins y reseñas), NV-63 (postales), NV-70 (perfil de persona y sus reglas de visibilidad).

## Requisitos
1. La persona ve todas las insignias: las ganadas con su fecha y las bloqueadas con su avance.
2. Otra persona que puede ver su perfil ve solo las ganadas, sin avance.
3. Las insignias salen de lo que la persona ya hace; no hay que tocar cada módulo para darlas.

## Diseño
- **Catálogo fijo** en `internal/insignias/catalogo.go` (id, nombre, descripción, fuente, meta), en este orden: `primer_plan` (1 plan), `planificador` (10 planes), `primera_resena` (1 reseña), `critico` (10 reseñas), `explorador` (check-in en 5 lugares distintos), `trotamundos` (20 lugares), `buena_compania` (5 amigos), `fotografo` (5 postales).
- **Fuentes (conteos con índices existentes):** planes `activo` del dueño (`ix_dueno_estado_fecha`); reseñas `publicada` (`ix_mis_resenas`); negocios distintos en check-ins `valido` (`ix_mis_checkins`); amistades `aceptada` como `de` o `para` (`ix_de_estado`, `ix_para_estado`); postales `lista`.
- **Al pedirlas, sin eventos:** se cuentan las fuentes; cada insignia alcanzada que aún no tiene fecha se guarda en `insignias_usuarios` (`{usuario_id, insignia, ganada_en}`, índice único `ux_insignia_usuario`; una inserción repetida por concurrencia no es error y gana la primera fecha). Una insignia ganada se conserva aunque el conteo baje (borró un plan).
- **Rutas:** `GET /usuarios/yo/insignias` (JWT, cuenta activa) → `{insignias[{id, nombre, descripcion, ganada, ganada_en, progreso{actual, meta}}], ganadas, total}`; `GET /personas/{id}/insignias` (JWT, cuenta activa, límite del perfil 120 por hora) → `{insignias[{id, nombre, ganada_en}]}` con las reglas de visibilidad de nv-70 (inexistente, inactiva, propia o con bloqueo = el mismo 404).
- **Ciclo de imports:** `personas` usa `insignias`, y `postales` y `amistades` dependen de `personas`; por eso `insignias` no importa módulos de negocio: lleva sus nombres de colección como constantes y `TestInsignias_ColeccionesIgualesALosModulos` comprueba que coincidan.
- **ARCO:** `insignias.Privacy` exporta las ganadas (`insignias_usuarios` en la exportación) y las borra con la cuenta; «Borrar mi historial» no las toca.
- **Fuera de esta spec:** sumar puntos o subir de nivel; insignias por eventos en vivo o push al ganarlas.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/usuarios/yo/insignias` | JWT, cuenta activa | 200, 401, 403, 503 |
| GET | `/api/v1/personas/{id}/insignias` | JWT, cuenta activa | 200, 401, 403, 404, 422, 429, 503 |

## Tests
- Unit `internal/insignias/catalogo_test.go`: ids únicos, metas positivas, fuentes conocidas; `Ganada` y `Progreso`.
- `tests/integration/insignias_test.go`: constantes iguales a las de los módulos; ganadas y bloqueadas con su avance (plan borrado y check-in no válido no cuentan; dos visitas al mismo lugar son un lugar); fecha estable aunque baje el conteo; otra persona ve solo las ganadas sin avance; ARCO exporta y borra; 4 peticiones a la vez guardan una sola fecha.
- `tests/security/insignias_security_test.go`: 401 ×4 en las dos rutas; propia e inexistente 404; id inválido 422; bloqueo en ambos sentidos 404; cuenta en eliminación 403.
- `tests/database/indexes_test.go`: `ux_insignia_usuario` único. Contract: `insignias/mias_200`, `insignias/persona_200` y la exportación con `insignias_usuarios`.

## Criterios
- Ninguna persona ve el avance ni las insignias bloqueadas de otra; el bloqueo las oculta.
- Una insignia ganada no se pierde ni cambia de fecha.

## Límites
- Catálogo de 8 insignias; cambiarlo es cambiar el código (sin panel de administración).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-21); implementada en la rama `feat/nv-78-insignias-usuarios`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor (tomadas como las propone la spec):** catálogo de 8 sobre datos existentes; cálculo al pedirlas con fecha guardada; otra persona ve solo las ganadas; sin puntos ni niveles.
- **Detalle:** evidencia en `specs/nv-78/checklists/gates.md` + `Memoria.md`.
