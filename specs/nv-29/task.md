# task.md — NV-29 Modelo de plan, paradas y cálculo de traslados

## Fase 0 — Preparación
- [x] T01 Rama `nv-29` desde `origin/main`; drift corregido en spec y plan (firma pendiente).

## Fase 1 — Implementación
- [x] T02 Cada parada tiene hora de llegada, duración, precio estimado y negocio aprobado y abierto (`Recalcular`, `Calculadora`, `FindVisibleMany`).
- [x] T03 Traslados entre paradas por movilidad: Haversine × 1,3 y `DefaultVelocidades`.
- [x] T04 Costos: total y por persona en el encabezado del plan.
- [x] T05 Control de concurrencia con `version` (`Repository.UpdateIfVersion`); índices y ARCO de `planes`.

## Fase 2 — Cierre
- [x] T06 Pruebas Unit e Integration, spec y `Memoria.md`.
- [ ] T07 Local: build, vet, Unit, lint, `critic`, `check_sdd`, `check_endpoints` (hecho); CI del PR: Integration, Security, Database, Contract, cobertura y Semgrep (pendiente).
