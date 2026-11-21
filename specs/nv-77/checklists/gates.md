# gates.md — NV-77 Descripción del perfil

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| critic B1–B4 | Sin secretos; rutas en `docs/endpoints.md` | `critic-guardrails` PASS; `check_endpoints` OK (255 rutas, sin rutas nuevas) (2026-11-21) | ✅ |
| Build, vet y lint | Sin errores; 0 hallazgos | Docker `golang:1.27.3`; `golangci-lint` v2.15.0 → 0 issues (escapes `\u` para las marcas de dirección en las pruebas) | ✅ |
| Unit | Limpieza y validación de 160 runas | `internal/usuarios/account/descripcion_test.go` | ✅ |
| Integration | Guardar, ver, borrar, 422 por largo y por moderación | `tests/integration/usuarios_descripcion_test.go` (local con contenedores y CI) | ✅ |
| Security | 401, sin editar a otra persona, sin marcas de dirección, JSON, bloqueo = 404 | `tests/security/usuarios_descripcion_security_test.go` (local y CI) | ✅ |
| Contract | `descripcion` en perfil, exportación y persona | capturas regeneradas con `CONTRACT_CAPTURE=1`; suite completa verde en local | ✅ |
| Privacidad | La exportación ARCO incluye la descripción; el borrado de la cuenta la borra con el documento | `exportacion_descargar_200.json`; `account.Privacy.PurgeAccount` | ✅ |
| SDD | `check_sdd` OK | spec en Borrador con evidencia | ✅ |
