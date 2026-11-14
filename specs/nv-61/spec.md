# NV-61 — Cupones: creación, cartera y canje con código de un solo uso

## Contexto
El negocio crea cupones (porcentaje, monto, 2x1 o regalo) con vigencia, días y horas válidos y límite de usos, y puede lanzar uno relámpago de 3 horas. El usuario los ve cerca, los guarda en su cartera y al canjear muestra un código de un solo uso que vive 5 minutos; el negocio lo escanea o teclea y el servidor responde válido o no. **Origen**: hoja «J6 Cupones» del Excel (roles C5 y C13; pantallas P-29, P-30, P-49 y el carrusel «Cupones cerca» de P-14), US-097. **Depende de**: NV-20 (`negocios`, `cupones_activos`), NV-17 (cursor), NV-26 a NV-28 (`ia.Guardia`: tope de gasto y límite por usuario), NV-56 (panel del negocio: tarjeta de P-49 por `negocios.FuenteCupones` y pausa o baja por `negocios.Observador`).

**Drift corregido (2026-11-14, contra `main` con nv-56, nv-59 y nv-60):** la pausa y la baja llegan por `negocios.Observador` (se registra en `buildReservas`, `cmd/api/reservas.go`), no por un gancho en el service de negocios; la tarjeta de P-49 es `negocios.FuenteCupones` (`canjeados_hoy`, `activos`) en `buildPanel`; la ficha en caché de nv-18 (`cache:negocio:{id}`) no muestra `cupones_activos` (solo lo lee favoritos desde MongoDB), así que el trabajo no borra la caché ni gasta Redis; no hay zona por ciudad: días y horas en `negocios.MexicoCity()`; `If-Match` sigue la convención del proyecto (falta = 400, versión vieja = 409; `apperr` no tiene 412 ni 428); la IA es la `ia.Guardia` compartida con el asistente (`GenerarValidado` + `ia.ConUsuario`), así que sugerir título comparte `rate:ia_usuario` (10/h) con los planes.

## Requisitos
1. El dueño del negocio crea, lista, edita, pausa y borra cupones; si ya tienen canjes solo cambian vigencia, límite y estado, y no se borran (409, se pausan).
2. El relámpago vale de ahora a ahora + 3 h; la vigencia es coherente (`inicio < fin`) y el `valor` tiene sentido para el `tipo`.
3. `GET /cupones/cerca` y `GET /cupones/{id}` son públicos y solo muestran cupones activos, vigentes y de negocios visibles.
4. El consumidor guarda un cupón una sola vez, lo filtra (vigentes, por vencer, usados), busca por prefijo y lo quita.
5. Canjear genera un código de 6 caracteres y un QR que viven 5 minutos; pedirlo otra vez mientras vive devuelve el mismo.
6. Validar es atómico e idempotente: un código se canjea una sola vez aunque lleguen dos escaneos a la vez, y el reintento del mismo negocio recibe la misma respuesta verde.
7. Estadísticas por cupón y día: vistas, guardados y canjes (US-097).
8. Un trabajo activa los programados, vence los terminados y recalcula `negocios.cupones_activos`.
9. Límites de peticiones por usuario en guardar, canjear, validar y sugerir título.
10. ARCO: la cartera se borra con la cuenta y se exporta.

