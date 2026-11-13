# gates.md — NV-63 Postales, recuerdos y momentos

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | — | pendiente |
| critic B2 | Sin textos, URLs firmadas ni datos personales en logs | — | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | — | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | — | pendiente |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | — | pendiente |
| Pruebas | Integration, Security, Database y Contract con servicios reales | CI del PR | pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | — | pendiente |
| Visibilidad | Privada, amigos, pública y bloqueos; nada visible sin moderación aprobada | `tests/integration/postales_test.go`, `momentos_test.go` | pendiente |
| Privacidad | Sin EXIF ni GPS; ARCO borra y exporta solo lo propio | pruebas de ARCO | pendiente |
