# NV-36 — Solicitudes y lista de amigos

## Contexto
Enviar, aceptar, rechazar, eliminar y bloquear amistades, con la lista ordenada por actividad. **Origen**: US-232, US-233, US-236 y hoja J2 del Excel. **Depende de**: NV-35.

## Requisitos
1. `POST /amistades` envía solicitud: no a uno mismo, no repetida, no a quien me bloqueó; si la otra persona ya me había enviado una, quedan como amigos.
2. `GET /amistades?estado=` lista amigos (por actividad reciente, con búsqueda por nombre), solicitudes recibidas y enviadas, con el contador de pendientes.
3. `PATCH /amistades/{id}` acepta o rechaza; rechazar no avisa a quien envió.
4. `DELETE /amistades/{id}` elimina al amigo o cancela una solicitud; `POST /amistades/{id}/bloquear` bloquea.

## Diseño
- **Colección `amistades`:** `de`, `para`, `estado` (`pendiente`, `aceptada`, `bloqueada`), `par` (los dos ids ordenados, índice único), `creada_en`, `actualizada_en`.
- **Bloqueo:** quien bloquea desaparece de las búsquedas y solicitudes de la otra persona; la relación queda como `bloqueada` sin avisar.
- **Notificaciones:** los eventos `solicitud_amistad` y `amistad_aceptada` quedan definidos, pero no se guardan ni se envían mientras nv-40 y nv-41 estén pospuestas. La app ve las solicitudes con el contador de pendientes.
- **Eliminar cuenta:** el `Purger` de `nv-05` borra las amistades del usuario.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/amistades` | JWT | 201, 404, 409, 422, 429 |
| GET | `/api/v1/amistades?estado=` | JWT | 200 |
| PATCH | `/api/v1/amistades/{id}` | JWT receptor | 200, 404, 409 |
| DELETE | `/api/v1/amistades/{id}` | JWT participante | 204, 404 |
| POST | `/api/v1/amistades/{id}/bloquear` | JWT participante | 204, 404 |

## Tests
- `Integration/Amistades 12` con MongoDB real: solicitud, solicitud cruzada, aceptar, rechazar sin aviso, repetida, a sí mismo, bloquear, eliminar, listas por estado y orden.
- `Concurrencia 3`: dos solicitudes cruzadas simultáneas dejan un solo documento.

## Criterios
- La lista de amigos y las solicitudes coinciden entre ambas personas.

## Límites
- Sin código QR de amistad.
- El bloqueo no oculta planes ya compartidos.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 3: amistades, ML y grafo
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-36/checklists/gates.md` + `Memoria.md`.
