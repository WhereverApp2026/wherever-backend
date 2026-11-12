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
- **Proyección:** una cola persistida en Mongo (`grafo_pendiente`, un documento por usuario, índice `ix_disponible_en`). Aceptar, eliminar o bloquear una amistad y crear, editar, repetir o borrar un plan encolan a quien actuó; el error de la cola va al log y no cambia la respuesta. La cola guarda **a quién recalcular**, no el evento: el trabajador (`internal/grafo`, una gorutina en cada instancia de la API) lee de Mongo las amistades aceptadas y los negocios de los planes activos del usuario y deja sus `AMIGO_DE` y `FUE_A` iguales en una transacción (`MERGE`, idempotente). Recalcular a una persona basta: `AMIGO_DE` es una por par y se consulta sin dirección. Un nodo sin relaciones se borra.
- **Reintentos:** si Neo4j falla, el usuario sigue en la cola con espera de 5 s que se duplica hasta 10 min; un usuario tomado se aparta 1 min (si la instancia cae, otra lo retoma). Si llega otro cambio mientras se aplica, no sale de la cola (`marca`).
- **ARCO:** `grafo.Privacy` se registra después de amistades y planes y encola al usuario; al recalcularlo sin relaciones, su nodo sale.
- **Reconstruir:** `make reconstruir-grafo` (`scripts/reconstruir-grafo`, como `make siembra`) crea el esquema (restricción nueva `negocio_id`), borra todas las `AMIGO_DE` y `FUE_A` y los nodos que quedan solos, y recalcula a cada usuario con amistad aceptada o plan activo. Los datos de demostración de nv-11 se conservan.
- **Consultas parametrizadas** siempre; nunca se arma Cypher con texto del usuario.
- **Límites de Aura Free:** 50,000 nodos y 175,000 relaciones; el trabajador cuenta nodos y relaciones (a lo más cada 10 min) y al 70 % deja `evento=grafo_limite` en el log. Los reintentos dejan `evento=grafo_reintento`.

## Contratos
Sin endpoints nuevos.

## Tests
- `Integration/Proyección 6` (`tests/integration/grafo_test.go`, `TestProyeccion_*`): aceptar proyecta (nodos solo con `id`), eliminar quita relación y nodos, bloquear quita, repetir no duplica, Neo4j caído reintenta con la amistad guardada, y un plan guardado proyecta `FUE_A` que se quita al borrarlo.
- `Integration/Reconstruir 2` (`TestReconstruir_*`): el grafo reconstruido coincide con Mongo y borra lo que Mongo no respalda; reconstruir dos veces deja lo mismo y conserva la demostración.
- `Unit` (`internal/grafo/grafo_test.go`): espera de reintento, negocios de planes activos sin repetir y aviso al 70 %.
- `Database`: `grafo_pendiente` con `_id_` e `ix_disponible_en`.

## Criterios
- Un fallo del grafo no afecta amistades ni planes.
- Se vigila el límite de nodos.

## Límites
- El límite de Aura Free no está claro: su página dice 50 000 nodos y la FAQ dice 200 000. Se usa la cifra menor hasta confirmarla en la consola.
- Aura Free se pausa por inactividad y tiene límites de tamaño.
- El grafo es una proyección: puede ir segundos atrás de Mongo (hasta 10 s si el cambio llegó a otra instancia).
- Dos recálculos cruzados del mismo par con lecturas viejas pueden dejar una arista de más hasta el siguiente cambio; `make reconstruir-grafo` lo corrige. Un usuario con más de 500 amigos proyecta solo 500 (`amistades.MaxList`).
- Si Mongo falla al encolar, el cambio queda solo en el log hasta reconstruir.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-12); ruta prioritaria, ola 3: amistades, ML y grafo
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** cola por usuario (estado, no eventos) con el trabajador dentro de la API; comando en `scripts/` en vez de `cmd/`.
- **Detalle:** la evidencia irá en `specs/nv-52/checklists/gates.md` + `Memoria.md`.
