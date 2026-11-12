# task.md — NV-34 Nombre de usuario y privacidad de búsqueda

## Fase 0 — Preparación
- [x] T01 Rama de la sesión desde `main`; la firma queda pendiente del revisor.

## Fase 1 — Implementación
- [x] T02 Campo nuevo nombre_usuario (3 a 20 caracteres, minúsculas, números y guion bajo; único…
- [x] T03 Ajustes de privacidad nuevos en PATCH /usuarios/yo/privacidad: buscable_por_usuario…

## Fase 2 — Cierre
- [x] T04 Pruebas, documentos y `Memoria.md`.
- [x] T05 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
