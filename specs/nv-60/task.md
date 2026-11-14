# task.md — NV-60 Pagos con Stripe: anticipo, webhook y depósitos a negocios

## Fase 0 — Preparación
- [x] T01 Enmienda de la constitución §3 aprobada y aplicada (1.8.0); decisión de Stripe Connect confirmada; nv-59 y nv-55 en `main`; rama contra `main`.
- [ ] T02 `STRIPE_SECRET_KEY` y `STRIPE_WEBHOOK_SECRET` en Parameter Store (`SecureString`, modo prueba) — operador; en `.env.example` sin valores y `STRIPE_` fuera de `futurePrefixes` — hecho.

## Fase 1 — Implementación
- [x] T03 `StripeAPI` con `stripe-go` y el doble `internal/testutil/stripetest` (rutas, registro de `Idempotency-Key`, eventos firmados).
- [x] T04 `pagos` y `liquidaciones` con sus índices (TTL `conservar_hasta`) en `scripts/indexes`.
- [x] T05 POST `/reservas/{id}/pago` con `Idempotency-Key`, reutilización del PaymentIntent, 410 al vencer y `carriesTokens`.
- [x] T06 POST `/webhooks/stripe`: firma, `idem:stripe_evento:{id}`, eventos `succeeded`, `payment_failed`, `charge.refunded`, `account.updated`; pago tardío; exento de `Version`.
- [x] T07 GET `/pagos/{id}/estado` y `Refunder` para la cancelación de nv-59.
- [x] T08 Connect: `cobros/conectar`, `cuenta-deposito` (CLABE), `saldo`, `liquidaciones`, `reservas/{rid}/cobro` y corte semanal.
- [x] T09 Logger (llaves ocultas), `Purger` (no borra pagos) y `Exporter`.

## Fase 2 — Cierre
- [x] T10 Pruebas, `docs/endpoints.md`, `docs/rate-limit-matrix.md`, aviso de privacidad en `main` y `Memoria.md`.
- [ ] T11 (local hecho; CI del PR y prueba manual pendientes) `go build`, `go vet`, Unit, lint, `critic`, `check_sdd`, `check_endpoints` en local; Integration, Security, Database y Contract en el CI del PR; prueba manual con tarjeta de prueba; evidencia en `gates.md`.
