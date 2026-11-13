# NV-40 — Dispositivos y centro de notificaciones

## Contexto
Cada persona registra sus teléfonos y consulta sus avisos dentro de la app. **Origen**: US-162 y hoja A6 del Excel. **Depende de**: NV-05, NV-36, NV-37 (y NV-39 para los avisos de votación).

## Requisitos
1. Registrar y quitar el token de FCM del teléfono: `PUT/DELETE /dispositivos` (máximo 5 por usuario).
2. Centro de notificaciones: `GET /notificaciones` (cursor), `GET /notificaciones/no-leidas`, `PATCH /notificaciones/{id}`, `POST /notificaciones/leer-todas`, `DELETE /notificaciones/{id}`.
3. Tipos: `solicitud_amistad`, `amistad_aceptada`, `invitacion_plan`, `votacion_abierta`, `votacion_cerrada`, emitidos por amistades (nv-36), grupos (nv-37) y votaciones (nv-39).

## Diseño
- **Paquete** `internal/notificaciones`: modelo, repositorio, service (también es el `Notifier`), handler, DTO y ARCO. Los módulos reciben `notificaciones.Notifier` con `WithNotifier`; por defecto `Noop`.
- **Colección `notificaciones`:** `{usuario_id, tipo, datos{ids}, leida_en?, creada_en, expira_en}`; `ix_usuario_creada` y `ttl_notificacion` (90 días sobre `expira_en`). `datos` solo lleva ids (persona, grupo, plan, votación): la app arma el texto, y nada sensible queda guardado.
- **Colección `dispositivos`:** `{usuario_id, token, token_hash, plataforma, ultimo_uso, creado_en}`; `ux_token_hash` y `ix_usuario_uso`. El token se guarda porque nv-41 lo necesita para enviar; se busca y se compara por su SHA-256. Un token que ya era de otra cuenta pasa a la nueva (cambio de sesión en el mismo teléfono). Con 5 teléfonos, el sexto es 422.
- **El token va en el cuerpo, nunca en la ruta:** la ruta queda en el log de acceso. Por eso `PUT/DELETE /dispositivos` con `{token}` en lugar de `/dispositivos/{fcm_token}`.
- **Avisar no rompe nada:** `Emit` guarda el aviso y, si falla, registra solo el tipo y sigue; la acción que lo originó responde igual. En votaciones, el aviso de cierre sale solo de la instancia que la cerró.
- **ARCO:** exportar = avisos y teléfonos (plataforma y fechas, nunca el token); «Borrar mi historial» borra los avisos; borrar la cuenta borra avisos y teléfonos.
- **Corrección de drift (2026-11-12):** el token salía en la ruta; ahora va en el cuerpo. Las preferencias por tipo y el horario silencioso aplican al push (nv-41), no al centro: el centro guarda todo.

## Contratos
Filas completas en `docs/endpoints.md`.

| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| PUT | `/api/v1/dispositivos` con `{token, plataforma?}` | JWT | 204, 400, 401, 403, 422 |
| DELETE | `/api/v1/dispositivos` con `{token}` | JWT | 204, 400, 401, 403, 404, 422 |
| GET | `/api/v1/notificaciones` | JWT | 200, 401, 422 |
| GET | `/api/v1/notificaciones/no-leidas` | JWT | 200, 401 |
| POST | `/api/v1/notificaciones/leer-todas` | JWT | 204, 401, 403 |
| PATCH | `/api/v1/notificaciones/{id}` | JWT destinatario | 200, 400, 401, 403, 404, 422 |
| DELETE | `/api/v1/notificaciones/{id}` | JWT destinatario | 204, 401, 403, 404 |

## Tests
- Unit `internal/notificaciones/dto_test.go`: validación, `leida` y `datos`, hash del token.
- Integration `tests/integration/notificaciones_test.go`: listar con cursor, no leídas, marcar y desmarcar, leer todas, borrar, el aviso de otra persona = 404; los avisos de invitación, votación abierta y cerrada, solicitud y aceptación de amistad; 5 teléfonos y el sexto = 422, re-registrar, el token pasa de cuenta, quitar el de otra = 404; exportar sin token y borrar.
- Security `tests/security/notificaciones_security_test.go`: las siete rutas piden sesión; el token no llega a los logs.
- Database: los cuatro índices. Contract: la exportación incluye `notificaciones`.

## Criterios
- El centro y el push cuentan lo mismo (nv-41 envía a partir del mismo `Emit`).
- Avisar nunca cambia la respuesta de la acción que lo originó.

## Límites
- Sin límites propios más allá del global; sin comandos de Redis.
- Sin push (nv-41) ni avisos programados.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-12) con la corrección de drift; en ejecución en la rama `nv-40`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** token en el cuerpo; token guardado (hace falta para enviar) con búsqueda por hash; el sexto teléfono es 422 (no reemplaza al más viejo); los avisos viven 90 días.
- **Detalle:** la evidencia irá en `specs/nv-40/checklists/gates.md` + `Memoria.md`.
