# NV-61 — Cupones: creación, cartera y canje con código de un solo uso

## Contexto
El negocio crea cupones (porcentaje, monto, 2x1 o regalo) con vigencia, días y horas válidos y límite de usos, y puede lanzar uno relámpago de 3 horas. El usuario los ve cerca, los guarda en su cartera y al canjear muestra un código de un solo uso que vive 5 minutos; el negocio lo escanea o teclea y el servidor responde válido o no. **Origen**: hoja «J6 Cupones» del Excel (roles C5 y C13; pantallas P-29, P-30, P-49 y el carrusel «Cupones cerca» de P-14), US-097. **Depende de**: NV-20 (`negocios`, `cupones_activos`), NV-17 (cursor), NV-18 (ficha en caché), NV-26 a NV-28 (IA con tope y límite por usuario), NV-56 (panel del negocio: dónde vive P-49 y la pausa del negocio).

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
- **Estado efectivo:** las lecturas filtran por `estado = activo` **y** `vigencia_inicio ≤ ahora < vigencia_fin`, así que la corrección no depende del trabajo. El trabajo corre cada 5 min y es idempotente (dos instancias a la vez no rompen nada, sin candado de Redis): `updateMany` de `programado→activo` y de `activo→vencido`, y `cupones_activos` del negocio se recalcula con un conteo y `$set` (no `$inc`), además de borrar `cache:negocio:{id}` de nv-18 si cambió. Vence también las carteras (`guardado`/`en_canje` con `vence_en` pasado → `vencido`).
- **Negocio no visible:** `cerca`, detalle, guardar y validar responden 404 o lo excluyen si el negocio no está `aprobado` o está eliminado. Pausar o eliminar el negocio (nv-56) pasa sus cupones a `pausado` con un gancho en el service de negocios.
- **Editar y borrar (If-Match):** `PATCH` y `DELETE` exigen `If-Match` con `version` (412 si no coincide, 428 si falta). Con `usos_actuales > 0` solo aceptan `vigencia_fin`, `limite_usos` (≥ `usos_actuales`) y `estado` (`activo`↔`pausado`); otro campo = 409. `DELETE` con canjes = 409 «páusalo». Los cambios de título o tipo no tocan las carteras ya guardadas (el `resumen` es una copia).
- **Guardar:** consumidor activo; `ux_cartera` da el 409 de duplicado sin leer antes. `costo_puntos` no se acepta en esta spec (los puntos llegan con su propia spec); el 422 por puntos queda fuera. Suma `guardados` en el cupón y en `cupones_metricas`.
- **Canjear (generar código):** solo `guardado` o `en_canje` vigente, en día y hora válidos (409 `motivo`: `vencido`, `fuera_de_horario`, `agotado`, `ya_canjeado`). Si ya está `en_canje` y `canje_expira_en` no pasó, devuelve el mismo código (idempotente); si pasó, genera otro. Código de 6 caracteres de `crypto/rand` sobre un alfabeto sin ambiguos (`23456789ABCDEFGHJKLMNPQRSTUVWXYZ`, 32⁶ ≈ 10⁹); choque con `ux_codigo_canje` = reintento, hasta 3. `qr` es el texto `wherever://canje/{codigo}` que la app dibuja; el servidor no genera imágenes.
- **Validar (atómico):** `FindOneAndUpdate` sobre `cupones_usuario` con `codigo_canje`, `estado = en_canje`, `canje_expira_en > ahora` y `negocio_id = {id}` → `canjeado`, `canjeado_en`, `validado_por`, `$unset codigo_canje`. Antes, en el cupón, `updateOne` condicional `usos_actuales < limite_usos` (o sin límite) con `$inc`; si llega al límite pasa a `agotado`. Si el segundo paso no encuentra el código, se revierte el `$inc` (compensación; sin transacción porque la cartera y el cupón cambian en un solo documento cada uno). Rojo = 409 `{valido: false, motivo}` con `ya_usado`, `vencido`, `otro_negocio`, `fuera_de_horario` o `no_existe`; un código de otro negocio no revela de cuál es.
- **Reintento del negocio:** si el código ya fue canjeado por este mismo negocio hace menos de 5 min (se conserva `codigo_canje_usado` con TTL lógico de 5 min en el documento), se responde el mismo 200 `{valido: true, cupon, repetido: true}` sin volver a contar. Así un escaneo repetido por red mala no da rojo ni cuenta dos usos, sin gastar comandos de Redis en `idem:{llave}`.
- **Horario:** días y horas en la zona del negocio (la de su ciudad, nv-20); un rango que cruza medianoche (`22:00`–`02:00`) es válido.
- **Búsqueda en cartera:** `q` es prefijo sobre `resumen_normalizado` (título + nombre del negocio, sin acentos ni mayúsculas), dentro de `usuario_id` (`ix_cartera_filtros` acota; el prefijo filtra en memoria, la cartera es pequeña, máximo 200 por página).
- **Filtros:** `vigentes` = `guardado` con `vence_en > ahora`; `por_vencer` = vigentes con `vence_en < ahora + 24 h`, con `segundos_restantes`; `usados` = `canjeado`.
- **Vistas:** cuentan solo las aperturas de `GET /cupones/{id}` (una escritura `$inc` por apertura); el carrusel `cerca` no cuenta para no escribir en cada lista.
- **Sugerir título:** cliente de nv-26 con el tope de nv-27 y el límite por usuario de nv-28; salida validada (≤40 caracteres, sin datos personales). Si la IA no está disponible, 503.
- **Límites (`Limiter.WindowUser`):** guardar 30/h (`rate:cupones_guardar:{usuario}`), canjear 10/h (`rate:cupones_canjear:{usuario}`), validar 60 por 10 min por usuario del negocio (`rate:cupones_validar:{usuario}`; frena probar códigos al azar), crear 20/día (`rate:cupones_crear:{usuario}`). Las públicas usan la política global. Son 4 llaves nuevas de Redis con TTL; el costo por petición limitada sigue `TestPresupuestoRedis_*`.
- **ARCO:** `Purger` borra `cupones_usuario` del usuario al eliminar la cuenta; «borrar historial» borra solo las `canjeado` y `vencido`. `validado_por` de otras carteras se pone en `null` si el usuario del negocio elimina su cuenta. `Exporter` agrega `cupones` (`resumen`, `estado`, `vence_en`, `canjeado_en`, fechas), nunca `codigo_canje`.
- **Roles:** dueño = `comerciante` + `negocios.propietario_id = sub` (se valida en el service; ajeno = 404). Consumidor con `usuarios.RequireActive` (403 en eliminación pendiente).

