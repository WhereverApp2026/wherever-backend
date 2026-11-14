# NV-69 — Extras de grupos: editar y disolver, punto de encuentro, modo sorpresa y «Me apunto»

## Contexto
Rutas de la hoja «J3 Grupos y chat» del Excel que ninguna spec cubría (NV-57 las dejó fuera). **Origen**: pantalla P-25, US-104 (sorpresa) y US-107 (traslados). **Depende de**: NV-05 (`preferencias.movilidad_default`, `RequireActive`), NV-29 y NV-30 (planes, `Velocidades`, `If-Match`), NV-36 (amistades y bloqueos), NV-37 (grupos e integrantes, `ocupados`), NV-38 (enlace y `groupForPlan`), NV-39 (votaciones), NV-46 (`sync_seq`), NV-57 (chat) y NV-68 (`visibilidad` del plan). Usa `negocios.Candidates` de NV-31 para los lugares sugeridos.

## Requisitos
1. El organizador cambia el nombre del grupo y lo disuelve.
2. Un integrante confirmado fija el punto de encuentro; la app sugiere 3 lugares justos para todos sin revelar dónde está cada quien.
3. Cada integrante ve cuánto tarda y gasta en llegar al punto, y de los demás solo un rango.
4. El organizador activa el modo sorpresa para una persona: ella no ve las paradas hasta la hora de revelar.
5. Una persona amiga del dueño se apunta a su plan visible y puede dejar de ir.
6. Unirse por enlace (`POST /grupos/unirse/{codigo}` del Excel) ya existe en NV-38; esta spec no agrega otra ruta.

## Diseño
### Editar y disolver
- **Drift con el código:** en NV-37 un grupo es de un solo plan (`plan_id` obligatorio) y no tiene `nombre`, `version` ni `eliminado_en`. Cambiar el plan de un grupo rompería esa relación 1:1, así que `PATCH` solo cambia `nombre` (1 a 40 caracteres; sin nombre, la vista usa el del plan). Se agregan `nombre` (opcional), `version` (int, 1 en los grupos existentes) y `eliminado_en`.
- **Versión:** `If-Match` en `PATCH` y `DELETE`, igual que planes (NV-30): sin cabecera = 400, vieja = 409.
- **Disolver:** borrado lógico (`eliminado_en`); se borra el `codigo_hash` del enlace y los integrantes pierden el acceso al plan (404). El plan sigue siendo del dueño. Si La Vaca (NV-58) tiene saldos sin liquidar = 409 `saldos_pendientes`. Los mensajes del chat se borran con el grupo en el trabajo nocturno de NV-57.
- **Quién:** solo el organizador; integrante = 403; ajeno = 404.

### Punto de encuentro
- **Fijar (`PUT`):** cualquier integrante confirmado. Cuerpo: `nombre` (1 a 60), `lat`, `lng`, `hora` (RFC 3339, el día del plan y no en el pasado) y `negocio_id` opcional; con `negocio_id`, el nombre y la ubicación salen del negocio aprobado y visible (422 si no). Guarda `punto_encuentro{nombre, ubicacion (Point), hora, propuesto_por, negocio_id}`, sube `version` y `actualizado_en` (la descarga de cambios de NV-46 lo ve). Sin `If-Match`: el último en fijarlo gana, como en el Excel.
- **Compartir mi ubicación:** `PATCH /grupos/{id}/integrantes/yo` (NV-37) gana `ubicacion{lat, lng}` opcional y `compartir_ubicacion: false` para retirarla. Se guarda **redondeada a 3 decimales** (≈110 m) en `integrantes.ubicacion_compartida` con `ubicacion_vence` a las 6 h; vencida se ignora y el trabajo nocturno la borra. **Nunca sale en ninguna respuesta**, ni a la propia persona: las vistas solo dicen `comparte_ubicacion: true|false`. Entra en el ARCO de grupos (exportar y purgar).
- **Sugerido (`GET`):** necesita al menos 2 integrantes confirmados con ubicación vigente (si no, 422 `faltan_ubicaciones`). El centro es el promedio de esas ubicaciones **ajustado a una cuadrícula de 0.01°** (≈1 km). Se buscan con `negocios.Candidates` 3 lugares aprobados, visibles y abiertos a la `hora` del punto (o a la llegada de la primera parada) en 1.5 km alrededor de ese centro, ordenados por la suma de distancias a todos.
- **Privacidad del sugerido:** la respuesta lleva solo las 3 tarjetas de lugar y `participantes` (cuántos compartieron). **No lleva el centro ni distancias por persona.** Con 2 personas, quien conoce su propia ubicación solo puede deducir la de la otra dentro de unos 3 km (cuadrícula más radio, al doble). La consulta no se registra con coordenadas en los logs.
- **Traslados (`GET`):** al `punto_encuentro` fijado (sin punto = 404 `sin_punto`). Para cada integrante confirmado con ubicación vigente, minutos con `Velocidades` de NV-29 según su `movilidad_default` y costo estimado con una tabla fija (a pie y bicicleta 0; auto por km; transporte público tarifa fija). La fila propia lleva `minutos` y `costo_estimado` exactos; la de cada otra persona solo `rango` (`hasta_15`, `15_30`, `30_60`, `mas_60`) y `costo_rango` en los mismos cortes por 50 pesos, con su `id`, `nombre` y `foto_url`. Quien no compartió aparece con `sin_ubicacion`.

