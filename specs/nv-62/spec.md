# NV-62 — Check-in y reseñas verificadas por aspecto

## Contexto
El check-in comprueba que la persona sí llegó (por ubicación a ≤ 100 m o escaneando el `qr_checkin` del mostrador). Solo quien hizo check-in reseña, una vez por visita: calificación general, 5 aspectos, chips, comentario y hasta 3 fotos. El negocio responde y reporta; cada reseña publicada recalcula `negocios.calificacion`. **Origen**: hoja «J7 Check-in y reseñas» del Excel (roles C10 y C13; pantallas P-27, P-28, «Por reseñar», «Mis reseñas» y P-50), US-113. **Depende de**: NV-20 (`negocios`, `calificacion`, `qr_checkin`), NV-17 (cursor), NV-18 (ficha en caché), NV-21 (`/lugares/{id}`), NV-26 a NV-28 (IA), NV-42 y NV-43 (moderación de texto e imágenes y cola del administrador), NV-45 (subida de imágenes), NV-56 (panel del negocio: P-50).

## Requisitos
1. `POST /checkins` por `ubicacion` (≤ 100 m, precisión creíble, sin saltos imposibles) o `qr_negocio` (el `qr_checkin` de nv-20); un check-in válido por usuario, negocio y día.
2. «Por reseñar» lista los check-ins válidos sin reseña de los últimos 30 días.
3. `POST /resenas` exige un check-in válido propio sin reseña (`ux_resena_checkin`): una reseña por visita verificada, y a lo más una por usuario y negocio cada 30 días (409) para que visitas diarias no inflen la calificación.
4. Texto y fotos pasan por la moderación de nv-42; la reseña queda `en_moderacion` y solo cuenta en la calificación al quedar `publicada`.
5. Recalcular `negocios.calificacion` (`promedio`, `total_resenas`, `aspectos`) de forma incremental y atómica al publicar, editar, ocultar o borrar.
6. El autor edita en las primeras 48 h y borra cuando quiera; «me fue útil» se marca y desmarca.
7. El dueño del negocio responde (con sugerencia opcional de IA) y ve un resumen por aspecto con tendencia mensual.
8. Reportes con motivo predefinido; a los 3 reportes de personas distintas la reseña se oculta y entra a la cola de nv-43.
9. Límites de peticiones por usuario en check-in, reseñas, útil, reportes y sugerencias.
10. ARCO: check-ins y reseñas se exportan; al eliminar la cuenta se borran los check-ins y las reseñas se anonimizan (nv-05).

