# NV-65 — Eventos, ferias y tianguis

## Contexto
Inicio y Explorar muestran eventos, ferias y tianguis cercanos de los próximos días (US-030). Los publica el administrador o un comerciante con negocio aprobado. **Origen**: hoja «A2 Lugares y búsqueda» del Excel `Wherever_Endpoints_CRUD_v3.xlsx` (colección «eventos» y cinco filas de endpoints). **Depende de**: NV-04 (`RequireRoles`, roles), NV-05 (`RequireActive`, ARCO con `privacidad.Purger`/`Exporter`), NV-17 (cursor firmado), NV-20/NV-55 (`negocios`, `propietario_id`, estado `aprobado`), NV-42 (moderación de texto) y NV-45 (`foto_url` de imágenes subidas).

## Requisitos
1. `GET /eventos` lista los eventos publicados cercanos que empiezan entre hoy y hoy + `dias` (7 por omisión), por fecha y distancia; público.
2. `GET /eventos/{id}` da el detalle de un evento publicado; público.
3. `POST /eventos` publica un evento; administrador, o comerciante dueño de un negocio aprobado.
4. `PATCH /eventos/{id}` edita o cancela; `DELETE /eventos/{id}` lo borra. Solo quien lo creó, el dueño del negocio organizador o un administrador.
5. Los eventos terminados dejan de mostrarse; los eventos de una persona entran en su exportación y en el borrado de su cuenta.

