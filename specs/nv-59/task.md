# task.md — NV-59 Reservas y disponibilidad

## Fase 0 — Preparación
- [x] T01 nv-55 y nv-56 en `main` (`reservas_config` con `confirmacion` y `permite_cambios`, escrita por nv-56); decisiones resueltas; rama contra `main`.

## Fase 1 — Implementación
- [x] T02 `disponibilidad.go`, `reembolso.go` y generador de código, con pruebas Unit.
- [x] T03 `reservas` y `bloqueos_horario` con sus índices en `scripts/indexes`.
- [x] T04 GET `/lugares/{id}/disponibilidad`.
- [x] T05 POST `/reservas` con `Idempotency-Key`, `lock:reserva:{negocio}:{fecha_hora}`, tope de 3 apartados y 10 por hora.
- [x] T06 GET `/reservas`, GET y PATCH `/reservas/{id}`, GET `/reembolso`, POST `/cancelar` (puertos `Anticipos` y `Refunder` y `ConfirmarPago` para nv-60).
- [x] T07 Agenda, PATCH de estado, escanear y bloqueos del negocio.
- [x] T08 Barrido cada 5 min (expirar apartados, `no_asistio`); `Purger` y `Exporter`; fuente del panel, reservas futuras y observador de nv-56.

## Fase 2 — Cierre
- [x] T09 Pruebas, `docs/endpoints.md`, `docs/rate-limit-matrix.md` y `Memoria.md`.
- [x] T10 `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; evidencia en `gates.md`.
