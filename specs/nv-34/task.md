# task.md — NV-34 Nombre de usuario y privacidad de búsqueda

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-34`.

## Fase 1 — Implementación
- [ ] T02 Campo nuevo nombre_usuario (3 a 20 caracteres, minúsculas, números y guion bajo; único…
- [ ] T03 Ajustes de privacidad nuevos en PATCH /usuarios/yo/privacidad: buscable_por_usuario…

## Fase 2 — Cierre
- [ ] T04 Pruebas, documentos y `Memoria.md`.
- [ ] T05 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
