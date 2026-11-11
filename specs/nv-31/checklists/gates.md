# gates.md — NV-31 Interpretar la frase y armar candidatos

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (local, 2026-11-11); la llave de Gemini sigue en SSM | OK |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; el log del asistente lleva solo conteos y el tipo de falla (`TestErrorIA_...`, `TestAsistente_GeneraPlan...` revisan que la frase no salga) | OK |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | No aplica: sin cambios de infraestructura ni pipeline | OK |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (44 rutas)`; nueva `POST /api/v1/planes/generar` | OK |
| Build y lint | `go build`, `go vet`, Unit y `golangci-lint` v2.15.0 | local en Docker: Unit verde (`internal/asistente` 59 %), `0 issues` | OK |
| Pruebas | Integration, Security, Database y Contract verdes con servicios reales | CI del PR (`asistente_test.go`, `asistente_security_test.go`) | pendiente |
| Cobertura | Mínimo 45 % | CI del PR (`check_coverage`) | pendiente |
| Semgrep | 0 hallazgos | CI del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)`; sección NV-31 en `Memoria.md` | OK |
| Privacidad IA | Al modelo solo instrucciones fijas, día y hora y la frase anonimizada | `TestInterpretar_FraseAnonimizadaYSinDatosPersonales`, `TestInterpretar_TextoConInstrucciones`; en Integration se revisa que no viajen id, correo, nombre, ubicación ni alergias | OK (Unit) |
| Alergias | Filtro duro siempre, aunque la frase no las diga | `TestContexto_AlergiasYDietaSiempreComoFiltroDuro`; Integration excluye el negocio con cacahuate | OK (Unit) |
