# NV-46 — Descarga de cambios

## Contexto
La app baja solo lo que cambió desde su última sincronización, incluidos los borrados. **Origen**: TEC-33 y hoja A6 del Excel. **Depende de**: NV-29, NV-18.

## Requisitos
1. `GET /sync/cambios?desde=` devuelve, paginado, lo que cambió para esta persona desde un punto (planes y ajustes), incluidos los borrados.
2. Un reloj atrasado en el teléfono no rompe la sincronización.
3. Solo se sincronizan datos de la propia persona.

## Diseño
- **Cursor de sincronización** con un contador por usuario (`sync_seq`, incrementado con `findOneAndUpdate`), no con la hora del teléfono ni del servidor, para evitar saltos por relojes.
- **Tombstones:** colección `borrados` (tipo, id, `sync_seq`) con TTL de 30 días; quien lleva más de 30 días sin conectarse hace una descarga completa (`410` con `requiere_completa: true`).

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/sync/cambios?desde=` | JWT | 200, 410, 422 |

## Tests
- `Integration/Cambios 8` con MongoDB real: cambios nuevos, borrados, orden, cursor, descarga completa por tombstone vencido.
- `Security/Sync 4`: nunca devuelve datos de otra persona.

## Criterios
- El reloj del teléfono no influye.

## Límites
- Descarga completa tras 30 días sin conexión.

## Aprobación y Control de Cambios
- **Estado:** 🕓 Pospuesta: fuera de la ruta prioritaria hasta la evaluación del 2 de diciembre de 2026 (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-46/checklists/gates.md` + `Memoria.md`.
