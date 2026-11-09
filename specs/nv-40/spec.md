# NV-40 — Dispositivos y centro de notificaciones

## Contexto
Cada persona registra sus teléfonos y consulta sus avisos dentro de la app. **Origen**: US-162 y hoja A6 del Excel. **Depende de**: NV-05, NV-36, NV-37.

## Requisitos
1. Registrar y quitar el token del dispositivo: `PUT/DELETE /dispositivos/{fcm_token}` (máximo 5 por usuario).
2. Centro de notificaciones: `GET /notificaciones` (cursor), `GET /notificaciones/no-leidas`, `PATCH /notificaciones/{id}`, `POST /notificaciones/leer-todas`, `DELETE /notificaciones/{id}`.
3. Tipos: `solicitud_amistad`, `amistad_aceptada`, `invitacion_plan`, `votacion_abierta`, `votacion_cerrada`.

## Diseño
- **Colecciones:** `notificaciones` (destinatario, tipo, datos mínimos, leída, creada) con TTL de 90 días; `dispositivos` (usuario, token con hash, plataforma, último uso).

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| PUT / DELETE | `/api/v1/dispositivos/{fcm_token}` | JWT | 204, 422 |
| GET | `/api/v1/notificaciones` | JWT | 200 |
| GET | `/api/v1/notificaciones/no-leidas` | JWT | 200 |
| PATCH / DELETE | `/api/v1/notificaciones/{id}` | JWT destinatario | 200, 204, 404 |
| POST | `/api/v1/notificaciones/leer-todas` | JWT | 204 |

## Tests
- `Integration/Centro 10` con MongoDB real: crear, listar con cursor, no leídas, marcar, borrar, la de otro (404).

## Criterios
- El centro y el push cuentan lo mismo.

## Límites
- Sin límites propios más allá de los ya anotados.

## Aprobación y Control de Cambios
- **Estado:** 🕓 Pospuesta: fuera de la ruta prioritaria hasta la evaluación del 2 de diciembre de 2026 (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-40/checklists/gates.md` + `Memoria.md`.
