# gates.md — NV-29 Modelo de plan, paradas y cálculo de traslados

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | salida de `critic` | pendiente |
| critic B2 | Sin tokens ni datos personales en logs | salida de `critic` | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | salida de `critic` + aprobación | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints` | pendiente |
| Pruebas | Cinco suites verdes con servicios reales | salida de `go test` | pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` | pendiente |
| Semgrep | 0 hallazgos | salida de `semgrep` | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `check_sdd` | pendiente |
| Recalculo | Ningún plan guardado tiene paradas traslapadas | salida de pruebas | pendiente |
