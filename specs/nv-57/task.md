# task.md — NV-57 Chat de grupo

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec y elegir la opción de tiempo real (A, B o C); nv-37 fusionada; rama contra `main`.
- [ ] T02 Medir con `scripts/medir-redis.sh` (y en Atlas, si es C) antes de fijar valores.

## Fase 1 — Implementación
- [ ] T03 Colección `mensajes`, índices e idempotencia por `id_cliente`.
- [ ] T04 GET y POST `/grupos/{id}/mensajes` con `antes_de`, `despues_de`, `q` y `ultimo_mensaje`.
- [ ] T05 PUT reacciones y DELETE del mensaje propio.
- [ ] T06 `PublicarSistema` para nv-39 y push genérico `mensaje_grupo` (nv-40, nv-41).
- [ ] T07 `Purger` y `Exporter` registrados.
- [ ] T08 Canal `/tiempo-real` con la opción aprobada (si aplica).

## Fase 2 — Cierre
- [ ] T09 Pruebas, documentos y `Memoria.md`.
- [ ] T10 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd` y `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
