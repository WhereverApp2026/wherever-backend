# gates.md — NV-62 Check-in y reseñas

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | — | pendiente |
| critic B2 | Sin tokens, coordenadas ni datos personales en logs | — | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | — | pendiente |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | — | pendiente |
| Pruebas | Integration, Security, Database y Contract con servicios reales | CI del PR | pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente |
| SDD | `check_sdd` OK | solo documentos: `SDD-OK` | pendiente |
| Una por visita | Una reseña por check-in | `TestResenas_Concurrencia_UnaPorVisita` | pendiente |
| Recálculo exacto | `calificacion` igual al cálculo desde cero | `TestResenas_Concurrencia_RecalculoExacto` | pendiente |
| Check-in diario | Uno válido por negocio y día | `TestCheckins_Concurrencia_UnoPorDia` | pendiente |
