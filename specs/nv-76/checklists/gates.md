# gates.md — NV-76 Insignia del Sello de Excelencia

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1–B4 | Sin secretos; rutas en `docs/endpoints.md` | `critic-guardrails` PASS; `check_endpoints` OK (255 rutas) (2026-11-21) | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | Docker `golang:1.27.3`; `golangci-lint` v2.15.0 → 0 issues | ✅ |
| Unit | SVG válido, nombre escapado y recortado, datos del Sello | `internal/negocios/panel/insignia_test.go` | ✅ |
| Integration | Con Sello 200 JSON/SVG; sin Sello, retirado o ajeno 404; formato inválido 422 | `tests/integration/negocios_insignia_test.go` (local con contenedores y CI) | ✅ |
| Security | 401, 403, dueño ajeno 404, ids inválidos, nombre con marcado escapado, CSP y nosniff | `tests/security/negocios_insignia_security_test.go` (local y CI) | ✅ |
| Contract | Forma del 200 | `tests/contract/testdata/negocios/insignia_200.json` | ✅ |
| SDD | `check_sdd` OK | spec en Borrador con evidencia | ✅ |