## Diseño
- **Paquete `internal/cupones`:** modelo, repositorio, service, handler, DTO, `Privacy` (Purger y Exporter de nv-05) e índices en `cupones.Indexes()` + `internal/indexes.All()`; el único creador sigue siendo `scripts/indexes`.
- **Colección `cupones`** (J6 §1): `negocio_id`, `titulo` (≤40), `tipo` (`porcentaje`, `monto`, `dos_por_uno`, `regalo`), `valor`, `condiciones` (≤200), `vigencia_inicio`, `vigencia_fin`, `relampago`, `dias_validos` (0–6, vacío = todos), `horas_validas` {`desde`, `hasta`} en `HH:MM` hora local del negocio, `limite_usos`, `usos_por_usuario` (1 por omisión), `usos_actuales`, `ubicacion`, `categoria`, `estado` (`programado`, `activo`, `pausado`, `agotado`, `vencido`), `vistas`, `guardados`, `version`, `creado_en`, `actualizado_en`. Índices `ix_cupones_cerca`, `ix_cupones_negocio` e `ix_vencimiento` como en el Excel.
- **Colección `cupones_usuario`** (J6 §2): `usuario_id`, `cupon_id`, `negocio_id`, `resumen` {`titulo`, `tipo`, `valor`, `negocio_nombre`, `resumen_normalizado`}, `estado` (`guardado`, `en_canje`, `canjeado`, `vencido`), `vence_en`, `codigo_canje`, `canje_expira_en`, `codigo_canje_usado` (solo para el reintento), `canjeado_en`, `validado_por`, `creado_en`, `actualizado_en`. Índices `ux_cartera`, `ix_cartera_filtros` y `ux_codigo_canje` (único parcial con `estado = en_canje`).
- **Colección `cupones_metricas`** (nueva, para US-097): un documento por cupón y día UTC `{cupon_id, dia, vistas, guardados, canjes}` con `$inc` y upsert; índice `ux_cupon_dia`. Los totales de `cupones` se mantienen igual para la lista del negocio.
- **Valor por tipo:** `porcentaje` 1–100, `monto` en centavos > 0 (constitución: dinero en centavos; el Excel decía pesos), `dos_por_uno` y `regalo` sin `valor` (422 si llega). Relámpago: el servidor fija `vigencia_inicio = ahora` y `vigencia_fin = ahora + 3 h` e ignora las que lleguen.
- **Estado efectivo:** las lecturas filtran por `estado` en (`activo`, `programado`) **y** `vigencia_inicio ≤ ahora < vigencia_fin`, así que la corrección no depende del trabajo. El trabajo corre cada 5 min en cada instancia y es idempotente (sin candado de Redis): `updateMany` de `programado→activo` y de `activo`/`programado`→`vencido`, y `cupones_activos` de los negocios tocados se recalcula con un conteo y `$set` (no `$inc`). La caché de la ficha no se toca (no muestra `cupones_activos`). Vence también las carteras (`guardado`/`en_canje` con `vence_en` pasado → `vencido`).
- **Negocio no visible:** `cerca`, detalle, guardar y canjear responden 404 o lo excluyen si el negocio no está `aprobado` o está eliminado (`cerca` lo confirma con `FindVisibleMany`, sin depender del aviso). `cupones.Observador` (nv-56): `Pausado` pasa `activo`/`programado` a `pausado` con `pausado_por_negocio`; `Reanudado` devuelve esos a `activo`; `DadoDeBaja` los pausa sin bandera (no vuelven). Validar exige que el negocio sea del comerciante, no que esté visible.
- **Editar y borrar (If-Match):** `PATCH` y `DELETE` exigen `If-Match` con `version` (falta = 400, versión vieja = 409, como el resto de la API). Con `usos_actuales > 0` solo aceptan `vigencia_fin`, `limite_usos` (≥ `usos_actuales`) y `estado` (`activo`↔`pausado`); otro campo = 409. `DELETE` con canjes = 409 «páusalo». Los cambios de título o tipo no tocan las carteras ya guardadas (el `resumen` es una copia).
- **Guardar:** consumidor activo; `ux_cartera` da el 409 de duplicado sin leer antes. `costo_puntos` no se acepta en esta spec (los puntos llegan con su propia spec); el 422 por puntos queda fuera. Suma `guardados` en el cupón y en `cupones_metricas`.
- **Canjear (generar código):** solo `guardado` o `en_canje` vigente, en día y hora válidos (409 `motivo`: `vencido`, `fuera_de_horario`, `agotado`, `ya_canjeado`). Si ya está `en_canje` y `canje_expira_en` no pasó, devuelve el mismo código (idempotente); si pasó, genera otro. Código de 6 caracteres de `crypto/rand` sobre un alfabeto sin ambiguos (`23456789ABCDEFGHJKLMNPQRSTUVWXYZ`, 32⁶ ≈ 10⁹); choque con `ux_codigo_canje` = reintento, hasta 3. `qr` es el texto `wherever://canje/{codigo}` que la app dibuja; el servidor no genera imágenes.
- **Validar (atómico):** primero `FindOneAndUpdate` sobre `cupones_usuario` con `codigo_canje`, `estado = en_canje`, `canje_expira_en > ahora` y `negocio_id = {id}` → `canjeado`, `canjeado_en`, `validado_por`, `codigo_canje_usado`, `$unset codigo_canje`; de varios escaneos simultáneos solo uno gana y los demás caen en el reintento. Después, en el cupón, un `updateOne` con pipeline condicional (`usos_actuales < limite_usos` o sin límite, y vigente) que suma el uso y pasa a `agotado` al llegar, en la misma escritura. Si el cupón ya no tenía usos, la cartera vuelve a `en_canje` (compensación; sin transacción porque cada paso toca un solo documento) y el rojo es `agotado`. **Cambio al implementar:** el orden se invirtió (antes cupón y luego cartera) para que un escaneo repetido del mismo código no dé `agotado` por error. Rojo = 409 `{valido: false, motivo}` con `ya_usado`, `vencido`, `otro_negocio`, `fuera_de_horario` o `no_existe`; un código de otro negocio no revela de cuál es.
- **Reintento del negocio:** si el código ya fue canjeado por este mismo usuario del negocio hace menos de 5 min (se conserva `codigo_canje_usado`, índice parcial `ix_codigo_usado`; el trabajo lo quita pasados 5 min), se responde el mismo 200 `{valido: true, cupon, repetido: true}` sin volver a contar. Así un escaneo repetido por red mala no da rojo ni cuenta dos usos, sin gastar comandos de Redis en `idem:{llave}`.
- **Horario:** días y horas en `America/Mexico_City` (`negocios.MexicoCity()`; no hay zona por ciudad); un rango que cruza medianoche (`22:00`–`02:00`) es válido y la madrugada cuenta como el día en que empezó.
- **Búsqueda en cartera:** `q` es prefijo de palabra sobre `resumen_normalizado` (título + nombre del negocio, sin acentos ni mayúsculas), dentro de `usuario_id` (`ix_cartera_filtros` acota; el prefijo filtra en memoria, la cartera es pequeña): con `q` la respuesta es una sola página de hasta 200, sin cursor.
- **Filtros:** `vigentes` = `guardado` o `en_canje` con `vence_en > ahora`; `por_vencer` = vigentes con `vence_en < ahora + 24 h`, con `segundos_restantes`; `usados` = `canjeado`.
- **Vistas:** cuentan solo las aperturas de `GET /cupones/{id}` (una escritura `$inc` por apertura); el carrusel `cerca` no cuenta para no escribir en cada lista.
- **Sugerir título:** `{tipo, valor?, descripcion? (≤200)}` → `{titulo}` con la `ia.Guardia` de nv-26 a nv-28 (`GenerarValidado`, `ia.ConUsuario`; caché de 1 h). Salida validada: 1–40 caracteres y sin correos, teléfonos ni enlaces (`ia.Anonimizar` no la cambia). Sin IA, tope o límite global = 503; límite del usuario (`rate:ia_usuario`, 10/h compartido con planes) = 429 con `Retry-After`.
- **Límites (`Limiter.WindowUser`):** guardar 30/h (`rate:cupones_guardar:{usuario}`), canjear 10/h (`rate:cupones_canjear:{usuario}`), validar 60 por 10 min por usuario del negocio (`rate:cupones_validar:{usuario}`; frena probar códigos al azar), crear 20/día (`rate:cupones_crear:{usuario}`). Las públicas usan la política global. Son 4 llaves nuevas de Redis con TTL; cada petición limitada cuesta el mismo pipeline de 3 comandos que mide `TestPresupuestoRedis_*` y nada más del módulo usa Redis (canje, reintento y trabajo van en MongoDB).
- **ARCO:** `Purger` borra `cupones_usuario` del usuario al eliminar la cuenta; «borrar historial» borra solo las `canjeado` y `vencido`. `validado_por` de otras carteras se quita si el usuario del negocio elimina su cuenta. `Exporter` agrega `cupones` (`resumen`, `estado`, `vence_en`, `canjeado_en`, fechas), nunca `codigo_canje`. Los cupones del negocio se van con su baja (observador), no con la cuenta.
- **Roles:** dueño = `comerciante` + `negocios.propietario_id = sub` (se valida en el service; ajeno = 404). Consumidor con `usuarios.RequireActive` (403 en eliminación pendiente).

