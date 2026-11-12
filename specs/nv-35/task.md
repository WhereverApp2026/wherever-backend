# task.md — NV-35 Buscar personas

## Fase 0 — Preparación
- [x] T01 Spec revisada (drift corregido: el límite de solicitudes y el bloqueo pasan a nv-36); rama `nv-35`.

## Fase 1 — Implementación
- [x] T02 GET /personas/buscar?q= busca por nombre de usuario (prefijo) o por teléfono exacto.
- [x] T03 Límite por usuario: 30 búsquedas por hora.

## Fase 2 — Cierre
- [x] T04 Pruebas, documentos y `Memoria.md`.
- [ ] T05 `go build`, `go vet`, Unit, lint (local ✅); Integration, Security, Database y Contract en el CI del PR; `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
