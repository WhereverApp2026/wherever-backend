# NV-21 — Lista, ficha y mapa de lugares

## Contexto
La app necesita explorar negocios cercanos con filtros, ver su ficha y verlos en un mapa. **Origen**: US-152, US-027, US-026 y hoja A2 del Excel. **Depende de**: NV-20.

## Requisitos
1. **Lista** `GET /lugares` con filtros combinables (categoría, precio máximo, abierto ahora, cerca de un punto con radio) y orden por distancia, calificación o precio.
2. **Mapa** `GET /lugares/mapa`: pines del área visible, agrupados por celda cuando hay muchos.
3. **Ficha** `GET /lugares/{id}` con datos, horarios y el campo calculado `abierto_ahora` según la hora de México.

## Diseño
- **Distancia:** `$geoNear`; el radio máximo es 20 km.
- **Abierto ahora:** se calcula en la API con la zona `America/Mexico_City`; soporta tramos que cruzan la medianoche.
- **Mapa:** se agrupa en una rejilla cuyo tamaño depende del zoom; devuelve conteo y centro por celda.
- **Privacidad:** la ubicación del usuario se usa en la consulta y no se guarda; no entra en logs.
- **Paginación:** se reutiliza `internal/shared/pagination`. Ordenado por distancia, el cursor guarda la distancia y el id del último resultado y la siguiente página se pide con `minDistance`, porque `$geoNear` no se reanuda por `_id`.
- **Filtro abierto ahora:** se reutiliza `horario_min[]` de NV-20: ventanas `{dia, desde, hasta}` en minutos desde las 00:00 del día (`dia` 0 = domingo), donde `hasta` pasa de 1440 cuando el tramo cruza la medianoche. Con la hora de `America/Mexico_City` (día `d`, minuto `m`) un negocio está abierto si tiene una ventana `{dia: d, desde ≤ m < hasta}` o la de ayer cruza hasta hoy `{dia: d−1, hasta > m + 1440}`. Apertura incluida y cierre excluido. El filtro va dentro de Mongo (`$elemMatch`), porque filtrar después de paginar dejaría páginas incompletas; la ficha usa la misma regla en Go.
- **Valores fijados al implementar (2026-11-10):** radio por defecto 5 km (`radio_m` máx. 20,000); sin `orden`, por distancia si hay `lat` y `lng` y por calificación si no; `orden=precio` usa `precio_promedio`; el cursor guarda `orden:valor` y un cursor de otro orden es 422. Mapa: hasta 100 pines; con más, celdas de `360/2^zoom/4` grados (4×4 por mosaico de 256 px), máximo 400, centro = promedio de sus negocios; `zoom` 3–20 y área de menos de 180° de ancho. La ficha oculta dueño, QR de check-in, plan, revisión, versión y `horario_min`. «Auth opcional»: sin token responde; un token inválido es 401, como en todo el API.
- **Código:** `internal/negocios/horario.go` (regla y filtro), `lugares_dto.go`, `lugares_repository.go`, `lugares_service.go` y las rutas en `handler.go`.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/lugares` | opcional | 200, 401, 422 |
| GET | `/api/v1/lugares/mapa` | no | 200, 422 |
| GET | `/api/v1/lugares/{id}` | no | 200, 404 |

## Tests
- `Unit/Horario 8` (`internal/negocios/horario_test.go`, `TestHorario_OpenAt`): tramo simple, dos tramos, cruce de medianoche, día sin servicio, borde exacto de apertura y cierre; más zona de México contra UTC, filtro de Mongo, cursor y celdas.
- `Integration/Lugares` (`tests/integration/lugares_test.go`, 11 `TestLugares_*` y 9 casos de 422) con MongoDB real y 30 negocios: filtros combinados, distancia, radio, orden, cursor con empates, solo aprobados, sin ubicación, cruce de medianoche, ficha, auth opcional y ubicación fuera de logs.
- `Integration/Mapa 4` (`TestMapa_*`): pines, agrupación por zoom, tope de pines con categoría y área inválida. Security: `TestLugaresSecurity_HiddenNeverPublic`; Contract: `negocios/lugares_*`.
- Rendimiento (`TestLugares_PerformanceP95`): lista con filtros y 1,000 negocios sintéticos por debajo de 150 ms (p95).

## Criterios
- Un negocio no aprobado nunca sale por ningún endpoint público.

## Límites
- La distancia es en línea recta, sin rutas.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Implementada y probada (2026-11-10), cinco suites verdes; falta la firma y Semgrep en CI. Ruta prioritaria, ola 1: catálogo y recomendaciones
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** 2026-11-10
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-21/checklists/gates.md` + `Memoria.md`.