## Diseño
- **Paquete `internal/resenas`** (check-ins y reseñas juntos, porque la reseña depende del check-in): modelo, repositorios, service, handler, DTO, `Privacy` e índices en `resenas.Indexes()` + `internal/indexes.All()`; el único creador sigue siendo `scripts/indexes`.
- **Colección `checkins`** (J7 §1): `usuario_id`, `negocio_id`, `plan_id` (opcional, parada de un plan de nv-30), `metodo`, `distancia_m`, `precision_m`, `valido`, `motivo_rechazo` (`fuera_de_radio`, `ubicacion_falsa`, `duplicado`), `dia` (fecha local del negocio, `AAAA-MM-DD`), `resena_id`, `creado_en`. Índices `ix_mis_checkins`, `ix_checkins_negocio`, `ix_por_resenar` y `ux_checkin_dia` (único parcial `{usuario_id, negocio_id, dia}` con `valido = true`). **Drift:** no se guarda `ubicacion_usuario` (minimización: basta `distancia_m`); `reserva_id`, `qr_reserva`, `negocio_confirma` y `ux_checkin_reserva` esperan la spec de reservas; `puntos_otorgados` espera la de recompensas.
- **Check-in por ubicación:** distancia haversine entre `lat`/`lng` y `negocios.ubicacion` ≤ 100 m y `precision_m` ≤ 100. Salto imposible: velocidad > 300 km/h entre la ubicación del negocio del último check-in válido y la del nuevo. Si falla, se guarda `valido = false` con el motivo y responde 422 con el mensaje neutro del Excel («Parece que aún no llegas. ¿Escaneas el código?»), sin acusar; `ubicacion_falsa` se registra en el log de eventos (sin coordenadas) para revisión.
- **Check-in por QR:** `codigo` se busca en `ux_qr_checkin` (nv-20); debe ser de un negocio visible e igual a `negocio_id` (si no, 422 igual que fuera de radio, sin revelar de qué negocio es). No exige ubicación.
- **Duplicado:** el segundo check-in válido del mismo día choca con `ux_checkin_dia` → 409 (sin guardar otro documento). Así una persona tiene a lo más una visita verificada por negocio y día.
- **Colección `resenas`** (J7 §2): `negocio_id`, `usuario_id` (null si se anonimizó), `checkin_id`, `calificacion` 1–5, `aspectos` {`sabor`, `atencion`, `ambiente`, `espera`, `precio_calidad`} 1–5, `chips` (≤5, de un catálogo fijo), `comentario` (≤500), `fotos` (≤3 ids de `imagenes` de nv-45 del mismo autor, ya `publicada`), `estado` (`en_moderacion`, `publicada`, `oculta`, `eliminada`), `contada` (bool: si suma en la calificación), `util`, `respuesta_negocio` {`texto`, `autor_id`, `fecha`, `estado`}, `reportes[]` {`por`, `motivo`, `fecha`}, `version`, `creado_en`, `actualizado_en`. Índices `ux_resena_checkin`, `ix_resenas_utiles`, `ix_resenas_estrellas`, `ix_mis_resenas`, `ix_resenas_texto` (`default_language: spanish`) e `ix_reportadas` (parcial con `reportes.0`).
- **Aspectos fijos (drift):** se usan los 5 de `negocios.Aspectos` para todos los giros; los aspectos por giro (cine: pantalla, comodidad) cambiarían el modelo de `negocios` y quedan fuera.
- **Colección `resenas_utiles`:** `{resena_id, usuario_id, creado_en}` con `ux_util` único; `PUT /util` alterna con insert o delete y `$inc` de ±1 en `util` solo si cambió (idempotente ante reintentos de la misma intención con `{"util": true|false}` en el cuerpo).
- **Moderación:** al crear o editar, `moderacion.Evaluar` (nv-42) sobre comentario y chips libres; las fotos ya vienen moderadas por nv-45. `aprobado` → `publicada`; `cuarentena` → `en_moderacion` y entra a `moderacion_cola` de nv-43, que al aprobar llama `resenas.Publicar`; `rechazado` → 422 con motivo genérico. Si la moderación falla, cierra (`en_moderacion`). La respuesta del negocio pasa por lo mismo. El Excel decía Cloud Vision; nv-42 usa Rekognition.
- **Recálculo de `calificacion` (atómico e incremental):** se agrega a `negocios` el subdocumento interno `calificacion_suma` {`total`, `estrellas`, `sabor`, `atencion`, `ambiente`, `espera`, `precio_calidad`} en enteros (no se expone en JSON). Contar o descontar una reseña es **un** `updateOne` con pipeline: `$inc` lógico de las sumas y `$set` de `calificacion.promedio = estrellas/total` y de cada aspecto (redondeo a 1 decimal; con `total = 0`, todo 0). Editar aplica la diferencia (nuevo − anterior) en el mismo update. La bandera `contada` en la reseña se cambia con filtro condicional (`contada: false → true`) **antes** del update del negocio, así un reintento no cuenta dos veces; si el update del negocio falla, se revierte la bandera. Después se borra `cache:negocio:{id}` (nv-18).
- **Negocios de la semilla:** traen `calificacion` sin `calificacion_suma`. La primera vez se inicializa con `$ifNull`: `total = total_resenas`, `estrellas = round(promedio × total)` y cada aspecto igual; desde ahí todo es exacto. Un recálculo completo (`scripts/` de mantenimiento) queda fuera.
- **Editar (48 h, If-Match):** solo el autor, `If-Match` con `version` (412/428); pasado el plazo 403. Vuelve a moderar; si estaba contada, aplica la diferencia.
- **Borrar:** el autor; pasa a `eliminada` (borrado lógico para que el check-in no se reutilice) y descuenta si estaba contada. Ocultar por moderación o reportes también descuenta; volver a publicar suma.
- **Reportes:** motivos `ofensiva`, `falsa`, `spam`, `datos_personales`, `otro`; uno por persona y reseña (409); el autor no reporta la suya. A los 3 reportes de personas distintas pasa a `oculta`, descuenta y entra a la cola de nv-43; el dueño del negocio cuenta como uno solo. El administrador la restablece o la deja oculta desde nv-43.
- **Lista pública (`/lugares/{id}/resenas`):** solo `publicada`; filtros `estrellas` (1–5), `aspecto` (ese aspecto ≥ 4), `con_fotos`, `q` (texto dentro del negocio) y `orden` `utiles` o `recientes`, con cursor de nv-17. El autor se muestra con `nombre_usuario` y foto; anonimizada = «Usuario de Wherever». Nunca correo, teléfono ni `checkin_id`.
- **Resumen del negocio:** agregación por `periodo` (`mes` o `trimestre`) con promedio por aspecto del periodo y del anterior, distribución de estrellas y total. El «avance al Sello» espera la regla de `sello_excelencia` de A5 / nv-56.
- **IA:** `sugerir-respuesta` usa nv-26 con el tope de nv-27 y el límite de nv-28; manda solo el texto de la reseña, sin nombre ni ids del autor; 503 si no hay IA.
- **Límites (`Limiter.WindowUser`):** check-in 10/h (`rate:checkins:{usuario}`), crear reseña 10/día (`rate:resenas_crear:{usuario}`), útil 60/h (`rate:resenas_util:{usuario}`), reportes 20/día (`rate:resenas_reporte:{usuario}`), sugerir respuesta 30/día (`rate:resenas_ia:{usuario}`). Lista pública con la política global. 5 llaves nuevas con TTL; costo por `TestPresupuestoRedis_*`.
- **ARCO:** `Purger` al eliminar la cuenta: borra `checkins` y `resenas_utiles` del usuario (los contadores `util` quedan), quita `por` de sus reportes y anonimiza sus reseñas (`usuario_id = null`; texto y calificación se quedan, como manda nv-05); sus fotos las borra nv-45. «Borrar historial» borra los check-ins sin reseña. `Exporter` agrega `checkins` (`negocio_id`, `metodo`, `valido`, `distancia_m`, fecha) y `resenas` (todo lo que escribió + `respuesta_negocio`).
- **Roles:** consumidor con `usuarios.RequireActive`; dueño = `comerciante` + `propietario_id = sub` (ajeno = 404); reseña ajena para editar o borrar = 404.

