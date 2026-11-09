# NV-29 — Modelo de plan, paradas y cálculo de traslados

## Contexto
Un plan es un itinerario de 1 a 6 paradas cuyas horas, traslados y costos se recalculan siempre igual. **Origen**: US-055, US-058 y hoja A4 del Excel. **Depende de**: NV-20, NV-05.

## Requisitos
1. Cada parada tiene hora de llegada, duración, precio estimado y negocio aprobado y abierto a esa hora.
2. **Traslados** entre paradas por movilidad (a pie, bici, auto, transporte): minutos y kilómetros.
3. **Costos:** total y por persona en el encabezado del plan.
4. Control de concurrencia con `version` e `If-Match`, como en el perfil.

## Diseño
- **Traslados:** distancia en línea recta por la fórmula de Haversine multiplicada por 1,3 (aproxima calles) y velocidad por movilidad en configuración (a pie 4,5; bici 12; auto 25; transporte 18 km/h).
- **Recalculo:** una función única `Recalcular(plan)` ordena por hora, calcula traslados y costos y valida que ninguna parada empiece antes de acabar la anterior más el traslado.
- **Validación de paradas:** el negocio debe estar `aprobado` y abierto en la franja; si no, 422 con la parada culpable.
- **Índices:** `dueno_id+estado+fecha`, `estado+fecha`.

## Contratos
Sin endpoints nuevos.

## Tests
- `Unit/Recalcular 12`: orden, traslados por movilidad, costos, parada que no cabe, plan de una parada y de seis.
- `Unit/Haversine 4`: distancias conocidas entre puntos de Tula.
- `Contract/Planes 3`: forma de plan y de error.

## Criterios
- Editar una parada recalcula horas y costos sin dejar huecos imposibles.

## Límites
- Los traslados son estimados, no rutas reales.
- Máximo 6 paradas por plan.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 2: planes con IA (hallazgo C1)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-29/checklists/gates.md` + `Memoria.md`.
