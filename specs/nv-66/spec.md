# NV-66 — Actividad de lugares: conteo, afluencia y vistos recientemente

## Contexto
Explorar necesita tres piezas que faltan: el botón «Ver 24 resultados» que se actualiza al mover filtros (P-19), las barras de afluencia por hora de la ficha (P-20) y «Vistos recientemente» (US-034). **Origen**: hoja «A2 Lugares y búsqueda» del Excel `Wherever_Endpoints_CRUD_v3.xlsx` (colección «historial_vistas» y cinco filas de endpoints). **Depende de**: NV-04 (`RequireAuth`), NV-05 (`RequireActive`, ARCO con `privacidad.Purger`/`Exporter`), NV-11 (presupuesto de Redis), NV-18 (caché de fichas), NV-20/NV-21 (`internal/negocios`: `ListQuery`, `matchConditions`, `FindVisible`, `afluencia`), NV-22 (`internal/busquedas`, `busquedas_recientes`) y NV-24 (`descartes_usuario`).

## Requisitos
1. `GET /lugares/conteo` cuenta cuántos lugares darían los mismos filtros que `GET /lugares`; público.
2. `GET /lugares/{id}/afluencia?dia=` da las 24 barras de un día y si hay datos suficientes; público.
3. `POST /lugares/{id}/vistas` registra que la persona abrió la ficha; `GET /lugares/recientes` los lista del más reciente al más viejo; `DELETE /lugares/recientes` los borra.
4. Las vistas son historial: se borran solas a los 180 días, con «borrar historial» y con la cuenta, y entran en la exportación.

## Diseño
### Rutas y orden
- Las tres rutas fijas (`/lugares/conteo`, `/lugares/recientes`) se registran antes que `/lugares/:id`, como ya hace `busquedas` con `/lugares/buscar`; si no, `:id` las atiende y responde 404.
- **Paquete:** conteo y afluencia en `internal/negocios` (usan su repositorio y filtros); vistas y recientes en `internal/vistas` (modelo, repositorio, service, handler, DTO y `privacy.go`), que lee lugares con `negocios.Repository.FindVisible` y un nuevo `FindVisibleMany`.

### Conteo
- Mismos filtros que `ListQuery` hoy: `categoria`, `precio_max`, `abierto_ahora`, `lat`/`lng`/`radio_m`. Sin `cursor`, `orden` ni `limite` (400 si llegan). Reutiliza `matchConditions` y el filtro de visibilidad de la lista; la distancia va con `$geoWithin`/`$centerSphere` (`CountDocuments` no admite `$near`).
- Cuenta hasta 1000 (`SetLimit`) con `maxTimeMS` de 2 s: `{ "total": n, "exacto": bool }`; `exacto=false` = «1000+». Plazo vencido o MongoDB caído = 503.
- Sin caché ni política propia: los filtros varían demasiado para reusar una llave y cada política suma 3 comandos de Redis a cada petición. La app espera 400 ms tras el último cambio de filtro antes de pedir (anotado para el equipo de la app). Queda la política global.
- **Drift con el Excel:** los filtros de calificación, accesibilidad, dieta, «Poco lleno ahora» y «Con reserva» aún no existen en `GET /lugares`; el conteo los gana cuando la lista los gane, con la misma función.

### Afluencia
- Lee `negocios.afluencia` (matriz 7 × 24, 0–100) de un lugar visible (`FindVisible`; si no, 404). `dia` 0–6 con 0 = domingo (como `time.Weekday`); por omisión, hoy en `America/Mexico_City`. Otro valor = 422.
- Respuesta `{ "dia": d, "horas": [24 enteros], "hora_actual": h | null, "suficiente": bool }`. `hora_actual` solo si `dia` es hoy. `suficiente=false` si la matriz falta, no es 7 × 24 o el día está en ceros; entonces `horas` va en ceros y la app muestra «Aún juntamos datos de este lugar».
- Pasa por la caché de fichas de nv-18 si el lugar ya está ahí; no agrega llaves nuevas.
- **Drift con el Excel:** quién llena `afluencia` (check-ins J7 y «Horas y cupo») no existe todavía; hoy casi todos los lugares darán `suficiente=false`. Esta spec solo la lee.

