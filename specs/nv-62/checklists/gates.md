# gates.md — NV-62 Check-in y reseñas

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `scripts/critic-guardrails.sh` → PASS (local en Docker, 2026-11-14) | ✅ |
| critic B2 | Sin tokens, coordenadas ni datos personales en logs | critic PASS; los avisos de reseñas llevan `evento` e ids, nunca coordenadas ni texto; el check-in no guarda la ubicación (`TestCheckins_UbicacionQRyDuplicado`, CI) | ✅ local |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin variables, Terraform ni pipeline | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (199 rutas)` en Docker | ✅ |
| Build y lint | `go build`, `go vet`, Unit, `golangci-lint` | local en Docker `golang:1.27.3`: build, vet y Unit verdes; `golangci-lint` v2.15.0 `0 issues` | ✅ |
| Pruebas | Integration, Security, Database y Contract con servicios reales | CI del PR (no se corrieron en local, AGENTS.md §4) | pendiente de CI |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente de CI |
| Semgrep | 0 hallazgos | job `semgrep` del PR | pendiente de CI |
| SDD | `check_sdd` OK | `SDD-OK (73 specs)` | ✅ |
| Una por visita | Una reseña por check-in | `TestResenas_Concurrencia_UnaPorVisita` | pendiente de CI |
| Recálculo exacto | `calificacion` igual al cálculo desde cero | `TestResenas_Concurrencia_RecalculoExacto` | pendiente de CI |
| Check-in diario | Uno válido por negocio y día | `TestCheckins_Concurrencia_UnoPorDia` | pendiente de CI |
| Privacidad y ARCO | Sin ubicación guardada; anonimiza al eliminar la cuenta; exporta check-ins y reseñas | `TestCheckins_UbicacionQRyDuplicado`, `TestResenas_ARCO`, contrato `exportacion_descargar_200` | pendiente de CI |
| Propiedad | Reseña ajena 404; negocio ajeno 404; negocio propio no se reseña | `TestResenas_EditarBorrarYUtil`, `TestResenas_RespuestaResumenYPanel`, `TestResenas_CrearPublicarYCache` | pendiente de CI |
