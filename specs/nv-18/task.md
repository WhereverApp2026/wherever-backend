# task.md — NV-18 Caché en Redis

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-18`.

## Fase 1 — Implementación
- [ ] T02 Caché en Redis con un ayudante cache.Obtener(clave, ttl, fn) que evita avalanchas…
- [ ] T03 La compresión ya está en producción (nv-12) y no se toca.

## Fase 2 — Cierre
- [ ] T04 Pruebas, documentos y `Memoria.md`.
- [ ] T05 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
