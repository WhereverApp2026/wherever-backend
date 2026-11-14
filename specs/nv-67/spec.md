# NV-67 — Reportes de información incorrecta de un negocio

## Contexto
Desde la ficha, una persona avisa que el horario, el precio o la ubicación están mal, o que el lugar cerró (US-073). El administrador revisa la cola y marca cada reporte como corregido o descartado. **Origen**: hoja «A2 Lugares y búsqueda» del Excel `Wherever_Endpoints_CRUD_v3.xlsx` (colección «reportes_negocio» y dos filas de endpoints). **Depende de**: NV-04 (`RequireRoles`, rol `administrador`), NV-05 (`RequireActive`, ARCO con `privacidad.Purger`/`Exporter`), NV-17 (cursor firmado), NV-20/NV-21 (`negocios.Repository.FindVisible`), NV-42 (moderación de texto) y NV-43/NV-55 (política `admin`).

## Requisitos
1. `POST /lugares/{id}/reportes` guarda un reporte con `motivo` y `detalle` opcional; responde 201.
2. Una persona tiene a lo más un reporte abierto por negocio y 10 reportes por día.
3. `GET /admin/reportes-negocio` lista la cola del administrador (agregada, ver drift); `PATCH /admin/reportes-negocio/{id}` lo marca `corregido` o `descartado`.
4. Los reportes de una persona entran en su exportación y en el borrado de su cuenta.

