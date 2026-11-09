# NV-18 — Caché en Redis

## Contexto
Una caché común reduce consultas a Mongo sin gastar de más la cuota gratuita de Redis. **Origen**: TEC-29, TEC-30, US-001, US-003. **Depende de**: NV-17.

## Requisitos
1. **Caché en Redis** con un ayudante `cache.Obtener(clave, ttl, fn)` que evita avalanchas (una sola consulta por clave a la vez).
2. La compresión ya está en producción (nv-12) y no se toca.

## Diseño
- **Presupuesto de Redis:** cada petición cacheada usa como máximo 2 comandos.
- **TTL por defecto:** configuración 5 min, categorías 1 h, fichas de negocio 5 min.

## Contratos
Sin endpoints nuevos.

## Tests
- `Integration/Caché 6` con Redis real: acierto y fallo, avalancha (50 peticiones, una consulta al origen), invalidación al editar, Redis caído (sigue respondiendo desde el origen).

## Criterios
- El consumo de Redis por caché queda medido.

## Límites
- La caché de Upstash tiene cuota mensual; si se agota, se desactiva la caché antes de que falle la API.

## Aprobación y Control de Cambios
- **Estado:** 🕓 Pospuesta: fuera de la ruta prioritaria hasta la evaluación del 2 de diciembre de 2026 (ver `Memoria.md`)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-18/checklists/gates.md` + `Memoria.md`.
