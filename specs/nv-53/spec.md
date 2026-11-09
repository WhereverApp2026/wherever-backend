# NV-53 — Sugerencias y señal social

## Contexto
Neo4j responde a «personas que quizá conoces» y «tus amigos han ido». **Origen**: Requisito del curso (base de grafos) y hoja J2 del Excel. **Depende de**: NV-52.

## Requisitos
1. `GET /amistades/sugerencias` devuelve personas con amigos en común, ordenadas por cantidad, excluyendo amigos, bloqueados y solicitudes pendientes, y respetando `buscable_por_usuario`.
2. `GET /lugares/{id}/amigos` devuelve cuántos amigos del usuario han estado y los avatares de hasta 3 (sobre planes guardados que incluyeron ese negocio).

## Diseño
- **Sugerencias** con tope de 20 y caché de 10 minutos (nv-18).
- **Si Neo4j no responde:** sugerencias devuelve lista vacía con `degradado: true`; no es error.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/amistades/sugerencias` | JWT | 200 |
| GET | `/api/v1/lugares/{id}/amigos` | JWT | 200, 404 |

## Tests
- `Integration/Grafo 10` con Neo4j real: sugerencias con amigos en común, exclusión de amigos, bloqueados y pendientes, orden, tope de 20.
- `Integration/SeñalSocial 4`: amigos que fueron al negocio, ninguno, más de 3.
- `Security/Grafo 4`: Cypher parametrizado (intento de inyección en el id), aislamiento entre usuarios, privacidad de quien apagó ser buscable.

## Criterios
- Neo4j tiene consultas reales que se usan en la app.

## Límites
- Sin algoritmos de grafos avanzados (comunidades, centralidad).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 3: amistades, ML y grafo
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-53/checklists/gates.md` + `Memoria.md`.