## Diseño
- **Paquete `internal/reportesnegocio`:** modelo, repositorio, service, handler, DTO y `privacy.go`; usa `negocios.Repository.FindVisible`. El armado para pruebas va en `internal/testutil/reportesnegociotest`.
- **Colección `reportes_negocio`** (Excel A2 §4): `negocio_id`, `usuario_id`, `motivo` (`horario`, `precio`, `cerrado_permanente`, `ubicacion`, `otro`), `detalle` (≤ 300, opcional; obligatorio con `otro`), `estado` (`abierto`, `corregido`, `descartado`), `resuelto_por`, `resuelto_en`, `version`, `creado_en`, `actualizado_en`.
- **Índices** (`scripts/indexes` y `/ready`): `ix_reportes_negocio` `{negocio_id: 1, estado: 1}`, `ix_reportes_cola` `{estado: 1, creado_en: 1}`, y dos nuevos: `ux_reporte_abierto` `{negocio_id: 1, usuario_id: 1}` único parcial con `estado = abierto` (un abierto por persona y negocio) e `ix_reportes_usuario` `{usuario_id: 1, creado_en: -1}` (límite diario y ARCO).
- **Crear:** JWT de cualquier rol más `RequireActive` (el Excel dice «Consumidor»; el dueño no reporta su propio negocio: 403). Negocio no visible = 404. Campo de más = 400; `motivo` desconocido, `detalle` largo o `otro` sin `detalle` = 422. `detalle` pasa por la moderación de texto de nv-42 (rechazo = 422 genérico) y se guarda sin cambios. Segundo abierto en el mismo negocio = 409 (por `ux_reporte_abierto`). 201 con `id` y `estado`.
- **Límite contra abuso, sin Redis:** 10 reportes por día por usuario, contados en MongoDB con `ix_reportes_usuario` antes de insertar (como el chat de nv-57); al pasarlo, 429 uniforme con `Retry-After`. Cuenta los guardados, no los rechazados. No se agrega política de Redis: reportar es raro, pero la cuenta en MongoDB es exacta por usuario y no gasta cuota de Upstash. Dos envíos en carrera pueden pasar el 11.º: es freno de abuso, no cupo exacto.
- **Cola del administrador:** `estado` (`abierto` por omisión), `negocio_id` opcional, del más viejo al más nuevo (`ix_reportes_cola`), cursor firmado de nv-17 (`Signer.Scope("admin_reportes_negocio", estado, negocio_id)`), `limite` 20 y máximo 100. Cada fila trae el nombre del negocio y `motivo`, `detalle`, `estado`, `creado_en`; sin datos de quien reporta salvo su `usuario_id`.
- **Resolver:** cuerpo `{estado}` con `corregido` o `descartado`, `If-Match` con `version` (falta = 428, distinto = 412). Solo desde `abierto`; otro = 409. Guarda `resuelto_por` y `resuelto_en`. Corregir el negocio sigue siendo cosa del dueño o de la edición admin de nv-55: este endpoint no toca `negocios`.
- **Rate limit admin:** las dos rutas `/admin/reportes-negocio*` comparten la política `admin` (200 por minuto por IP, `rate:admin:{ip}`) antes de `RequireRoles`, como nv-43, nv-55 y nv-64.
- **ARCO:** `reportesnegocio.Privacy` se registra como `Purger` y `Exporter`. `PurgeAccount` borra todos los reportes de la persona (abiertos y resueltos). `PurgeHistory` no hace nada: un reporte no es historial de uso. `Export` devuelve sus reportes (`negocio_id`, nombre del negocio, `motivo`, `detalle`, `estado`, fechas).
- **Drift con el Excel:** (a) el Excel solo trae el `PATCH`; sin una lista el administrador no tiene cola, así que se agrega `GET /admin/reportes-negocio` con el índice `ix_reportes_cola` que el propio Excel define. (b) La fila del `PATCH` dice «Administrador o dueño» y luego «Administrador»; se toma solo administrador: el dueño ve y responde los reportes desde su panel (A5, nv-56 o una spec posterior). (c) Se agregan `resuelto_en`, `version` y dos índices.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/lugares/{id}/reportes` | JWT | 201, 400, 401, 403, 404, 409, 422, 429 |
| GET | `/api/v1/admin/reportes-negocio?estado=&negocio_id=&cursor=&limite=` | JWT administrador | 200, 401, 403, 422, 429, 503 |
| PATCH | `/api/v1/admin/reportes-negocio/{id}` | JWT administrador | 200, 400, 401, 403, 404, 409, 412, 422, 428, 429 |

Filas copiadas del Excel (A2): `POST /lugares/{id}/reportes` «Reportar información incorrecta (US-073)» (Body: motivo, detalle; 201; Consumidor); `PATCH /admin/reportes-negocio/{id}` «El administrador marca el reporte como corregido o descartado; el dueño solo responde desde su panel (A5)» (Body: estado; 200; Administrador). Se copian a `docs/endpoints.md` al ejecutar, junto con la lista agregada. 403 en el `POST` = cuenta en eliminación o dueño del negocio.

## Tests
- `tests/integration/reportes_negocio_test.go`: crear, 409 con otro abierto en el mismo negocio y 201 tras resolverse, `otro` sin `detalle` = 422, negocio oculto = 404, 429 en el 11.º del día; cola por estado y negocio con cursor; resolver con `If-Match` (412, 428), 409 al resolver dos veces.
- `tests/security/reportes_negocio_security_test.go`: 401 sin token, 403 de consumidor y comerciante en las rutas admin, 403 del dueño al reportar su negocio y con cuenta en eliminación, detalle rechazado por moderación = 422.
- `tests/integration/privacidad_reportes_test.go`: exportación con sus reportes; borrado de cuenta los quita todos; borrar historial no los toca.
- `tests/database/indexes_test.go`: los cuatro índices y `ux_reporte_abierto` único parcial. `tests/contract/reportes_negocio_contract_test.go`: 201, 200 de la cola y del `PATCH`.
- Unit: validación de `motivo` y `detalle`, transiciones de estado (`internal/reportesnegocio/service_test.go`).

## Criterios
- Nadie inunda la cola: un abierto por persona y negocio, 10 al día, sin gastar comandos de Redis propios.
- Solo un administrador cambia el estado de un reporte.

## Límites
- Sin vista del dueño ni respuesta del dueño (panel A5), sin notificaciones al dueño ni a quien reportó.
- Sin cambios automáticos al negocio por muchos reportes (por ejemplo, ocultar un «cerrado_permanente»).
- Los reportes de reseñas (J7, `POST /resenas/{id}/reportes`) van aparte.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-13), ola por decidir
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** `GET /admin/reportes-negocio` agregado; `PATCH` solo administrador; cualquier rol activo puede reportar salvo el dueño; un abierto por persona y negocio (409); 10 al día contados en MongoDB sin Redis; `PurgeAccount` borra también los resueltos; el `PATCH` no edita el negocio.
- **Detalle:** evidencia en `specs/nv-67/checklists/gates.md` + `Memoria.md` al ejecutar.
