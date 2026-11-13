# NV-39 — Votaciones

## Contexto
El grupo decide entre 2 o 3 opciones por parada sin chats largos. **Origen**: US-098, US-099, US-237 y hojas J3 y A4 del Excel. **Depende de**: NV-37 (grupos) y NV-30 (editar el plan).

## Requisitos
1. **Votaciones (US-099):** el organizador abre una votación para una parada con 2 o 3 opciones y hora límite; un voto por persona, editable hasta el cierre; gana la opción con más votos; en empate, la propuesta primero.
2. Al cerrar (a mano o al vencer) la ganadora reemplaza el negocio de la parada y el plan se recalcula; queda constancia de la ganadora, quién cerró y si se aplicó.

## Diseño
- **Paquete** `internal/votaciones` (modelo, repositorio, service, handler, ARCO y `Closer`). Usa `grupos.Repository`, `planes.Repository` (plan activo), `planes.Service.ReplaceBusiness` (aplica la ganadora con `Recalcular` y `If-Match` interno, un reintento) y `negocios.Repository.FindCards` (opciones).
- **Colección `votaciones`:** `{grupo_id, plan_id, parada_id, opciones[ids de negocios visibles], votos{usuario_hex: negocio_id}, votos_version, cierra_en, estado (abierta|cerrada), ganadora?, cerrada_por (organizador|vencimiento)?, cerrada_en?, aplicada?, motivo?, creada_por, creada_en, actualizada_en}`. Índices: `ux_parada_abierta` (una abierta por parada, parcial), `ix_grupo_creada`, `ix_estado_cierra`.
- **Votar:** `$set votos.{usuario}` + `$inc votos_version`, solo si sigue abierta y `cierra_en` no pasó. Votan invitados y confirmados; quien declinó no (403).
- **Cerrar sin perder votos:** se cuentan los votos leídos y se cierra con un `UpdateOne` condicionado a `estado = abierta` y a la misma `votos_version`. Si alguien votó en medio u otra instancia cerró antes, se relee. Así cada votación se cierra una sola vez aunque corran dos instancias.
- **Aplicar:** después de cerrar se cambia la parada del plan del organizador. Si el plan o la parada ya no existen, o la ganadora no abre a esa hora, queda `aplicada: false` con `motivo`. Si falla la base, `aplicada` queda sin valor y el trabajador lo reintenta.
- **Vencimiento sin Redis:** cada instancia corre `Closer` cada minuto (`startVotes` en `cmd/api`), que busca en MongoDB (`ix_estado_cierra`) las vencidas y las cerradas sin aplicar. No hay candado `lock:` porque cerrar ya es condicionado: **0 comandos de Redis**, contra el ~7 % de la cuota de Upstash que estimaba la revisión (hallazgo de `Memoria.md`). Además, listar o votar en una vencida la cierra en ese momento.
- **Privacidad:** la respuesta da conteos y `mi_voto`, nunca quién votó qué. ARCO: exportar = los votos propios; borrar la cuenta = borrar las votaciones que abrió y quitar sus votos de las demás.
- **Corrección de drift (2026-11-12):** la spec decía «Depende de NV-38», pero las votaciones no usan el enlace. El cierre nocturno de nv-37 pasó a un trabajador por minuto sin Redis. Las notificaciones `votacion_abierta` y `votacion_cerrada` son de nv-40/41.

## Contratos
Filas completas en `docs/endpoints.md`.

| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/grupos/{id}/votaciones` | JWT, organizador | 201, 400, 401, 403, 404, 409, 422 |
| GET | `/api/v1/grupos/{id}/votaciones` | JWT, integrante | 200, 401, 404 |
| PUT | `/api/v1/votaciones/{id}/voto` | JWT, invitado o confirmado | 200, 400, 401, 403, 404, 409, 422 |
| POST | `/api/v1/votaciones/{id}/cerrar` | JWT, organizador | 200, 401, 403, 404, 409 |

## Tests
- Unit `internal/votaciones/model_test.go`: ganadora (más votos, empates, sin votos, voto inválido), validación de cuerpos, la vista no dice quién votó qué.
- Integration `tests/integration/votaciones_test.go`: votar, cambiar el voto, cerrar y aplicar al plan; empate; permisos (integrante no abre ni cierra, ajeno 404, declinado no vota); opción ajena, parada fuera del plan, lugar inexistente y hora límite fuera de rango = 422; segunda votación abierta en la parada = 409; votar o cerrar ya cerrada = 409; ganadora que no cabe queda sin aplicar.
- `Integration/Trabajo`: dos `FinishDue` simultáneos cierran y aplican una sola vez (el plan sube una versión).
- Security `tests/security/votaciones_security_test.go`: sesión, cuenta activa y votación ajena = 404.
- Database: los tres índices. Contract: la exportación incluye `votaciones`.

## Criterios
- Cerrar una votación cambia el plan y deja constancia.
- Cada votación se cierra una sola vez, aunque corran dos instancias.

## Límites
- Sin límites propios más allá del global; sin comandos de Redis.
- Hasta 100 votaciones por grupo en la lista.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-12) con la corrección de drift; en ejecución en la rama `nv-39`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** votan invitados y confirmados; hora límite de 5 minutos a 7 días; sin votos gana la primera; cierre por vencimiento sin Redis; una ganadora que no cabe deja el plan igual con motivo.
- **Detalle:** la evidencia irá en `specs/nv-39/checklists/gates.md` + `Memoria.md`.
