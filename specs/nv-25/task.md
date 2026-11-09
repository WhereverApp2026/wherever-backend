# task.md — NV-25 Clima e Inicio

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-25`.

## Fase 1 — Implementación
- [ ] T02 GET /inicio arma la pantalla en una llamada: saludo por hora, clima, chips según franja…
- [ ] T03 GET /contexto/clima devuelve el clima de la ciudad o coordenada, con caché de 30…

## Fase 2 — Cierre
- [ ] T04 Pruebas, documentos y `Memoria.md`.
- [ ] T05 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
