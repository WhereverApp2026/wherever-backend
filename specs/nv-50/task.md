# task.md — NV-50 Embeddings de negocios

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-50`.

## Fase 1 — Implementación
- [ ] T02 Cada negocio tiene un vector de significado (embedding) calculado con un modelo…
- [ ] T03 Los vectores se calculan al cargar o editar un negocio y se pueden recalcular todos con…
- [ ] T04 Si el servicio de embeddings falla o no hay tope disponible, todo sigue funcionando con…

## Fase 2 — Cierre
- [ ] T05 Pruebas, documentos y `Memoria.md`.
- [ ] T06 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