### Vistas y recientes
- **Colección `historial_vistas`** (Excel A2 §3, ajustada): `usuario_id`, `negocio_id`, `origen` (`busqueda`, `mapa`, `inicio`, `plan_ia`, `cupon`; opcional), `fecha` y `expira_en` (`fecha` + 180 días). Índices `ux_vista` `{usuario_id: 1, negocio_id: 1}` único, `ix_recientes` `{usuario_id: 1, fecha: -1}` y `ttl_vistas` `{expira_en: 1}` con `expireAfterSeconds: 0`, en `scripts/indexes` y `/ready`.
- **Drift con el Excel:** el Excel mezcla en `historial_vistas` vistas, «No me interesa» y búsquedas (`tipo`, `termino`, `ux_busqueda`). El código ya separó búsquedas en `busquedas_recientes` (nv-22) y descartes en `descartes_usuario` (nv-24), así que aquí solo hay vistas: sin `tipo` ni `termino`, y `ux_vista` sin `tipo`.
- **Registrar:** JWT de cualquier rol más `RequireActive` (como `/busquedas/recientes`); el Excel dice «Consumidor», pero un comerciante también explora. Lugar no visible = 404. `upsert` por `(usuario_id, negocio_id)` con `$set fecha, expira_en, origen`. Si la vista anterior tiene menos de 10 min, no se escribe nada (204 igual): abrir y cerrar la ficha no gasta escrituras. Responde 204.
- **Límite contra abuso, sin Redis:** 60 lugares distintos por hora por usuario, contados en MongoDB con `ix_recientes` (`fecha` en la última hora), como el chat de nv-57; al pasarlo, 429 uniforme con `Retry-After: 3600`. Una vista ya cuesta unos 6 comandos de Redis (3 de la política global y 3 de `Writes`); una política propia sumaría 3 más por cada ficha abierta, el gesto más frecuente de la app. Con 30 fichas al día por persona son ~5 400 comandos al mes por persona activa: se anota en la estimación de nv-11 al ejecutar y se fija con `TestPresupuestoRedis_VistasNV66`.
- **Recientes:** `limite` 10 por omisión, máximo 50, sin cursor. Lee `ix_recientes`, trae las tarjetas con `FindVisibleMany` (una consulta `$in`) en el mismo formato `PlaceItem` de la lista más `visto_en`, y omite los lugares que dejaron de ser visibles.
- **Borrar:** `DELETE /lugares/recientes` borra todas las vistas de la persona (no toca búsquedas ni descartes); 204 aunque no haya nada.
- **Estadísticas del negocio:** el Excel dice que las vistas las alimentan; quedan fuera (nv-56 difiere `estadisticas_diarias`). Cuando existan, se agregan de forma anónima antes del TTL y del borrado ARCO, nunca leyendo `historial_vistas` con `usuario_id` desde el panel del negocio.

