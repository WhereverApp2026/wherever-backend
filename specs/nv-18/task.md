# task.md — NV-18 Caché en Redis

## Fase 0 — Preparación
- [x] T01 Corregir el desfase de la spec (paquete, llaves, alcance sin configuración); rama `claude/nice-almeida-3fcaaa` desde `main`. Firma del revisor pendiente.

## Fase 1 — Implementación
- [x] T02 `internal/platform/cache`: `Obtener` con `singleflight`, `Invalidar` con un `DEL`, pausa de 30 s ante errores de Redis; pruebas Unit.
- [x] T03 `internal/negocios`: categorías (1 h) y fichas (5 min) en caché, «abierto ahora» recalculado, invalidación al importar con el `_id` real; armado en `cmd/api` y `authtest`.
- [x] T04 La compresión (nv-12) no se toca.

## Fase 2 — Cierre
- [x] T05 `tests/integration/cache_test.go` (Caché 6) y `TestPresupuestoRedis_CacheDeFicha`; estimación de Redis en `Memoria.md`.
- [ ] T06 (local verde: build, vet, Unit, lint, critic, sdd, endpoints; Integration/Security/Database/Contract en el CI del PR) cinco suites, cobertura, Semgrep; evidencia en `gates.md`.
