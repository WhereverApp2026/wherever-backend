# NV-39 — Votaciones

## Contexto
El grupo decide entre 2 o 3 opciones por parada sin chats largos. **Origen**: US-098, US-099, US-237 y hojas J3 y A4 del Excel. **Depende de**: NV-38.

## Requisitos
1. **Votaciones (US-099):** el organizador abre una votación para una parada con 2 o 3 opciones y hora límite; un voto por persona, editable hasta el cierre; gana la opción con más votos; en empate, la propuesta primero.

## Diseño
- **Votaciones:** colección `votaciones` con `parada_id`, `opciones[]` (ids de negocios aprobados), `votos{usuario→opcion}`, `cierra_en`, `estado`.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST / GET | `/api/v1/grupos/{id}/votaciones` | JWT | 201, 200 |
| PUT | `/api/v1/votaciones/{id}/voto` | JWT integrante | 200, 409 |
| POST | `/api/v1/votaciones/{id}/cerrar` | JWT organizador | 200, 409 |

## Tests
- `Integration/Votaciones 10`: voto, cambiar voto, voto de ajeno, cerrar, empate, cerrar antes, voto tras cerrar (409), aplicar ganadora y recalcular el plan.
- `Integration/Trabajo 2`: el trabajo cierra votaciones vencidas una sola vez con dos instancias.

## Criterios
- Cerrar una votación cambia el plan y deja constancia.

## Límites
- Sin límites propios más allá de los ya anotados.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-08), con hallazgos en `Memoria.md`; ola 5: grupos y notificaciones, después de la ruta prioritaria (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-39/checklists/gates.md` + `Memoria.md`.
