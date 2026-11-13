# plan.md — NV-60 Pagos con Stripe: anticipo, webhook y depósitos a negocios

1. Mapeo
- Crear: `internal/pagos/` (model, repository, service, handler, dto, `stripe.go` con `StripeAPI`, `webhook.go`, `liquidacion.go`, privacy), `internal/testutil/stripetest` (doble HTTP de Stripe y firma de eventos), `internal/testutil/pagostest`, `tests/integration/pagos_test.go`, `tests/security/pagos_security_test.go`.
- Modificar: `.specify/memory/constitution.md` y `constitution.md` (§3 y §12, con la aprobación del revisor), `go.mod` (`stripe-go` fijado), `internal/platform/config` (llaves por constructor, rechazo de `*_live_` fuera de prod) y su `config_example_test.go` (`STRIPE_` sale de `futurePrefixes`), `configs/.env.example`, `infra/aws-dev` (dos `SecureString` en Parameter Store), `internal/http/middleware` (`carriesTokens`, exención de `Version` para el webhook), logger (llaves ocultas), `internal/reservas` (puertos `Refunder` y `Payments`), `scripts/indexes` (`pagos` con TTL, `liquidaciones`), `docs/endpoints.md` (8 filas), `docs/rate-limit-matrix.md`, `docs/aviso-privacidad.md` solo si aún no trae lo de pagos (y `AVISO_VERSION`), `tests/database/indexes_test.go`, fixtures de contrato.
- Sin cambios: datos de tarjeta en cualquier DTO o colección.

2. Guardarraíles
- Servicios reales en las pruebas salvo Stripe (doble por interfaz externa), sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- El webhook verifica la firma antes de leer el JSON; idempotencia con `idem:stripe_evento:{id}` y `eventos_procesados`.
- El CI nunca usa llaves reales de Stripe.

3. Secuencia
1. Preparación: enmienda de §3 aprobada; nv-59 y nv-55 en `main`; cuenta de Stripe en modo prueba con Connect habilitado; llaves de prueba cargadas en Parameter Store por el operador.
2. Implementación: `StripeAPI` y su doble; cobro; webhook; reembolso; Connect y cuenta de depósito; corte semanal y consultas del negocio.
3. Cierre: privacidad, contrato, documentos y `Memoria.md`; prueba manual en el servidor de desarrollo con una tarjeta de prueba de Stripe y `stripe listen`.

4. Verificación
- `go build`, `go vet`, cinco suites, `critic`, `check_endpoints`, `check_sdd`, cobertura ≥ 45 %, Semgrep en 0, `govulncheck` sin hallazgos en `stripe-go`.
