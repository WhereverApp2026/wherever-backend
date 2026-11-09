# task.md — NV-35 Buscar personas

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-35`.

## Fase 1 — Implementación
- [ ] T02 GET /personas/buscar?q= busca por nombre de usuario (prefijo) o por teléfono exacto.
- [ ] T03 Límites por usuario: 30 búsquedas y 20 solicitudes por hora.

## Fase 2 — Cierre
- [ ] T04 Pruebas, documentos y `Memoria.md`.
- [ ] T05 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
