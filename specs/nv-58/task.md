# task.md — NV-58 La Vaca: gastos compartidos del grupo

## Fase 0 — Preparación
- [ ] T01 nv-37 en `main`; decisiones del revisor resueltas; rama de trabajo contra `main`.

## Fase 1 — Implementación
- [ ] T02 `corte.go`: saldos, residuo y transferencias mínimas en centavos, determinista; pruebas de tabla y de propiedad.
- [ ] T03 `gastos` y `deudas_vaca` con sus índices en `scripts/indexes`.
- [ ] T04 POST y GET `/grupos/{id}/gastos` con `id_cliente` idempotente, filtros y cursor.
- [ ] T05 PATCH y DELETE `/gastos/{id}` con `If-Match`, recálculo bajo `lock:vaca:{grupo}` y 409 `deudas_pagadas` salvo `confirmar`.
- [ ] T06 GET `/grupos/{id}/vaca`, PATCH `/vaca/deudas/{id}` y GET `/vaca/mis-deudas`.
- [ ] T07 `Purger` y `Exporter` (gastos y deudas en la exportación; la Vaca se conserva al borrar la cuenta).

## Fase 2 — Cierre
- [ ] T08 Pruebas, `docs/endpoints.md` y `Memoria.md`.
- [ ] T09 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
