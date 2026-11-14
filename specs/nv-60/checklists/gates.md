# gates.md — NV-60 Pagos con Stripe: anticipo, webhook y depósitos a negocios

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | llaves de Stripe solo en Parameter Store; `.env.example` sin valores; el doble usa llaves falsas | PASS (critic local) |
| critic B2 | Sin tokens ni datos personales en logs | el logger oculta `secret`, `signature` y `clabe`; el webhook no registra su cuerpo | PASS (critic local); `TestPagosSecurity_ClientSecretSoloEnSuRuta` en CI |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor | sin Terraform: el operador carga dos SecureString en `/wherever/dev/`; enmienda §3 1.8.0 aprobada el 2026-11-14 | PASS |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints` OK (173 rutas; 7 nuevas, `cuenta-deposito` ya era de nv-55) | PASS |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | local 2026-11-14 | PASS |
| Pruebas | Integration, Security, Database y Contract con servicios reales y el doble de Stripe | local en Docker 2026-11-14: Integration `TestPagos*` y `TestReservas*`, Security `TestPagosSecurity*` y `TestReservasSecurity*`, Database y Contract completas en verde; el resto en el CI del PR | PASS local; CI pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente (CI) |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente (CI) |
| Dependencias | `govulncheck` sin hallazgos con `stripe-go` | job `go-security` del PR | pendiente (CI) |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (73 specs)`; entrada NV-60 en `Memoria.md` | PASS |
| Firma | Webhook sin firma, con firma mala o vieja = 400 | `tests/security/pagos_security_test.go` | PASS local |
| Una vez | El mismo evento 10 veces confirma una vez | `TestPagos_Webhook_ConcurrenteUnaVez` | PASS local |
| Sin tarjeta | Ningún documento ni log con número, CVC o CLABE completa | DTO estricto (`TestDecodeEstricto_RechazaCamposDeTarjeta`), `TestPagos_CobroWebhookYReembolso` | PASS local |
| Constitución | Enmienda §3 1.8.0 aprobada antes de agregar `stripe-go` | decisión del revisor 2026-11-14 | PASS |
