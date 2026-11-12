# NV-36 — Solicitudes y lista de amigos

## Contexto
Enviar, aceptar, rechazar, eliminar y bloquear amistades, con la lista ordenada por actividad. **Origen**: US-232, US-233, US-236 y hoja J2 del Excel. **Depende de**: NV-35.

## Requisitos
1. `POST /amistades` envía solicitud: no a uno mismo, no repetida, no a quien me bloqueó; si la otra persona ya me había enviado una, quedan como amigos.
2. `GET /amistades?estado=` lista amigos (por actividad reciente, con búsqueda por nombre), solicitudes recibidas y enviadas, con el contador de pendientes.
3. `PATCH /amistades/{id}` acepta o rechaza; rechazar no avisa a quien envió.
4. `DELETE /amistades/{id}` elimina al amigo o cancela una solicitud; `POST /amistades/{id}/bloquear` bloquea.
5. Límite por usuario: 20 solicitudes por hora en `POST /amistades` (viene de nv-35; `Limiter.WindowUser`, llave `rate:amistades_solicitud:{usuario}`).

## Diseño
- **Paquete `internal/amistades`:** repositorio, service, handler, DTO y `Privacy` (Purger y Exporter de nv-05).
- **Colección `amistades`:** `de`, `para`, `estado` (`pendiente`, `aceptada`, `bloqueada`), `par` (los dos ids ordenados `a:b`, índice único `ux_par`), `creada_en`, `actualizada_en`. Índices `ix_de_estado` e `ix_para_estado` (persona, estado, `actualizada_en` descendente).
- **Enviar:** a uno mismo o con `usuario_id` inválido = 422; a una cuenta que no existe, no está activa o me bloqueó = 404 igual; pendiente mía o ya amigos = 409. Si la otra persona ya me había enviado una, se acepta. Si dos solicitudes cruzadas chocan en `ux_par`, la perdedora relee el par y acepta la de la otra.
- **Responder:** solo quien recibió; para quien envió o un tercero es 404 (no se distingue de una solicitud que no existe). Ya respondida = 409. Rechazar borra el documento y responde `estado: rechazada`; quien envió solo deja de verla y puede volver a enviar.
- **Lista:** por `actualizada_en` (la última vez que cambió la relación) de la más reciente a la más antigua, hasta 500; `q` filtra por nombre o nombre de usuario sin acentos ni mayúsculas; las cuentas no activas no aparecen. `pendientes` = recibidas sin responder, en cualquier lista.
- **Bloqueo:** sobre una relación existente (`{id}` de la amistad o solicitud); queda `bloqueada` con `de` = quien bloqueó, sin avisar. Las dos personas dejan de verse en `GET /personas/buscar` (`personas.Blocks`, que cumple el repositorio de amistades; la exclusión va en la consulta con `$nin`). Quien fue bloqueado recibe 404 al enviar, responder, eliminar o bloquear; quien bloqueó recibe 409 al enviar. No hay desbloqueo en esta spec.
- **Notificaciones:** los eventos `solicitud_amistad` y `amistad_aceptada` no se guardan ni se envían mientras nv-40 y nv-41 estén pospuestas. La app ve las solicitudes con el contador de pendientes.
- **ARCO:** el `Purger` de `nv-05` borra las amistades del usuario al eliminar la cuenta; "borrar historial" no las toca. La exportación incluye `amistades` (`persona_id`, `estado`, `direccion`, fechas), sin los bloqueos que hizo la otra persona.
- **Sin `If-Match`:** cada cambio es una transición de estado con filtro condicional (`estado` esperado), no una edición de campos.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/amistades` | JWT | 201, 400, 401, 403, 404, 409, 422, 429 |
| GET | `/api/v1/amistades?estado=&q=` | JWT | 200, 401, 403, 422 |
| PATCH | `/api/v1/amistades/{id}` | JWT receptor | 200, 400, 401, 403, 404, 409, 422 |
| DELETE | `/api/v1/amistades/{id}` | JWT participante | 204, 401, 403, 404 |
| POST | `/api/v1/amistades/{id}/bloquear` | JWT participante | 204, 401, 403, 404 |

403 = cuenta en eliminación pendiente (`usuarios.RequireActive`).

## Tests
- `tests/integration/amistades_test.go` con MongoDB real: solicitud en ambas listas, solicitud cruzada, aceptar y ya respondida, rechazar sin aviso, repetida, a sí mismo e inexistente, bloquear (oculta en la búsqueda e impide solicitudes), eliminar y cancelar, solicitud ajena (404), listas por estado, orden y búsqueda, borrar cuenta y exportar, límite 20/h (21.ª = 429) y 401.
- `TestAmistades_Concurrencia_SolicitudesCruzadasUnSoloDocumento`: 3 rondas de dos solicitudes cruzadas simultáneas dejan un solo documento `aceptada`.
- `tests/security/amistades_security_test.go`: las cinco rutas, 401 sin token y 403 con la cuenta en eliminación pendiente.
- `tests/database/indexes_test.go`: índices de `amistades`. `tests/contract`: `amistades: []` en la exportación.
- Unit: `internal/amistades/amistades_test.go` (par, dirección, búsqueda por nombre, DTOs, entradas inválidas) e `internal/personas/service_test.go` (excluye bloqueados).

## Criterios
- La lista de amigos y las solicitudes coinciden entre ambas personas.

## Límites
- Sin código QR de amistad.
- Sin desbloqueo.
- El bloqueo no oculta planes ya compartidos.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-11); ruta prioritaria, ola 3: amistades, ML y grafo
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** orden por `actualizada_en` (no hay campo de actividad del usuario); bloqueo en ambos sentidos en la búsqueda; rechazar borra; quien envió recibe 404 al intentar aceptar.
- **Detalle:** evidencia en `specs/nv-36/checklists/gates.md` + `Memoria.md`.
