# NV-29 — Modelo de plan, paradas y cálculo de traslados

## Contexto
Un plan es un itinerario de 1 a 6 paradas cuyas horas, traslados y costos se recalculan siempre igual. **Origen**: US-055, US-058 y hoja A4 del Excel. **Depende de**: NV-20, NV-05.

## Requisitos
1. Cada parada tiene hora de llegada, duración, precio estimado y negocio aprobado y abierto a esa hora.
2. **Traslados** entre paradas por movilidad (`a_pie`, `bicicleta`, `auto`, `transporte_publico`, el catálogo de nv-05): minutos y kilómetros.
3. **Costos:** total y por persona en el encabezado del plan. `precio` de la parada es por persona: `costo_por_persona` = suma de precios y `costo_total` = eso × `personas`.
4. Control de concurrencia con `version` e `If-Match`, como en el perfil: `Repository.UpdateIfVersion` filtra por dueño, estado activo y versión; versión vieja = `ErrVersionConflict` (409 en nv-30, decisión de nv-05) y plan ajeno = `ErrNotFound`.

## Diseño
- **Traslados:** distancia en línea recta por la fórmula de Haversine multiplicada por 1,3 (aproxima calles) y velocidad por movilidad (a pie 4,5; bici 12; auto 25; transporte 18 km/h) en `planes.DefaultVelocidades()`, que llega por constructor a `NewCalculadora`. Sin variable de entorno hasta que un consumidor necesite ajustarla. Kilómetros a 2 decimales y minutos hacia arriba.
- **Modelo `planes` (paquete `internal/planes`):** `dueno_id`, `nombre`, `fecha`, `personas`, `presupuesto`, `movilidad`, `paradas[{id, negocio_id, llega, minutos, precio, nota}]`, `traslados[{desde, hasta, minutos, km}]`, `costo_total`, `costo_por_persona`, `estado` (`activo`, `borrado`), `origen` (`manual`, `ia`, `reglas`), `version`, `creado_en`, `actualizado_en`. El JSON no lleva `dueno_id` ni `estado`.
- **Recalculo:** una función única `Recalcular(plan, lugares, velocidades)` ordena por hora, calcula traslados y costos y valida que ninguna parada empiece antes de acabar la anterior más el traslado. Es pura; `Calculadora.Aplicar(ctx, plan)` carga los negocios con `negocios.Repository.FindVisibleMany` y la llama.
- **Validación de paradas:** el negocio debe estar `aprobado`, no eliminado y abierto (`negocios.OpenAt`) al llegar y en el último minuto de la estancia; si no, `*StopError{Index, StopID, Reason}` (`negocio_no_disponible`, `cerrado`, `no_cabe`, `duracion`, `precio`), que nv-30 devuelve como 422 con la parada culpable.
- **ARCO:** `planes.Privacy` se registra como `Purger` (antes de `usuarios`) y `Exporter`. «Borrar mi historial» no borra planes: son contenido que la persona armó, no historial de uso.
- **Índices:** `ix_dueno_estado_fecha`, `ix_estado_fecha` en `planes.Indexes()`, registrados en `internal/indexes`.

## Contratos
Sin endpoints nuevos.

## Tests
- `Unit/Recalcular` (`internal/planes/recalculo_test.go`, 12): orden, traslados por movilidad, costos, parada que no cabe, traslado que cabe justo, una parada, seis, 0 y 7 paradas, negocio no disponible, cerrado en la franja, encabezado inválido, duración y precio.
- `Unit/Haversine` (4 aserciones en una prueba): Zócalo–Atlantes, Zócalo–Catedral, mismo punto y simetría.
- `Unit/Plan_FormaJSON`: forma JSON del plan, paradas y traslados en `snake_case`, sin `dueno_id` ni `estado`. Sustituye a `Contract/Planes 3`: sin endpoints no hay contrato HTTP que capturar; el contrato llega con nv-30.
- `Integration/Planes` (`tests/integration/planes_test.go`, 4): la calculadora rechaza un negocio en borrador, solo el dueño lee su plan, dos ediciones con la misma versión (gana una) y ARCO exporta y borra solo lo del dueño.
- `Database/indexes`: los 2 índices de `planes`.

## Criterios
- Editar una parada recalcula horas y costos sin dejar huecos imposibles.

## Límites
- Los traslados son estimados, no rutas reales.
- Máximo 6 paradas por plan.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Implementada (2026-11-11) en la rama `nv-29`; falta la firma. Ruta prioritaria, ola 2: planes con IA (hallazgo C1)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** confirmar `precio` por persona, velocidades en código (sin variable de entorno) y que «Borrar mi historial» no borre planes.
- **Detalle:** la evidencia irá en `specs/nv-29/checklists/gates.md` + `Memoria.md`.
