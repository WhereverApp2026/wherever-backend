# NV-32 — Itinerario con IA y verificación contra el catálogo

## Contexto
La IA propone 2 a 4 paradas y la API descarta cualquier lugar que no exista o esté cerrado. **Origen**: US-035, US-039, US-040, US-041, US-042, US-043 y hoja A4 del Excel. **Depende de**: NV-31.

## Requisitos
1. **Itinerario (US-042):** la IA elige de 2 a 4 paradas, en orden horario, con duración sugerida, dentro del presupuesto total.
2. **Verificación (US-043):** la API descarta cualquier parada que no esté en los candidatos, esté cerrada a esa hora o rompa el presupuesto; la reemplaza con el siguiente candidato del motor y registra cada descarte.
3. Cada parada trae su `por_que` (nv-23).

## Diseño
- **Descartes:** colección `ia_descartes` (motivo, id del negocio, sin texto del usuario), TTL de 30 días, para medir la tasa de alucinación.
- **Ánimo y alergias:** solo con consentimiento vigente; las alergias siempre actúan como filtro duro.

## Contratos
Sin endpoints nuevos.

## Tests
- `Unit/Verificar 10`: id inventado, negocio cerrado a esa hora, presupuesto excedido, orden de horas roto; se reemplaza y se registra.

## Criterios
- El plan devuelto solo tiene lugares reales, abiertos y dentro del presupuesto.

## Límites
- Sin límites propios más allá de los ya anotados.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 2: planes con IA (hallazgo C1)
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-32/checklists/gates.md` + `Memoria.md`.
