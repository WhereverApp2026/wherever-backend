# NV-53 — Sugerencias y señal social

## Contexto
Neo4j responde a «personas que quizá conoces» y «tus amigos han ido». **Origen**: Requisito del curso (base de grafos) y hoja J2 del Excel. **Depende de**: NV-52.

## Requisitos
1. `GET /amistades/sugerencias` devuelve personas con amigos en común, ordenadas por cantidad, excluyendo amigos, bloqueados y solicitudes pendientes, y respetando `buscable_por_usuario`.
2. `GET /lugares/{id}/amigos` devuelve cuántos amigos del usuario han estado y los avatares de hasta 3 (sobre planes guardados que incluyeron ese negocio).

## Diseño
- **Módulo** `internal/sugerencias` (service + handler) y consultas Cypher en `internal/platform/neo4j/consultas.go`, siempre parametrizadas.
- **Mongo manda:** las relaciones salen de `amistades.Repository.ListByUser` (amigos, pendientes y bloqueos en ambos sentidos se excluyen de sugerencias; solo las `aceptada` cuentan como amigos en la señal social) y `buscable_por_usuario` de `usuarios.FindSuggestableByIDs`. Neo4j recorre el grafo de nv-52 y da hasta 100 candidatos; Mongo filtra y quedan hasta 20, por amigos en común (empate por id).
- **Sin caché:** nv-18 es de la ola 4 y una caché de 10 minutos mostraría a alguien recién bloqueado. La consulta es de dos saltos con tope.
- **Si Neo4j no responde** (o tarda más de 3 s): las dos rutas responden 200 vacías con `degradado: true`; no es error.

## Contratos
| Método | Ruta | Auth | Respuestas |
|---|---|---|---|
| GET | `/api/v1/amistades/sugerencias` | JWT | 200, 401, 403 |
| GET | `/api/v1/lugares/{id}/amigos` | JWT | 200, 401, 403, 404 |

Formas en `docs/endpoints.md`.

## Tests
- Unit `internal/sugerencias/sugerencias_test.go` (5): orden, tope y no buscables; degradado; conteo y 3 avatares; 404; 401.
- Integration `tests/integration/sugerencias_test.go` con Neo4j real (8): orden por amigos en común, exclusión de amigos, pendientes, bloqueos, no buscables e inactivos, tope de 20, Neo4j caído, 401; señal social con plan guardado, más de 3 y solo amigos, ninguno y 404.
- Security `tests/security/sugerencias_security_test.go` (4): Cypher parametrizado (intento de inyección en el id), aislamiento entre usuarios, privacidad de quien apagó ser buscable, 401.

## Criterios
- Neo4j tiene consultas reales que se usan en la app.

## Límites
- Sin algoritmos de grafos avanzados (comunidades, centralidad).

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-12); implementada, falta el CI del PR
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** sin caché de 10 minutos y degradado también en la señal social (ver Diseño).
- **Detalle:** la evidencia irá en `specs/nv-53/checklists/gates.md` + `Memoria.md`.
