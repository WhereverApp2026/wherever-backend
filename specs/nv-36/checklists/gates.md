# gates.md — NV-36 Solicitudes y lista de amigos

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (local, 2026-11-11) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; la respuesta no lleva correo ni teléfono | ✅ |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin cambios de infraestructura ni pipeline | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (64 rutas)` | ✅ |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` v2.15.0 | local en Docker: sin errores, `0 issues` | ✅ |
| Pruebas | Integration, Security, Database y Contract con servicios reales | CI del PR | pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)` | ✅ |
| Par único | Solicitudes cruzadas dejan un documento | `TestAmistades_Concurrencia_SolicitudesCruzadasUnSoloDocumento` (CI) | pendiente |
