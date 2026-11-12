# task.md — NV-22 Búsqueda y autocompletar

## Fase 0 — Preparación
- [x] T01 Rama `nv-22`; drift corregido (tolerancia a errores en Go, no con el índice de texto). La firma sigue pendiente.

## Fase 1 — Implementación
- [x] T02 Autocompletar desde 2 letras, tolerante a acentos y mayúsculas, y búsqueda GET…
- [x] T03 Búsquedas recientes del usuario (GET/DELETE /busquedas/recientes), solo con sesión.

## Fase 2 — Cierre
- [x] T04 Pruebas, documentos y `Memoria.md`.
- [x] T05 `go build`, `go vet`, Unit y lint en local; las otras cuatro suites en el CI del PR; `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