## Drift corregido contra el código (2026-11-14)
Lo que sigue manda sobre el Diseño de arriba donde choquen.
- **Moderación (nv-42):** `resenas.Moderador` es la interfaz `Evaluar(ctx, moderacion.Contenido) moderacion.Resultado`. La API arma `moderacion.New(nil, moderacion.NewTextoIA(guardia), modSvc, log)`: umbrales de `moderacion_reglas` y la `ia.Guardia` compartida (tope y límites de nv-27/nv-28). Sin IA no hay moderador de texto y todo comentario queda `en_moderacion` (fail-closed). Solo se modera el `comentario`: los chips salen de un catálogo fijo. Una reseña sin comentario se publica sin llamar al moderador (`Evaluar` con contenido vacío da cuarentena).
- **Cola (nv-43):** `moderacion.Service.Encolar(Entrada{Tipo, Referencia, AutorID, Texto, Resultado})` exige veredicto `cuarentena` y texto; la reseña entra con `Tipo: "resena"` y la respuesta del negocio con `"resena_respuesta"`. Los reportes entran igual, con `Falla: "reportes"`. `resenas.Destino` se registra con `WithDestino` para los dos tipos: aprobar publica (o restablece) y cuenta; rechazar deja `rechazada` (estado nuevo, cuenta para la regla del Sello). Rechazo inmediato del moderador = 422 sin guardar (la visita sigue libre). Mientras la reseña está `en_moderacion` no se edita (409): así la cola nunca decide sobre un texto viejo.
- **Fotos (nv-45):** `imagenes.Service.Publicada(ctx, autor, id)` devuelve la URL de una imagen publicada propia (`ErrNoPublicada` → 422). La reseña guarda `fotos[{id, url}]`; las fotos ya vienen moderadas. Al anonimizar se vacía `fotos` (nv-45 borra los objetos).
- **P-50 (nv-56):** `Service` cumple `negocios.FuenteResenas` y se registra en `PanelFuente.Resenas` (`buildPanel`). `LlegadasHoy` = check-ins válidos del día; `CalificacionSemana` y `CambioVsSemanaAnterior` = promedio de estrellas de las publicadas de los últimos 7 días contra los 7 anteriores; `PorResponder` = publicadas sin respuesta; `Verificadas90`, `Promedio90` y `RechazadasModeracion90` para la regla del Sello.
- **Caché de la ficha (nv-18):** cada cambio de `calificacion` llama `negocios.Service.InvalidarFicha` (borra `cache:negocio:{id}` y categorías).
- **`qr_checkin` (nv-20):** `negocios.Repository.FindVisibleByQR` busca por `ux_qr_checkin` solo entre visibles.
- **Calificación:** `negocios.Repository.AplicarSuma` hace el `updateOne` de pipeline (inicializa `calificacion_suma` con `$ifNull` desde la semilla, suma con `$max: 0` y recalcula promedio y aspectos con `$round` a 1 decimal). Si falla tras mover la bandera `contada`, la reseña vuelve a su documento anterior (`ReplaceOne` condicionado a la versión) y la petición da 503.
- **If-Match:** como nv-61 y el resto del código (`apperr` no tiene 412/428): falta o mal formada = 400, versión vieja = 409. Las filas de contrato se ajustan.
- **Zona del día:** fija `negocios.MexicoCity()` (como nv-59 y nv-61).
- **Límites:** 4 políticas nuevas (`checkins`, `resenas_crear`, `resenas_util`, `resenas_reporte`); `sugerir-respuesta` usa el límite por usuario de la guardia (`rate:ia_usuario`, 429), sin llave `rate:resenas_ia`. Cada petición limitada cuesta el pipeline de 3 comandos de Upstash; el resto va en MongoDB.
- **Índices:** se agrega `ix_resenas_recientes` (`negocio_id, estado, creado_en, _id`) porque `ix_resenas_estrellas` no sirve el orden por fecha sin filtro de estrellas; `ix_reportadas` es `reportes.por` parcial (lo usa ARCO); `resenas_utiles` lleva `ix_util_usuario` para ARCO; `ix_resenas_texto` es compuesto `{negocio_id, comentario: text}`.
- **Precisión mala** (`precision_m` > 100) se guarda como `fuera_de_radio`; el salto imposible solo se revisa por ubicación y con más de 1 km entre negocios.

