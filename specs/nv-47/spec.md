# NV-47 — Subida de acciones hechas sin conexión

## Contexto
La app envía en lote lo hecho sin internet, sin duplicar ni perder datos. **Origen**: TEC-33 y hoja A6 del Excel. **Depende de**: NV-46.

## Requisitos
1. `POST /sync/pendientes` recibe hasta 100 acciones hechas sin conexión, cada una con un `id_cliente` único.
2. **Idempotente:** repetir el mismo lote no duplica nada; cada acción devuelve su resultado (`aplicada`, `duplicada`, `conflicto`, `rechazada`).
3. **Conflictos:** si la `version` que trae la acción no coincide con el servidor, no se sobrescribe; se devuelve la copia del servidor y la app decide.

## Diseño
- **Acciones soportadas ahora:** crear, editar y borrar planes y paradas, y ajustes del perfil.
- **Registro de acciones aplicadas:** `sync_acciones` (usuario, `id_cliente`, resultado) con TTL de 7 días, para la idempotencia.
- **Presupuesto:** un lote cuesta un número fijo de comandos de Redis (el límite), no uno por acción.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/sync/pendientes` | JWT | 200, 413, 422, 429 |

## Tests
- `Integration/Pendientes 12`: lote nuevo, el mismo lote otra vez, conflicto de versión, acción de otra persona, lote de 101, lote parcial con una inválida.
- `Concurrencia 3`: dos lotes simultáneos del mismo usuario mantienen `sync_seq` sin huecos ni repetidos.

## Criterios
- Repetir un lote no cambia nada.
- Un conflicto nunca pierde datos.

## Límites
- Sin sincronización de favoritos ni de reseñas (no están en este plan).
- Sin resolución automática de conflictos: la decide la app.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-08), con hallazgos en `Memoria.md`; ola 7: sin conexión y cifrado, después de la ruta prioritaria (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-47/checklists/gates.md` + `Memoria.md`.
