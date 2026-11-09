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
- **Filtro abierto ahora:** los horarios se guardan también como minutos de la semana (`horario_min`), para filtrar dentro de Mongo. Filtrar después de paginar dejaría páginas incompletas.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/lugares` | opcional | 200, 422 |
| GET | `/api/v1/lugares/mapa` | no | 200, 422 |
| GET | `/api/v1/lugares/{id}` | no | 200, 404 |

## Tests
- `Unit/Horario 8`: tramo simple, dos tramos, cruce de medianoche, día sin servicio, borde exacto de apertura y cierre.
- `Integration/Lugares 12` con MongoDB real y 30 negocios: filtros combinados, distancia, orden, cursor, solo aprobados, sin ubicación.
- `Integration/Mapa 4`: agrupación por zoom y tope de pines.
- Rendimiento: lista con filtros y 1,000 negocios sintéticos por debajo de 150 ms (p95).

## Criterios
- Un negocio no aprobado nunca sale por ningún endpoint público.

## Límites
- La distancia es en línea recta, sin rutas.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 1: catálogo y recomendaciones
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-21/checklists/gates.md` + `Memoria.md`.