## Contratos
Filas de J7 §3, sin los métodos de reserva. Se copian a `docs/endpoints.md` al programar. Fiber registra `/resenas/pendientes` y `/resenas/mias` antes de `/resenas/{id}`.

| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/checkins` | JWT consumidor | 201, 400, 401, 403, 404, 409, 422, 429 |
| GET | `/api/v1/checkins?cursor=` | JWT consumidor | 200, 401, 403 |
| GET | `/api/v1/resenas/pendientes` | JWT consumidor | 200, 401, 403 |
| POST | `/api/v1/resenas` | JWT consumidor | 201, 400, 401, 403, 404, 409, 422, 429 |
| GET | `/api/v1/lugares/{id}/resenas?estrellas=&aspecto=&con_fotos=&q=&orden=&cursor=` | Público | 200, 404, 422, 429 |
| GET | `/api/v1/resenas/mias?cursor=` | JWT consumidor | 200, 401, 403 |
| PATCH | `/api/v1/resenas/{id}` | JWT autor + If-Match | 200, 400, 401, 403, 404, 409, 422 |
| DELETE | `/api/v1/resenas/{id}` | JWT autor | 204, 401, 403, 404 |
| PUT | `/api/v1/resenas/{id}/util` | JWT | 200, 400, 401, 403, 404, 429 |
| PUT | `/api/v1/negocios/{id}/resenas/{rid}/respuesta` | JWT dueño | 200, 400, 401, 403, 404, 422 |
| POST | `/api/v1/negocios/{id}/resenas/{rid}/sugerir-respuesta` | JWT dueño | 200, 401, 403, 404, 429, 503 |
| POST | `/api/v1/resenas/{id}/reportes` | JWT | 201, 400, 401, 403, 404, 409, 422, 429 |
| GET | `/api/v1/negocios/{id}/resenas/resumen?periodo=` | JWT dueño | 200, 401, 403, 404, 422 |

403 en `POST /resenas` = sin check-in válido propio (como el Excel), además de rol y eliminación pendiente; 409 = esa visita ya tiene reseña. `DELETE` sin `If-Match`: es borrado lógico idempotente del autor.

## Tests
- `tests/integration/resenas_test.go` con MongoDB y Redis reales: check-in dentro y fuera de radio, precisión mala, salto imposible, QR correcto y de otro negocio, duplicado del día (409), «por reseñar», reseña sin check-in (403), segunda reseña de la misma visita (409), moderación aprobada, cuarentena y rechazada (con el moderador de nv-42 simulado como interfaz externa), recálculo al publicar, editar, borrar y ocultar, semilla sin `calificacion_suma`, 48 h, útil idempotente, respuesta del negocio, 3 reportes ocultan, resumen por periodo, filtros y texto, caché de la ficha borrada, borrar cuenta (anonimiza) y exportar.
- `TestResenas_Concurrencia_UnaPorVisita`: 3 rondas de 5 `POST /resenas` simultáneos con el mismo `checkin_id` dejan una reseña y `total_resenas` +1.
- `TestResenas_Concurrencia_RecalculoExacto`: 20 reseñas publicadas en paralelo en un negocio dejan `total` y promedios iguales a los calculados desde cero.
- `TestCheckins_Concurrencia_UnoPorDia`: 5 check-ins simultáneos del mismo usuario y negocio dejan uno válido.
- `tests/security/resenas_security_test.go`: 401, 403 por rol y en eliminación pendiente, 404 con reseña o negocio ajeno, 429 en check-in y reportes; la lista pública no expone correo, teléfono ni `checkin_id`.
- `tests/database/indexes_test.go`: índices de `checkins`, `resenas` y `resenas_utiles`. `tests/contract`: `checkins: []` y `resenas: []` en la exportación.
- Unit `internal/resenas/*_test.go`: haversine, velocidad, `dia` en la zona del negocio, diferencia de sumas, redondeo, plazo de 48 h, DTOs y catálogo de chips.

## Criterios
- `negocios.calificacion` siempre coincide con las reseñas `publicada` contadas.
- Nadie reseña sin una visita verificada propia, y cada visita da a lo más una reseña.

## Límites
- Sin reservas: `qr_reserva`, `negocio_confirma` y `reserva_id` llegan con su spec.
- Sin puntos ni sellos del pasaporte (spec de recompensas) y sin el aviso «¿Qué tal estuvo?» mientras nv-40 y nv-41 estén pospuestas.
- Sin aspectos por giro ni avance al Sello.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-14), implementada en `internal/resenas` (ola 10). Sin firma.
- **Revisores:** Osmar Reséndiz (decisiones aceptadas el 2026-11-14; firma pendiente)
- **Fecha:** —
- **Decisiones del revisor (Osmar Reséndiz, 2026-11-14), aceptadas tal como las proponía la spec:** un check-in válido por negocio y día; una reseña por negocio cada 30 días, consultada con `ix_mis_resenas`; no se guarda la ubicación del usuario; campo nuevo `calificacion_suma` en `negocios`, inicializado desde la semilla; 5 aspectos fijos; 3 reportes ocultan una reseña; al eliminar la cuenta las reseñas se anonimizan, no se borran; reservas y puntos quedan fuera.
- **Drift corregido:** ver «Drift corregido contra el código (2026-11-14)» (moderación de nv-42 y cola de nv-43, fotos de nv-45, P-50 de nv-56, caché de nv-18, `qr_checkin` de nv-20, If-Match 400/409, sin `rate:resenas_ia`, estado `rechazada`, `ix_resenas_recientes`).
- **Pruebas:** `tests/integration/resenas_test.go`, `tests/security/resenas_security_test.go`, `tests/database/indexes_test.go`, contrato `usuarios/exportacion_descargar_200` (`checkins` y `resenas`) y Unit `internal/resenas/reglas_test.go`.
- **Detalle:** evidencia en `specs/nv-62/checklists/gates.md`.