### Modo sorpresa
- **`PUT`**, solo organizador: `{activo, para_usuario_id, revelar_en}`. `para_usuario_id` es un integrante confirmado distinto del organizador; `revelar_en` está entre ahora y el fin del día del plan. `activo: false` lo apaga. Se guarda `sorpresa{activo, para_usuario_id, revelar_en}` y sube `version`.
- **Recorte sin trabajo programado:** se calcula al leer (`now < revelar_en`), así que no hay temporizador ni estado en una instancia: las dos instancias dan lo mismo. Para esa persona, `GET /planes/{id}`, `GET /grupos/{id}`, el paquete sin conexión (NV-68) y la descarga de cambios (NV-46) solo traen `fecha`, la hora de la primera parada y el punto de encuentro (o, si no hay, el lugar de la primera parada); `paradas`, `traslados` y costos por parada van vacíos y llega `sorpresa: {revelar_en}`. Las votaciones (NV-39) de paradas ocultas no se le listan. `GET /grupos/{id}` de los demás muestra a quién va dirigida.
- **Escrituras:** mientras dure, esa persona no puede editar el plan ni votar paradas ocultas (404 como si no existieran).
- **Chat:** no se filtra (NV-57); queda en Límites.

### «Me apunto»
- **`POST /planes/{id}/me-apunto`:** el plan debe estar `publico` (NV-68); la persona debe ser amiga aceptada del dueño (NV-36) y no tener bloqueo con él en ningún sentido. Plan `privado` = 403 (como el Excel); plan inexistente, borrado o con bloqueo = 404.
- Si el plan no tiene grupo, se crea con `groupForPlan` de NV-38 (el dueño de organizador); el índice único de `plan_id` resuelve la carrera entre dos instancias (`ErrDuplicate` relee el grupo). Quien se apunta entra `confirmado` con el mismo `$inc ocupados` condicionado de NV-37; lleno = 409 como en NV-37 (el Excel decía 422). Ya confirmado = 200 sin cambios; invitado o declinado pasa a confirmado.
- **`DELETE /planes/{id}/me-apunto`:** es `Leave` de NV-37 buscando el grupo por plan; 204. El organizador no puede (409, que disuelva el grupo).
- **Límite:** comparte la política `grupos_unirse` de NV-38 (30 por hora por usuario).

### Unirse por enlace (corrección de drift)
- NV-38 ya implementó `POST /grupos/unirse` con `{codigo}` en el cuerpo: el código en la ruta quedaría en los logs de acceso y de CloudFront. No se agrega `POST /grupos/unirse/{codigo}`.