### Privacidad (ARCO)
- `vistas.Privacy` se registra como `Purger` y `Exporter`. `PurgeHistory` y `PurgeAccount` borran todas las vistas del usuario; `Export` devuelve `negocio_id`, nombre del lugar, `origen` y `fecha`, de la más nueva a la más vieja.
- Ningún log lleva `usuario_id` junto a `negocio_id` de una vista; el conteo no registra coordenadas (como nv-25).

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/lugares/conteo?categoria=&precio_max=&abierto_ahora=&lat=&lng=&radio_m=` | Público | 200, 400, 422, 429, 503 |
| GET | `/api/v1/lugares/{id}/afluencia?dia=` | Público | 200, 404, 422, 429 |
| POST | `/api/v1/lugares/{id}/vistas` | JWT | 204, 400, 401, 403, 404, 422, 429 |
| GET | `/api/v1/lugares/recientes?limite=` | JWT | 200, 401, 403, 422, 429 |
| DELETE | `/api/v1/lugares/recientes` | JWT | 204, 401, 403, 429 |

Filas copiadas del Excel (A2): `GET /lugares/conteo` «Cuenta cuántos resultados darían los filtros… P-19» (Mismos filtros que /lugares; 200 { total }; Público); `GET /lugares/{id}/afluencia?dia=` «Barras de afluencia por hora de un día…» (Path: id · Query: dia (0-6); 200 { horas: [24 valores], hora_actual, suficiente: bool }; Público); `POST /lugares/{id}/vistas` «Registra que el usuario abrió la ficha…» (Body: origen; 204; Consumidor); `GET /lugares/recientes` «"Vistos recientemente" (US-034)» (Query: limite (10); 200; Consumidor); `DELETE /lugares/recientes` «Limpia el historial de vistos» (204; Consumidor). Se copian a `docs/endpoints.md` al ejecutar. 403 = cuenta en eliminación pendiente; la respuesta del conteo agrega `exacto` y la de afluencia `dia`.

## Tests
- `tests/integration/lugares_conteo_test.go`: el conteo coincide con lo que da la lista con los mismos filtros (categoría, precio, abierto, radio), tope en 1000 con `exacto=false`, `cursor` = 400.
- `tests/integration/lugares_afluencia_test.go`: día con datos, día en ceros y matriz ausente = `suficiente=false`, `hora_actual` solo hoy, `dia=7` = 422, lugar no visible = 404.
- `tests/integration/vistas_test.go`: upsert sin duplicar, segunda vista antes de 10 min no escribe, orden y `limite`, lugar oculto omitido, borrar, 429 en el 61.º lugar distinto y la ruta `/lugares/recientes` no cae en `/lugares/:id`.
- `tests/integration/privacidad_vistas_test.go`: borrar historial y borrar cuenta dejan `historial_vistas` vacía para la persona; la exportación trae sus vistas.
- `tests/security/vistas_security_test.go`: 401 sin token, 403 con cuenta en eliminación, nadie ve las vistas de otro, ningún log con `usuario_id` y `negocio_id` de una vista.
- `tests/integration/presupuesto_redis_test.go` (`TestPresupuestoRedis_VistasNV66`): una vista cuesta lo mismo que cualquier escritura sin política propia; el conteo, lo mismo que una lectura pública.
- `tests/database/indexes_test.go`: `ux_vista` único, `ix_recientes` y TTL de `historial_vistas`. `tests/contract/lugares_actividad_contract_test.go`: 200 del conteo, de la afluencia y de recientes; 204 de vista y borrado.
- Unit: validación de `dia` y filtros del conteo, `suficiente`, ventana de 10 min (`internal/negocios/afluencia_test.go`, `internal/vistas/service_test.go`).

## Criterios
- El conteo y la lista nunca discrepan con los mismos filtros.
- Las vistas de una persona desaparecen con «borrar historial», con su cuenta y a los 180 días, y aparecen en su exportación.
- Abrir fichas no agrega comandos de Redis más allá de los de cualquier escritura.

## Límites
- Sin estadísticas del negocio ni alimentar recomendaciones con vistas.
- Sin escribir `afluencia` (check-ins J7).
- Sin filtros nuevos en `GET /lugares`.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador (2026-11-13), ola por decidir
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** `historial_vistas` solo con vistas (búsquedas y descartes ya viven aparte); vistas para cualquier rol activo; límite de vistas contado en MongoDB sin Redis (60/h) y sin escritura antes de 10 min; conteo sin caché ni política propia con tope 1000; `dia` 0 = domingo; estadísticas del negocio fuera y solo anónimas.
- **Detalle:** evidencia en `specs/nv-66/checklists/gates.md` + `Memoria.md` al ejecutar.
