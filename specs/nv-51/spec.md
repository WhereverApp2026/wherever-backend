# NV-51 — Búsqueda semántica y evaluación

## Contexto
Encontrar lugares por significado y medir si mejora frente a solo reglas. **Origen**: Requisito del curso (ML) y US-035. **Depende de**: NV-50.

## Requisitos
1. `GET /lugares/semantica?q=` devuelve los negocios más cercanos en significado a la frase, solo aprobados.
2. El motor de reglas (nv-23) suma un factor `afinidad_semantica` y el asistente (nv-31) usa esta búsqueda para elegir sus candidatos.
3. Medición de calidad: un conjunto de 30 frases con los lugares esperados; se reportan aciertos entre los 5 primeros (top-5) frente al motor sin semántica.

## Diseño
- **Embedding de la consulta:** una llamada por frase, con caché de 24 horas por huella; cuenta contra el tope de gasto de nv-26 (muy bajo).

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/lugares/semantica?q=` | opcional | 200, 422, 429 |

## Tests
- `Integration/Semántica 8` con MongoDB real: frases del conjunto de 30, filtro por estado, límite, servicio de embeddings caído (respuesta del motor de reglas).
- Medición: top-5 con y sin semántica sobre las 30 frases; se guarda en `docs/ml-evaluacion.md`.

## Criterios
- La búsqueda semántica mejora el top-5 frente a solo reglas, con evidencia medida.
- Hay un informe de evaluación para la defensa.

## Límites
- Con 30 negocios la mejora puede ser pequeña; se reporta lo medido.
- Plantilla solo en español.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 3: amistades, ML y grafo
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-51/checklists/gates.md` + `Memoria.md`.