## Contratos
Filas de J6 §3; corregidas: `POST /cupones/validar` de J6 §4 queda bajo `/negocios/{id}`, como en J6 §3. Se copian a `docs/endpoints.md` al programar.

| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| POST | `/api/v1/negocios/{id}/cupones` | JWT dueño | 201, 400, 401, 403, 404, 422, 429 |
| GET | `/api/v1/negocios/{id}/cupones?estado=&cursor=` | JWT dueño | 200, 401, 403, 404, 422 |
| GET | `/api/v1/negocios/{id}/cupones/{cid}/estadisticas?desde=&hasta=` | JWT dueño | 200, 401, 403, 404, 422 |
| PATCH | `/api/v1/negocios/{id}/cupones/{cid}` | JWT dueño + If-Match | 200, 400, 401, 403, 404, 409, 412, 422, 428 |
| DELETE | `/api/v1/negocios/{id}/cupones/{cid}` | JWT dueño + If-Match | 204, 401, 403, 404, 409, 412, 428 |
| POST | `/api/v1/negocios/{id}/cupones/sugerir-titulo` | JWT dueño | 200, 400, 401, 403, 404, 422, 429, 503 |
| POST | `/api/v1/negocios/{id}/cupones/validar` | JWT dueño | 200, 400, 401, 403, 404, 409, 422, 429 |
| GET | `/api/v1/cupones/cerca?lat=&lng=&radio_km=&categoria=&cursor=` | Público | 200, 422, 429 |
| GET | `/api/v1/cupones/{id}` | Público | 200, 404, 429 |
| POST | `/api/v1/cupones/{id}/guardar` | JWT consumidor | 201, 401, 403, 404, 409, 429 |
| GET | `/api/v1/mis-cupones?filtro=&q=&cursor=` | JWT consumidor | 200, 401, 403, 422 |
| POST | `/api/v1/mis-cupones/{id}/canjear` | JWT consumidor | 200, 401, 403, 404, 409, 429 |
| DELETE | `/api/v1/mis-cupones/{id}` | JWT consumidor | 204, 401, 403, 404 |

403 = rol distinto o cuenta en eliminación pendiente. `radio_km` 0.1–20 (5 por omisión). Respuesta de validar: 200 `{valido: true, cupon, repetido}` · 409 `{valido: false, motivo}`.

## Tests
- `tests/integration/cupones_test.go` con MongoDB y Redis reales: crear cada tipo y relámpago, valor inválido por tipo (Unit), editar con y sin canjes, borrar con canjes (409), If-Match (412/428), cerca por distancia y categoría sin negocios no visibles, guardar y duplicado, filtros y prefijo, canjear dos veces (mismo código), fuera de horario, validar verde, rojo por cada motivo, reintento del mismo negocio (`repetido`), límite de usos (`agotado`), estadísticas por día, trabajo de vencimiento y `cupones_activos`, borrar cuenta y exportar.
- `TestCupones_Concurrencia_CanjeUnaVez`: 3 rondas de 10 validaciones simultáneas del mismo código dejan un canje y `usos_actuales` +1.
- `TestCupones_Concurrencia_LimiteUsos`: `limite_usos = 1` y dos códigos distintos validados a la vez: uno verde y uno `agotado`.
- `tests/security/cupones_security_test.go`: 401 sin token, 403 por rol y en eliminación pendiente, 404 con negocio ajeno, 429 en validar.
- `tests/database/indexes_test.go`: índices de `cupones`, `cupones_usuario` y `cupones_metricas`. `tests/contract`: `cupones: []` en la exportación y forma de validar.
- Unit `internal/cupones/*_test.go`: valor por tipo, ventana de horas que cruza medianoche, días válidos, alfabeto y largo del código, filtros, DTOs.

## Criterios
- Un código no se canjea dos veces ni pasa de `limite_usos`.
- Lo que ve el usuario en «cerca» siempre está activo y vigente aunque el trabajo se atrase.

## Límites
- Sin puntos: `costo_puntos` y la compra con puntos llegan con la spec de recompensas.
- Sin notificación de «cupón por vencer» mientras nv-40 y nv-41 estén pospuestas.
- Sin límites de cupones por plan del negocio (los fija A5 / nv-56).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-12), ola por decidir
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** `monto` en centavos; reintento idempotente por documento (sin `idem:` en Redis); lecturas por fechas y trabajo sin candado; vistas solo en el detalle; `costo_puntos` fuera; nueva colección `cupones_metricas`.
- **Detalle:** evidencia en `specs/nv-61/checklists/gates.md`.
