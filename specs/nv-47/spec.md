# NV-47 — Subida de acciones hechas sin conexión

## Contexto
La app envía en lote lo hecho sin internet, sin duplicar ni perder datos. **Origen**: TEC-33 y hoja A6 del Excel. **Depende de**: NV-46.

## Requisitos
1. `POST /sync/pendientes` recibe hasta 100 acciones hechas sin conexión, cada una con un `id_cliente` único.
2. **Idempotente:** repetir el mismo lote no duplica nada; cada acción devuelve su resultado (`aplicada`, `duplicada`, `conflicto`, `rechazada`).
3. **Conflictos:** si la `version` que trae la acción no coincide con el servidor, no se sobrescribe; se devuelve la copia del servidor y la app decide.

## Diseño
- **Acciones:** `plan.crear`, `plan.editar`, `plan.borrar`, `parada.agregar`, `parada.editar` y `parada.quitar` (con `version` = If-Match del plan y `datos` con el cuerpo de la ruta equivalente de nv-30), `preferencias.editar` y `privacidad.editar` (`version` del perfil opcional). Se aplican con los mismos `planes.Service` y `account.Service` de la API: mismas reglas, mismos 404/422 y cada cambio numerado para la descarga (nv-46).
- **Idempotencia:** `sync_acciones{usuario_id, id_cliente, resultado, plan_id?, creada_en, expira_en}` con `ux_usuario_cliente` (único) y TTL de 7 días (`ttl_accion`). Cada acción se **reclama** insertando su registro antes de aplicarla: si ya existía, responde `duplicada` con `resultado_original` y `plan_id`, aunque los dos lotes lleguen a la vez.
- **Resultados:** en el orden del lote. `conflicto` (409 de la API) trae la copia del servidor (`plan` o `ajustes`); `rechazada` trae `error{codigo, mensaje, campos}` (404, 422, 400). Una acción rechazada no detiene las demás.
- **Fallo de la base a la mitad:** lo ya aplicado queda registrado; la acción que falló se suelta (`Release`) y la respuesta es 503. Reintentar el mismo lote es seguro.
- **Límite y Redis:** 30 lotes por minuto por usuario (`rate:sync_pendientes:{usuario}`); un lote cuesta los mismos 3 comandos del limitador, traiga 1 o 100 acciones.
- **Corrección de drift (2026-11-12):** un lote de 101 es 422 (validación), no 413: la constitución §4 no tiene 413 salvo el límite de cuerpo de 10 MB. Los conflictos de ajustes se comparan con `usuarios.version` porque `PATCH /preferencias` y `/privacidad` no llevan If-Match.

## Contratos
Fila completa en `docs/endpoints.md`.

| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/sync/pendientes` | JWT | 200, 400, 401, 403, 422, 429, 503 |

## Tests
- Integration `tests/integration/pendientes_test.go`: lote nuevo (crear, editar, privacidad), el mismo lote otra vez (`duplicada`, sin otro plan), conflicto de versión con la copia del servidor sin sobrescribir; plan de otra persona y datos inválidos como `rechazada` sin detener el lote; 101 acciones y `plan_id` faltante = 422.
- `Concurrencia`: dos lotes simultáneos del mismo usuario dejan `sync_seq` seguidos y sin repetir.
- Database: índices de `sync_acciones`.

## Criterios
- Repetir un lote no cambia nada.
- Un conflicto nunca pierde datos.

## Límites
- Sin sincronización de favoritos ni de reseñas (no están en este plan).
- Sin resolución automática de conflictos: la decide la app.
- El conflicto de ajustes compara la versión antes de aplicar (no en la misma operación): dos dispositivos que editan a la vez pueden pisarse, como ya pasa en `PATCH /preferencias`.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-12) con la corrección de drift; en ejecución en la rama `nv-47`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** reclamar antes de aplicar; 503 sueltando la acción que falló; 101 = 422; 30 lotes por minuto.
- **Detalle:** la evidencia irá en `specs/nv-47/checklists/gates.md` + `Memoria.md`.
