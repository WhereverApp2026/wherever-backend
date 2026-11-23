# NV-85 — Filtros y tarjetas de Explorar

## Contexto
Explorar en la maqueta (`30_Explorar`, `31_Filtros`, `32_ExpMapa`, `34_Sello`) filtra por **«De amigos»**, **«Con Sello de Excelencia»**, **«Con cupón»**, varios **tipos de lugar** a la vez, distancia y presupuesto, y cuenta el resultado («Ver 24 lugares»). Cada tarjeta muestra el Sello, la **afluencia ahora** («Poco concurrido», «Moderado», «Hora pico»), el cupón («Cupón $50») y las iniciales de amigos que fueron. `GET /lugares` hoy filtra solo por una categoría, precio máximo, abierto y distancia, y no trae esos datos. **Origen**: revisión de la maqueta (2026-11-21). **Depende de**: NV-21 (lugares), NV-56 (Sello), NV-61 (cupones), NV-62 (check-ins), NV-66 (afluencia), NV-82 (privacidad).

## Requisitos
1. Filtrar por Sello, cupón vigente, lugares donde fueron mis amigos y varias categorías.
2. Contar el resultado con los mismos filtros.
3. Mostrar en cada tarjeta Sello, afluencia actual, el mejor cupón y amigos que fueron.

## Diseño
- **Parámetros nuevos en `GET /lugares`, `GET /lugares/conteo` y `GET /lugares/mapa`:** `con_sello` (bool), `con_cupon` (bool: cupón publicado y vigente ahora, nv-61), `de_amigos` (bool, requiere sesión: negocios con check-in válido de un amigo aceptado en los últimos 180 días que no ocultó su actividad, nv-82), `categorias` (lista separada por comas, hasta 6; `categoria` sigue funcionando), `precio` (lista de niveles 1–3 en vez de solo máximo). Sin sesión, `de_amigos` = 401.
- **Tarjeta (`items[]`) agrega:** `sello_excelencia`, `afluencia_ahora` (`poco`, `moderado`, `pico` o `null`, de la curva de nv-66 en la hora actual), `cupon{id, titulo}` (el de mayor descuento vigente) y, con sesión, `amigos_fueron{total, iniciales[≤3]}` (solo iniciales y ids, sin nombres completos).
- **Índices:** `ix_negocios_sello` parcial (`sello_excelencia: true`) y uso de `ix_cupones_vigentes` existente; `de_amigos` arma la lista de negocios con un `distinct` sobre `checkins` por los ids de amigos (≤ 500) con `ix_mis_checkins`.
- **«Con Sello cerca de ti» (`34_Sello`)** = `GET /lugares?con_sello=true&lat=&lng=`.

## Contratos
| Método | Ruta | Cambio |
|---|---|---|
| GET | `/api/v1/lugares` | `con_sello`, `con_cupon`, `de_amigos`, `categorias`, `precio`; tarjeta con `sello_excelencia`, `afluencia_ahora`, `cupon`, `amigos_fueron` |
| GET | `/api/v1/lugares/conteo` | los mismos filtros |
| GET | `/api/v1/lugares/mapa` | los mismos filtros |

## Tests
- Integration: cada filtro y sus combinaciones; conteo igual al listado; afluencia por hora; mejor cupón; amigos que fueron sin quien ocultó su actividad.
- Security: `de_amigos` sin sesión = 401; nunca nombres completos de amigos ni de personas que no son mis amigos; inyección en `categorias` = 422.
- Database: `ix_negocios_sello`. Contract: tarjeta nueva.

## Criterios
- «Ver N lugares» coincide con lo que se lista con los mismos filtros.
- Las iniciales de amigos solo salen de amistades aceptadas que comparten su actividad.

## Límites
- Sin filtros por servicios (terraza, pet friendly) en esta spec; sin orden por popularidad entre amigos.

## Drift al ejecutar (2026-11-22)
- **Código:** `internal/negocios/explorar.go` (filtros `ExploreFilters` embebidos en `ListQuery`, `CountQuery` y `MapQuery`; tarjeta `ExploreItem`; `CrowdLevel`, `BestCoupon`, `Initials`). Adaptadores: `amistades.Repository.FriendIDs`, `resenas.Repository.VisitedPlaces`/`VisitsAt` (`internal/resenas/explorar.go`) y `cupones.Repository.OffersFor` (`internal/cupones/explorar.go`), conectados con `negocios.Service.WithExplore` en `cmd/api/main.go` y `authtest`.
- **Tarjeta en un tipo propio:** `ExploreItem` (solo `GET /lugares`) envuelve a `PlaceItem`; búsquedas, recientes, grupos y planes siguen con la tarjeta de antes y sus contratos no cambian.
- **`con_cupon`:** filtra por `negocios.cupones_activos > 0` (lo recalcula cupones al cambiar un cupón y su trabajo cada 5 min), no por `ix_cupones_vigentes`, que no existe. Así lista, conteo y mapa filtran en la misma consulta de `negocios`. El cupón de la tarjeta sí se lee vigente al momento con `ix_cupones_negocio` (`estado` + fechas, como nv-61): en el hueco de hasta 5 min un lugar puede salir con `con_cupon` y `cupon: null`.
- **Mejor cupón:** «mayor descuento» = mayor ahorro estimado con `precio_promedio` del lugar: porcentaje × promedio, monto tal cual, 2×1 = un promedio, regalo = 0 (sale si es el único). Empate: el que vence antes y luego el id menor.
- **`precio`:** niveles 1–4 (como `nivel_precio` y `precio_max`), no 1–3; `precio_max` sigue y se combina con `precio`.
- **Afluencia:** tres niveles fijos sobre la curva 0–100 de nv-66: 0–33 `poco`, 34–66 `moderado`, 67–100 `pico`; sin curva o con el día en ceros = `null`.
- **Amigos:** amistades aceptadas (hasta 500, `MaxList`) activas y sin `actividad: nadie`. Con `de_amigos` la cuenta debe estar activa (en eliminación pendiente = 403, como `RequireActive`); quien consulta va en la misma lectura de cuentas, sin consulta extra. `amigos_fueron` sale solo con sesión: `{total, iniciales[≤ 3]{id, iniciales}}`, del check-in más reciente al más viejo; iniciales = primer y último nombre.
- **Sello:** usa `sello_excelencia` tal como existe hoy (nv-56/nv-76). La regla y la vigencia del Sello son de nv-80 (pendiente del revisor); no se inventó ninguna.
- **Índice nuevo:** `ix_negocios_sello` = `{estado: 1, calificacion.promedio: -1}` parcial con `sello_excelencia: true`. Hay que crearlo en Atlas (`go run ./scripts/indexes`) antes del despliegue.
- **Cursor:** con `de_amigos` el ámbito firmado incluye a quien consulta (otro = 422).
- **Pruebas:** Unit `internal/negocios/explorar_test.go`; Integration `tests/integration/explorar_test.go`; Security `tests/security/explorar_security_test.go`; Contract `tests/contract/explorar_contract_test.go` (`negocios/lugares_tarjeta_200`) y `negocios/lugares_200` con los campos nuevos; Database `ix_negocios_sello` en `tests/database/indexes_test.go`. Sin Redis nuevo.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-22), ejecutada; ver `checklists/gates.md`.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** (1) «de amigos» = check-in de un amigo en 180 días; (2) solo iniciales de amigos en la tarjeta; (3) afluencia en tres niveles con la curva existente.
- **Detalle:** `specs/nv-85/checklists/gates.md` al ejecutar.