## Diseño
- **Paquete `internal/eventos`:** modelo, repositorio, service, handler, DTO y `privacy.go`. El armado para pruebas va en `internal/testutil/eventostest`.
- **Colección `eventos`** (Excel A2 §2): `titulo` (≤ 60), `tipo` (`evento`, `feria`, `tianguis`), `descripcion` (≤ 300, opcional), `negocio_id` (opcional), `ubicacion` (GeoJSON Point), `direccion` (≤ 200), `inicio`, `fin`, `precio` (centavos, 0 = gratis), `foto_url` (opcional), `estado` (`borrador`, `publicado`, `cancelado`), `creado_por`, `version`, `creado_en`, `actualizado_en`.
- **Índices** (`scripts/indexes` y `/ready`): `ix_eventos_cerca` `{ubicacion: "2dsphere", estado: 1, inicio: 1}`, `ix_eventos_fin` `{fin: 1}`, `ix_eventos_creador` `{creado_por: 1}` (ARCO y permisos) y `ix_eventos_negocio` `{negocio_id: 1}` parcial con `negocio_id` presente.
- **Lista:** `lat` y `lng` obligatorios, `radio_m` (10 000 por omisión, máximo 50 000), `dias` 1–30 (7), `tipo` opcional. Filtra `estado = publicado`, `inicio` en `[hoy, hoy + dias]` (día en `America/Mexico_City`) y `fin > ahora`. Orden por `inicio` y luego distancia; cursor firmado de nv-17 (`Signer.Scope("eventos", lat, lng, radio_m, dias, tipo)`), `limite` 20 y máximo 50. Cada evento lleva `distancia_m`.
- **Detalle:** `borrador` y `cancelado` solo los ven quien puede editar; para el resto, 404. Un id mal formado es 404.
- **Crear:** comerciante: `negocio_id` obligatorio, negocio suyo (`propietario_id`) y `estado = aprobado`, sin pausa ni baja; si no, 403. Administrador: `negocio_id` opcional. `fin > inicio`, `inicio` desde hace 1 h hasta 180 días adelante y duración máxima de 30 días; si no, 422. `estado` inicial `publicado` (o `borrador` si lo pide). `titulo` y `descripcion` pasan por la moderación de texto de nv-42: rechazo = 422 con el motivo genérico. `foto_url` solo acepta URLs del bucket de nv-45. Responde 201 con `Location` y `ETag`.
- **Editar:** cuerpo parcial con los mismos campos (sin `negocio_id` ni `creado_por`), `If-Match` con `version` obligatorio (falta = 428, distinto = 412). `estado = cancelado` cancela: el evento sigue en el detalle con su estado, fuera de la lista. Un evento cancelado no vuelve a `publicado`.
- **Borrar:** `If-Match` igual que editar; 204 y borrado físico (no es historial de nadie).
- **Permisos de edición:** `creado_por` = quien pregunta, o dueño del `negocio_id`, o rol `administrador`; se valida en el service. Otro usuario = 403; evento inexistente = 404.
- **Límite:** política nueva `eventos_publicar`, 10 por día por usuario (`Limiter.WindowUser`, `rate:eventos_publicar:{usuario}`) en `POST /eventos`. Cuesta 3 comandos de Redis por publicación, como las demás ventanas; publicar es raro y no mueve la estimación de nv-11. Las lecturas no tienen política propia (solo la global) ni caché en Redis.
- **ARCO:** `eventos.Privacy` se registra como `Purger` y `Exporter`. `PurgeAccount` borra los eventos que creó sin `negocio_id` y quita `creado_por` de los que pertenecen a un negocio (siguen siendo del negocio). `PurgeHistory` no hace nada: no es historial. `Export` devuelve sus eventos (título, tipo, fechas, estado, negocio).
- **Drift con el Excel:** se agregan `creado_por` y `version` (los pide «quien lo creó» y el `If-Match` de `AGENTS.md` §5); `precio` en centavos (constitución); `GET /eventos` exige `lat`/`lng` (el índice es 2dsphere). La sección de eventos de `GET /inicio` queda fuera (nv-25 no la tiene): se agrega cuando exista esta colección, con la misma consulta.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/eventos?lat=&lng=&radio_m=&dias=&tipo=&cursor=&limite=` | Público | 200, 422, 429, 503 |
| GET | `/api/v1/eventos/{id}` | Público (sesión opcional) | 200, 404, 429 |
| POST | `/api/v1/eventos` | JWT administrador o comerciante | 201, 400, 401, 403, 422, 429 |
| PATCH | `/api/v1/eventos/{id}` | JWT creador, dueño del negocio o administrador | 200, 400, 401, 403, 404, 409, 412, 422, 428, 429 |
| DELETE | `/api/v1/eventos/{id}` | JWT creador, dueño del negocio o administrador | 204, 401, 403, 404, 412, 428, 429 |

Filas copiadas del Excel (A2): `GET /eventos` «Eventos, ferias y tianguis cercanos de los próximos días (US-030)» (Query: lat, lng, dias (7), tipo; Público); `GET /eventos/{id}` «Detalle de un evento» (Público); `POST /eventos` «Publica un evento» (Body: titulo, tipo, descripcion, ubicacion, inicio, fin, precio; 201; Administrador o comerciante); `PATCH /eventos/{id}` «Edita o cancela un evento» (Body parcial; 200; Quien lo creó o administrador); `DELETE /eventos/{id}` «Borra un evento» (204; Quien lo creó o administrador). Se copian a `docs/endpoints.md` al ejecutar. 409 en PATCH = reabrir un evento cancelado.

## Tests
- `tests/integration/eventos_test.go`: lista por distancia y fecha (fuera los de más de `dias`, terminados, borradores y cancelados), cursor, `tipo`; crear como comerciante con negocio aprobado y como admin; comerciante con negocio ajeno o no aprobado = 403; editar con `If-Match` (412, 428), cancelar y no reabrir (409); borrar; 429 en la 11.ª publicación.
- `tests/security/eventos_security_test.go`: 401 sin token en escrituras, consumidor = 403 al crear, otro comerciante = 403 al editar y borrar, borrador ajeno = 404, texto rechazado por moderación = 422.
- `tests/integration/privacidad_eventos_test.go`: exportación con sus eventos; borrado de cuenta borra los suyos sin negocio y quita `creado_por` de los del negocio.
- `tests/database/indexes_test.go`: los cuatro índices. `tests/contract/eventos_contract_test.go`: 200 de lista y detalle, 201, 204.
- `tests/integration/presupuesto_redis_test.go` (`TestPresupuestoRedis_EventosNV65`): `eventos_publicar` cuesta lo mismo que otra ventana por usuario.
- Unit: validación de fechas, duración y `tipo`; permisos de edición; transiciones de estado (`internal/eventos/service_test.go`).

## Criterios
- Un consumidor nunca publica; un comerciante solo publica para su negocio aprobado y solo edita lo suyo.
- Ningún evento terminado, borrador o cancelado aparece en la lista.

## Límites
- Sin eventos en `GET /inicio`, sin notificaciones de eventos, sin asistencia ni «me interesa».
- Sin sincronización sin conexión propia (nv-47 la agrega si hace falta).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-13), ola por decidir
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** `creado_por` y `version` nuevos; `lat`/`lng` obligatorios; comerciante solo con negocio aprobado propio; el dueño del negocio también edita; cancelado no se reabre; borrado físico; `eventos_publicar` 10 por día; ARCO conserva los eventos del negocio sin `creado_por`; Inicio sin eventos por ahora.
- **Detalle:** evidencia en `specs/nv-65/checklists/gates.md` + `Memoria.md` al ejecutar.
