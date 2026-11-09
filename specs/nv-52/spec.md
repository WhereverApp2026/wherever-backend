# NV-52 — Proyección de amistades a Neo4j

## Contexto
Las amistades aceptadas se reflejan en el grafo sin que Neo4j pueda romper nada. **Origen**: Requisito del curso (base de grafos) y hoja J2 del Excel. **Depende de**: NV-36, NV-11, NV-20, NV-30.

## Requisitos
1. Las amistades aceptadas se reflejan en Neo4j como relaciones `AMIGO_DE`, y los planes guardados (nv-30) como `FUE_A` hacia los negocios de sus paradas; los usuarios y negocios, como nodos con solo su id. nv-53 necesita las dos.
2. La proyección a Neo4j es **eventual**: Mongo manda; si Neo4j falla, la amistad igual se guarda y se reintenta.
3. Un comando reconstruye todo el grafo desde Mongo.
4. Sin datos personales en el grafo: solo identificadores.

## Diseño
- **Nodos:** `(:Usuario {id})`, `(:Negocio {id})`; **relaciones:** `(:Usuario)-[:AMIGO_DE]->(:Usuario)` y `(:Usuario)-[:FUE_A]->(:Negocio)`.
- **Proyección:** una cola de eventos persistida en Mongo (`grafo_pendiente`) que un trabajador aplica con reintentos; es idempotente (`MERGE`).
- **Consultas parametrizadas** siempre; nunca se arma Cypher con texto del usuario.
- **Límites de Aura Free:** 50,000 nodos y 175,000 relaciones; la API cuenta nodos y avisa al 70 %.

## Contratos
Sin endpoints nuevos.

## Tests
- `Integration/Proyección 6`: aceptar proyecta, eliminar quita, repetir evento no duplica, Neo4j caído reintenta y la acción original responde bien.
- `Integration/Reconstruir 2`: el grafo reconstruido coincide con Mongo.

## Criterios
- Un fallo del grafo no afecta amistades ni planes.
- Se vigila el límite de nodos.

## Límites
- El límite de Aura Free no está claro: su página dice 50 000 nodos y la FAQ dice 200 000. Se usa la cifra menor hasta confirmarla en la consola.
- Aura Free se pausa por inactividad y tiene límites de tamaño.
- El grafo es una proyección: puede ir segundos atrás de Mongo.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador revisado (2026-11-08); ruta prioritaria, ola 3: amistades, ML y grafo
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor antes de ejecutar:** Confirmar el alcance y los valores propuestos en Diseño.
- **Detalle:** la evidencia irá en `specs/nv-52/checklists/gates.md` + `Memoria.md`.
