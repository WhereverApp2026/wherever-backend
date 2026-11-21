# gates.md — NV-78 Insignias de las personas

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1–B4 | Sin secretos; rutas en `docs/endpoints.md` | `critic-guardrails` PASS; `check_endpoints` OK (256 rutas en esta rama) (2026-11-21) | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | Docker `golang:1.27.3`; `golangci-lint` v2.15.0 → 0 issues; arquitectura verde | ✅ |
| Unit | Catálogo, ganada y progreso | `internal/insignias/catalogo_test.go` | ✅ |
| Integration | Ganadas y bloqueadas, fecha estable, vista ajena, ARCO, concurrencia | `tests/integration/insignias_test.go` (local con contenedores y CI) | ✅ |
| Security | 401, 403, 404 por bloqueo, propia o inexistente, 422 | `tests/security/insignias_security_test.go` (local y CI) | ✅ |
| Database | `ux_insignia_usuario` único | `tests/database/indexes_test.go` (suite completa verde en local) | ✅ |
| Contract | Formas de las dos rutas y la exportación | `insignias/mias_200`, `insignias/persona_200`, `usuarios/exportacion_descargar_200` (suite completa verde en local) | ✅ |
| SDD | `check_sdd` OK | spec en Borrador con evidencia | ✅ |
