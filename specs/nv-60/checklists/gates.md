# gates.md — NV-60 Pagos con Stripe: anticipo, webhook y depósitos a negocios

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | llaves de Stripe solo en Parameter Store | pendiente |
| critic B2 | Sin tokens ni datos personales en logs | `client_secret`, `stripe-signature` y `clabe` ocultos | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor | dos parámetros nuevos en `infra/aws-dev`; enmienda de §3 | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints` con las 8 rutas | pendiente |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | — | pendiente |
| Pruebas | Integration, Security, Database y Contract con servicios reales y el doble de Stripe | CI del PR | pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente |
| Dependencias | `govulncheck` sin hallazgos con `stripe-go` | job `go-security` del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | spec en borrador: `SDD-OK` | pendiente |
| Firma | Webhook sin firma, con firma mala o vieja = 400 | `tests/security/pagos_security_test.go` (CI) | pendiente |
| Una vez | El mismo evento 10 veces confirma una vez | `TestPagos_Webhook_ConcurrenteUnaVez` (CI) | pendiente |
| Sin tarjeta | Ningún documento ni log con número, CVC o CLABE completa | integración + revisión del logger | pendiente |
| Constitución | Enmienda §3 1.8.0 aprobada antes de agregar `stripe-go` | — | pendiente |