## Contratos
Filas de J6 §3; corregidas: `POST /cupones/validar` de J6 §4 queda bajo `/negocios/{id}`, como en J6 §3. Se copian a `docs/endpoints.md` al programar.

| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/negocios/{id}/cupones` | JWT dueño | 201, 400, 401, 403, 404, 422, 429 |
| GET | `/api/v1/negocios/{id}/cupones?estado=&cursor=` | JWT dueño | 200, 401, 403, 404, 422 |
| GET | `/api/v1/negocios/{id}/cupones/{cid}/estadisticas?desde=&hasta=` | JWT dueño | 200, 401, 403, 404, 422 |
| PATCH | `/api/v1/negocios/{id}/cupones/{cid}` | JWT dueño + If-Match | 200, 400, 401, 403, 404, 409, 422 |
| DELETE | `/api/v1/negocios/{id}/cupones/{cid}` | JWT dueño + If-Match | 204, 400, 401, 403, 404, 409 |
| POST | `/api/v1/negocios/{id}/cupones/sugerir-titulo` | JWT dueño | 200, 400, 401, 403, 404, 422, 429, 503 |
| POST | `/api/v1/negocios/{id}/cupones/validar` | JWT dueño | 200, 400, 401, 403, 404, 409, 422, 429 |
| GET | `/api/v1/cupones/cerca?lat=&lng=&radio_km=&categoria=&cursor=` | Público | 200, 422, 429 |
| GET | `/api/v1/cupones/{id}` | Público | 200, 404, 429 |
| POST | `/api/v1/cupones/{id}/guardar` | JWT consumidor | 201, 401, 403, 404, 409, 429 |
| GET | `/api/v1/mis-cupones?filtro=&q=&cursor=` | JWT consumidor | 200, 401, 403, 422 |
| POST | `/api/v1/mis-cupones/{id}/canjear` | JWT consumidor | 200, 401, 403, 404, 409, 429 |
| DELETE | `/api/v1/mis-cupones/{id}` | JWT consumidor | 204, 401, 403, 404 |

`{id}` de `mis-cupones` es el `id` de la cartera (`cupones_usuario`), no el del cupón. 403 = rol distinto o cuenta en eliminación pendiente. `radio_km` 0.1–20 (5 por omisión). Respuesta de validar: 200 `{valido: true, cupon, repetido}` · 409 `{valido: false, motivo}`.

## Tests
- Sin `internal/testutil/cuponestest`: los negocios salen de `reservastest.Negocio` y el service real va en `authtest.Env.Cupones`.
- `tests/integration/cupones_test.go` con MongoDB y Redis reales (más `TestCupones_PropiedadDelNegocio`: el negocio ajeno da 404 en las siete rutas del dueño): crear cada tipo y relámpago, valor inválido por tipo (Unit), editar con y sin canjes, borrar con canjes (409), If-Match (400/409), cerca por distancia y categoría sin negocios no visibles, guardar y duplicado, filtros y prefijo, canjear dos veces (mismo código), fuera de horario, validar verde, rojo por cada motivo, reintento del mismo negocio (`repetido`), límite de usos (`agotado`), estadísticas por día, trabajo de vencimiento y `cupones_activos`, borrar cuenta y exportar.
- `TestCupones_Concurrencia_CanjeUnaVez`: 3 rondas de 10 validaciones simultáneas del mismo código dejan un canje y `usos_actuales` +1.
- `TestCupones_Concurrencia_LimiteUsos`: `limite_usos = 1` y dos códigos distintos validados a la vez: uno verde y uno `agotado`.
- `tests/security/cupones_security_test.go`: 401 sin token, 403 por rol y en eliminación pendiente, 429 en validar (el 404 con negocio ajeno va en Integration).
- `tests/database/indexes_test.go`: índices de `cupones`, `cupones_usuario` y `cupones_metricas`. `tests/contract`: `cupones: []` en la exportación y `cupones/validar_409` (escrito a mano; si el CI difiere, regenerar con `CONTRACT_CAPTURE=1`).
- Unit `internal/cupones/*_test.go`: valor por tipo, ventana de horas que cruza medianoche, días válidos, alfabeto y largo del código, filtros, DTOs.

## Criterios
- Un código no se canjea dos veces ni pasa de `limite_usos`.
- Lo que ve el usuario en «cerca» siempre está activo y vigente aunque el trabajo se atrase.

## Límites
- Sin puntos: `costo_puntos` y la compra con puntos llegan con la spec de recompensas.
- Sin notificación de «cupón por vencer» mientras nv-40 y nv-41 estén pospuestas.
- Sin límites de cupones por plan del negocio (los fija A5 / nv-56).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-14), ola 10; implementada, sin firma.
- **Revisores:** Osmar Reséndiz (sin firma)
- **Fecha:** —
- **Decisiones del revisor (Osmar Reséndiz, 2026-11-14):** se aceptan las propuestas tal como están: `monto` en centavos; reintento idempotente por documento (sin `idem:` en Redis); lecturas por fechas y trabajo cada 5 minutos sin candado; vistas solo en el detalle; `costo_puntos` fuera; nueva colección `cupones_metricas`.
- **Cambios al implementar:** drift de nv-56, nv-18, nv-26 a nv-28 e `If-Match` (ver Contexto); orden de validar invertido (cartera y luego cupón); `usos_por_usuario` fijo en 1 (`ux_cartera`); `q` en una sola página.
- **Detalle:** evidencia en `specs/nv-61/checklists/gates.md`.