- **Límites nuevos:** `grupos_ubicacion`, 30 por hora por usuario (`rate:grupos_ubicacion:{usuario}`) para sugerido y traslados juntos. Las demás escrituras usan la política general de la API.
- **Variables:** ninguna nueva; cuadrícula, radio, rangos y tabla de costos son constantes.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| PATCH | `/api/v1/grupos/{id}` con `{nombre}` e `If-Match` | JWT organizador | 200, 400, 401, 403, 404, 409, 422 |
| DELETE | `/api/v1/grupos/{id}` con `If-Match` | JWT organizador | 204, 400, 401, 403, 404, 409 |
| PUT | `/api/v1/grupos/{id}/punto-encuentro` | JWT integrante confirmado | 200, 400, 401, 403, 404, 422 |
| GET | `/api/v1/grupos/{id}/punto-encuentro/sugerido` | JWT integrante | 200, 401, 404, 422, 429, 503 |
| GET | `/api/v1/grupos/{id}/punto-encuentro/traslados` | JWT integrante | 200, 401, 404, 429, 503 |
| PUT | `/api/v1/grupos/{id}/sorpresa` | JWT organizador | 200, 400, 401, 403, 404, 422 |
| POST | `/api/v1/planes/{id}/me-apunto` | JWT amigo del dueño | 200, 401, 403, 404, 409, 429 |
| DELETE | `/api/v1/planes/{id}/me-apunto` | JWT integrante | 204, 401, 403, 404, 409 |
| PATCH | `/api/v1/grupos/{id}/integrantes/yo` (gana `ubicacion` y `compartir_ubicacion`) | JWT integrante | como NV-37 |
| POST | `/api/v1/grupos/unirse` con `{codigo}` | JWT | sin cambios (NV-38) |

403 también = cuenta en eliminación pendiente en las escrituras (`RequireActive`). Las rutas de `/planes/{id}/me-apunto` las registra el módulo de grupos, como `/planes/{id}/invitaciones`. Al ejecutar, las filas pasan a `docs/endpoints.md` y la política nueva a `docs/rate-limit-matrix.md`.

## Tests
- `tests/integration/grupos_extras_test.go`: renombrar y disolver con `If-Match` (400, 409), disolver deja el plan al dueño y a los demás en 404, 409 con saldos de La Vaca; punto fijado con y sin negocio, hora pasada = 422.
- `tests/integration/punto_encuentro_test.go`: sugerido con 2 y 3 ubicaciones, 422 con menos de 2 o vencidas, lugares abiertos a la hora; traslados exactos solo en la fila propia.
- `tests/integration/sorpresa_test.go`: la persona sorprendida ve solo el inicio en plan, grupo, paquete y descarga de cambios hasta `revelar_en` y todo después; votaciones ocultas; los demás ven todo.
- `tests/integration/me_apunto_test.go`: amigo entra confirmado y crea el grupo si falta, dos apuntes simultáneos crean un solo grupo, privado = 403, no amigo y bloqueado = 404, lleno = 409, salir = 204, organizador = 409.
- `tests/security/grupos_extras_security_test.go`: 401 en todas; 403 de integrante en `PATCH`, `DELETE` y `sorpresa`; ninguna respuesta ni log lleva `ubicacion_compartida`, el centro del sugerido ni coordenadas de integrantes; la sorpresa no se filtra por ninguna de las rutas de lectura listadas.
- `tests/database/indexes_test.go`: sin índices nuevos (se reusan `ix_plan` único e `ix_mis_grupos`). `tests/contract/grupos_extras_contract_test.go`: forma de las respuestas.
- `tests/integration/presupuesto_redis_test.go`: `grupos_ubicacion` cuesta lo mismo que una ventana existente.
- Unit: cuadrícula y promedio, rangos de traslado, tabla de costos, recorte de sorpresa por hora y validaciones (`internal/grupos`).

## Criterios
- Nadie puede saber dónde está otra persona del grupo con más precisión que unos kilómetros, y la persona sorprendida no ve las paradas antes de tiempo por ninguna ruta de lectura.

## Límites
- Sin cambiar el plan de un grupo (relación 1:1 de NV-37).
- Sin filtrar el chat en modo sorpresa ni eventos en vivo del punto de encuentro (spec de tiempo real del grupo).
- Sin rutas reales con API de mapas: traslados por `Velocidades` y tabla fija.
- Sin `POST /grupos/unirse/{codigo}` (sigue el `POST` con cuerpo de NV-38).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-13), ola por decidir
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** `PATCH` solo cambia `nombre`; `version` e `If-Match` en grupos; disolver bloqueado por saldos de La Vaca; ubicación guardada a 3 decimales, 6 h y nunca devuelta; centro a cuadrícula de 1 km y sin centro en la respuesta (con 2 personas la fuga queda en ≈3 km); traslados ajenos solo por rango; sorpresa calculada al leer, sin trabajo programado; me-apunto exige plan `publico` y amistad, lleno = 409; unirse sigue en el `POST` de NV-38.
- **Detalle:** evidencia en `specs/nv-69/checklists/gates.md` + `Memoria.md` al ejecutar.
