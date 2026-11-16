# gates.md — NV-75 Consejo del día y descripción con IA para el negocio

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git; `GEMINI_API_KEY` solo en `.env`/SSM | `critic-guardrails` | pendiente |
| critic B2 | Al modelo solo agregados; logs sin contexto ni salida | `consejos_security_test.go` | pendiente |
| critic B3 | Cambios de infraestructura o pipeline aprobados | sin cambios; variable nueva documentada | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints` | pendiente |
| Build, vet y lint | Sin errores; 0 hallazgos | `golangci-lint` | pendiente |
| Unit | Verde | `internal/consejos`, descripción | pendiente |
| Integration | `consejos_test.go`, `negocios_descripcion_test.go` | CI del PR | pendiente |
| Security | `consejos_security_test.go` | CI del PR | pendiente |
| Database | `ux_consejo_dia`, TTL | CI del PR | pendiente |
| Contract | `consejos_contract_test.go` | CI del PR | pendiente |
| Redis | Consejo guardado = lectura; generación ≈ 14 comandos | `TestPresupuestoRedis_ConsejoNV75` | pendiente |
| Gemini | ≤ `IA_CONSEJO_DIA` consejos con IA al día; gasto dentro del tope | `consejos_test.go` | pendiente |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | CI del PR | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK` (spec en borrador) | pendiente |
