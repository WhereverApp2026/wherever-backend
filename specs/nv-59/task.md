# task.md — NV-59 Reservas y disponibilidad

## Fase 0 — Preparación
- [ ] T01 nv-55 en `main` (`reservas_config` con `confirmacion` y `permite_cambios`); decisiones resueltas; rama contra `main`.

## Fase 1 — Implementación
- [ ] T02 `disponibilidad.go`, `reembolso.go` y generador de código, con pruebas Unit.
- [ ] T03 `reservas` y `bloqueos_horario` con sus índices en `scripts/indexes`.
- [ ] T04 GET `/lugares/{id}/disponibilidad`.
- [ ] T05 POST `/reservas` con `Idempotency-Key`, `lock:reserva:{negocio}:{fecha_hora}`, tope de 3 apartados y 10 por hora.
- [ ] T06 GET `/reservas`, GET y PATCH `/reservas/{id}`, GET `/reembolso`, POST `/cancelar` (puerto `Refunder` para nv-60).
- [ ] T07 Agenda, PATCH de estado, escanear y bloqueos del negocio.
- [ ] T08 Barrido cada 5 min (expirar apartados, `no_asistio`); `Purger` y `Exporter`.

## Fase 2 — Cierre
- [ ] T09 Pruebas, `docs/endpoints.md`, `docs/rate-limit-matrix.md` y `Memoria.md`.
- [ ] T10 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
