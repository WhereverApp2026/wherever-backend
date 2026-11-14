# task.md — NV-70 Perfil de persona, silenciar amigos y amigos en contactos

## Fase 0 — Preparación
- [ ] T01 Spec revisada y decisiones del revisor tomadas; filas en `docs/endpoints.md`; rama de trabajo contra `main`.

## Fase 1 — Implementación
- [ ] T02 PUT /amistades/{id}/silencio con `silenciado_por`; `silenciado` en `GET /amistades` y en la exportación ARCO.
- [ ] T03 GET /personas/{id}: parte básica para todos, parte de amigo con Neo4j degradable, 404 uniforme y 120 por hora.
- [ ] T04 POST /personas/contactos: normalizar, HMAC en memoria, una consulta `$in`, quitar bloqueos, `indice`, 10 por día, nada guardado ni registrado.

## Fase 2 — Cierre
- [ ] T05 Pruebas, `docs/endpoints.md`, `docs/rate-limit-matrix.md` y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
