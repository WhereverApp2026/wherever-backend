# gates.md — NV-27 Tope de gasto y caché de la IA

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` (2026-11-11); `.env.example` solo suma tope y precios | listo |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; los avisos de la guardia solo llevan `parte` y el error; `usos_ia` no guarda la frase ni la salida | listo |
| critic B3 | Cambios de infraestructura o pipeline aprobados por el revisor (si aplica) | no aplica: sin cambios de Terraform ni del pipeline | listo |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (43 rutas)`, sin rutas nuevas | listo |
| Pruebas | Cinco suites verdes con servicios reales | Unit verde en `golang:1.27.3` (2026-11-11) y `golangci-lint` v2.15.0 con 0 hallazgos; Integration, Security, Database y Contract en el CI del PR | parcial |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | pendiente |
| Semgrep | 0 hallazgos | 0 con `p/golang`, `p/owasp-top-ten`, `p/secrets` y `.semgrep/semgrep.yaml` sobre lo cambiado (semgrep 1.120.0) | listo |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)`; `Memoria.md` con la sección NV-27 | listo |
| Tope | Al llegar al tope no se llama al modelo | `TestGuardia_TopeExactoNoLlamaAlModelo` (Unit, verde) y `TestIAGasto_AlLlegarAlTopeNoLlamaYReconstruye` (Integration, en el CI) | parcial |
