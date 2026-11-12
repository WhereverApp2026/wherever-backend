# gates.md — NV-18 Caché en Redis

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `scripts/critic-guardrails.sh` → PASS (2026-11-12) | ok |
| critic B2 | Sin tokens ni datos personales en logs | `critic` PASS; el aviso de la caché no lleva la llave | ok |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | sin cambios de infraestructura ni pipeline | n/a |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints` OK, sin endpoints nuevos | ok |
| Build/Vet/Unit/Lint | Local en `golang:1.27.3` | `go build`, `go vet`, Unit verdes; `golangci-lint` v2.15.0: 0 issues | ok |
| Pruebas | Cinco suites verdes con servicios reales | CI del PR | pendiente |
| Cobertura | Mínimo 45 % | CI del PR | pendiente |
| Semgrep | 0 hallazgos | CI del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `check_sdd` SDD-OK | ok |
| Redis caído | La API responde sin caché | `TestCache_RedisCaidoRespondeMongo` + Unit `TestCache_RedisCaidoRespondeElOrigenYSeApaga` | ok en Unit; Integration en CI |
| Consumo | Comandos por petición ≤ 2 | `TestPresupuestoRedis_CacheDeFicha`: fallo +2, acierto +1 | CI del PR |
