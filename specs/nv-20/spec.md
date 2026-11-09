# NV-20 — Negocios aprobados, categorías y carga semilla

## Contexto
Sin negocios en la base no hay nada que recomendar: esta spec crea el catálogo y carga los 20 a 30 del piloto. **Origen**: US-152, US-027, US-026 y hoja A2 del Excel. **Depende de**: NV-05.

## Requisitos
1. Colección `negocios` con estados `borrador`, `pendiente`, `aprobado` y `suspendido`.
2. **Carga semilla** (US-152): un administrador sube un CSV de 20 a 30 negocios; se valida fila por fila, se reporta cada error con su número de fila y los negocios válidos quedan `aprobado`.
3. **Categorías** `GET /categorias` (catálogo cerrado de 15 gustos del onboarding más etiquetas).

## Diseño
- **Modelo:** `nombre`, `nombre_norm` (sin acentos, minúsculas), `categorias[]`, `precio_nivel` (1–4) y `precio_promedio`, `ubicacion` (GeoJSON Point), `direccion`, `horarios[{dia, abre, cierra}]` (varios tramos por día), `calificacion`, `etiquetas[]`, `acepta_alergias[]`, `accesible`, `mascotas`,…
- **Índices:** `2dsphere` en `ubicacion`; `nombre_norm` ascendente para el prefijo; texto con `default_language: spanish` sobre nombre, etiquetas y descripción; compuesto `estado+categorias+precio_nivel`.
- **Semilla:** el CSV pasa por un validador de columnas fijas (`docs/semilla-negocios.csv` como plantilla).

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/categorias` | no | 200 |
| POST | `/api/v1/admin/negocios/semilla` | administrador | 200, 403, 413, 422 |

## Tests
- `Unit/Semilla 8`: CSV válido, columna faltante, precio fuera de rango, coordenadas fuera de México, fila duplicada, archivo grande.
- `Security/Catálogo 4`: negocio `borrador` no aparece por id ni búsqueda; un consumidor no usa la semilla (403).

## Criterios
- Hay 20 a 30 negocios aprobados cargados desde el CSV.
- Un negocio no aprobado nunca sale por ningún endpoint público.

## Límites
- Sin caché ni `ETag` (nv-17 y nv-18 pospuestas): las respuestas no devuelven 304.
- El CSV con los 20 a 30 negocios reales lo prepara el equipo; sin él, la carga se prueba con datos de ejemplo marcados como tales.
- Calificación inicial fija por el CSV; las reseñas reales no están en este plan.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 1: catálogo y recomendaciones
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-20/checklists/gates.md` + `Memoria.md`.
