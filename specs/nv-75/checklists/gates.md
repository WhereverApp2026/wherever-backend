# gates.md — NV-75 Consejo del día y descripción con IA para el negocio

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1 | Sin secretos nuevos en Git; `GEMINI_API_KEY` solo en `.env`/SSM | `critic-guardrails` PASS (2026-11-16) | ✅ |
| critic B2 | Al modelo solo agregados; logs sin contexto ni salida | `consejos_security_test.go` (CI del PR) | CI |
| critic B3 | Cambios de infraestructura o pipeline aprobados | sin cambios de pipeline; `IA_CONSEJO_DIA` opcional (60) en `.env.example` | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints` OK (253 rutas) | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | Docker `golang:1.27.3`: build, vet, `golangci-lint` v2.15.0 → 0 issues | ✅ |
| Unit | Verde | `internal/consejos/reglas_test.go`, `internal/negocios/panel/descripcion_test.go`, config | ✅ |
| Integration | `consejos_test.go`, `negocios_descripcion_test.go` | CI del PR | CI |
| Security | `consejos_security_test.go` | CI del PR | CI |
| Database | `ux_consejo_dia`, TTL | CI del PR | CI |
| Contract | `consejos_contract_test.go` | CI del PR | CI |
| Redis | Consejo guardado = lectura (4); generación = 4 + 12; descripción = 7 + 12 | `TestPresupuestoRedis_ConsejoNV75` (CI) | CI |
| Gemini | ≤ `IA_CONSEJO_DIA` consejos con IA al día; gasto dentro del tope | `TestConsejos_CupoDiarioAgotado` (CI) | CI |
| Cobertura y Semgrep | ≥ 45 % y 0 hallazgos | CI del PR | CI |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (76 specs)`; spec en Borrador con evidencia | ✅ |
