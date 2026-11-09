# task.md — NV-29 Modelo de plan, paradas y cálculo de traslados

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-29`.

## Fase 1 — Implementación
- [ ] T02 Cada parada tiene hora de llegada, duración, precio estimado y negocio aprobado y abierto…
- [ ] T03 Traslados entre paradas por movilidad (a pie, bici, auto, transporte): minutos y…
- [ ] T04 Costos: total y por persona en el encabezado del plan.
- [ ] T05 Control de concurrencia con version e If-Match, como en el perfil.

## Fase 2 — Cierre
- [ ] T06 Pruebas, documentos y `Memoria.md`.
- [ ] T07 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
