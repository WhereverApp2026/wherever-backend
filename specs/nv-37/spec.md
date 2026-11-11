# NV-37 — Grupos e integrantes

## Contexto
Un grupo reúne hasta 12 personas alrededor de un plan. **Origen**: US-098, US-099, US-237 y hojas J3 y A4 del Excel. **Depende de**: NV-29, NV-36.

## Requisitos
1. `POST /grupos` crea un grupo sobre un plan propio (hasta 12 integrantes, el organizador cuenta).
2. `POST /planes/{id}/invitaciones` y `POST /grupos/{id}/integrantes` invitan **solo a amigos aceptados**; cada invitación genera una notificación.
3. El integrante confirma o declina (`PATCH /grupos/{id}/integrantes/yo`); puede salir, y el organizador puede sacar a alguien.
4. El organizador ve quién aceptó y quién no.
5. El plan del grupo lo ven y lo leen los integrantes; solo el organizador lo edita.

## Diseño
- **Colección `grupos`:** `plan_id`, `organizador_id`, `integrantes[{usuario_id, estado: invitado|confirmado|declinado, desde}]`, `codigo` (hash del código), `codigo_vence`, `estado`.
- **Tope de 12** aplicado en la misma operación atómica de Mongo (`findOneAndUpdate` con condición sobre el tamaño), no leyendo y escribiendo después.
- **Roles:** organizador, integrante, ajeno (404).
- **Notificaciones:** `invitacion_plan`, `votacion_abierta` y `votacion_cerrada` (se entregan con nv-41).

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/grupos` | JWT | 201, 404, 422 |
| GET | `/api/v1/grupos`, `/api/v1/grupos/{id}` | JWT integrante | 200, 404 |
| POST | `/api/v1/grupos/{id}/integrantes` | JWT organizador | 200, 409, 422 |
| POST | `/api/v1/planes/{id}/invitaciones` | JWT organizador | 200, 409, 422 |
| PATCH / DELETE | `/api/v1/grupos/{id}/integrantes/yo`, `/{usuario_id}` | JWT | 200, 204, 403, 404 |

## Tests
- `Integration/Grupos 12` con MongoDB real: crear, invitar amigo, invitar a quien no es amigo (422), unirse por enlace, enlace vencido (410), enlace revocado, salir, sacar.
- `Concurrencia 4`: 20 personas intentan unirse a un grupo de 11; entran exactamente las que caben.

## Criterios
- Nunca hay más de 12 integrantes.

## Límites
- Sin chat, WebSocket, punto de encuentro, modo sorpresa ni Vaca.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-08), con hallazgos en `Memoria.md`; ola 5: grupos y notificaciones, después de la ruta prioritaria (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-37/checklists/gates.md` + `Memoria.md`.
