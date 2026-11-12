# gates.md — NV-17 Paginación por cursor y validadores HTTP

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| Dependencia | `internal/shared/etag` de nv-16 existe y se reutiliza; no se duplica | `internal/ciudades/handler.go` usa `etag.De`/`Coincide`; `go build` verde | ✅ |
| Cursor | Ida y vuelta con ámbito; alterado, otra llave u otro ámbito → 422 `cursor`; vacío = primera página | `internal/shared/pagination/cursor_test.go` (ida y vuelta, vacío, 11 rechazos, otro ámbito) | ✅ |
| Firma | HMAC-SHA-256 con `CURSOR_HMAC_KEY`; `hmac.Equal`; el JSON se interpreta después de verificar | `cursor.go`: `hmac.Equal` antes de `parse`; `TestCursor_RejectedTokens` | ✅ |
| Ámbito | Un cursor de otra ruta, filtro o usuario no se acepta; `Position` devuelve 422 `cursor` | `TestPaginationQuery_PositionVerifiesSignatureAndScope`, `TestPlanes_CursorDeOtraPestana` (otra cuenta), `TestLugaresCursor_*` | ✅ |
| Forma | El validador solo comprueba Base64/JSON/ObjectId; `validation.New` conserva su firma y no conoce la llave | `validCursor` → `pagination.Valid`; `validation.New` sin cambios de firma; `TestValid_ChecksShapeOnly` | ✅ |
| Sin config nueva | No se toca `internal/platform/config`, `.env.example` ni SSM; `CURSOR_HMAC_KEY` entra con la primera lista posterior | Sustituido por el ajuste aprobado: `CURSOR_HMAC_KEY` obligatoria en config, `.env.example`, `deploy-env.sh`, `nightly.yml`; valor solo en SSM | ✅ (ajuste) |
| ETag catálogo | Autocompletar 200 con ETag débil y `public, max-age=60`; `If-None-Match` vigente → 304 sin cuerpo; cambio → 200 con ETag nuevo | `tests/integration/etag_test.go` (CI del PR); `curl` por CloudFront pendiente (T08) | CI |
| No-store | `/usuarios/yo` y `/auth/sesiones` con `Cache-Control: no-store`; el perfil nunca responde 304 | `TestETag_DatosDelUsuarioNoStore` (CI del PR) | CI |
| OpenAPI | Autocompletar documenta 304/ETag; las privadas documentan `no-store` | `openapi.json`: 304, `If-None-Match` y `ETag` en autocompletar; `no-store` en perfil y sesiones | ✅ |
| Endpoints | Sin rutas nuevas; `docs/endpoints.md` refleja las cabeceras y el 304 | `check_endpoints: OK (74 rutas)`; filas de sesiones, perfil y autocompletar y nota de cursor firmado | ✅ |
| Alcance | Sin lista de negocio ni consumidor de producción del `Signer`; el primero es la primera lista posterior | Drift: se migran `/lugares` y `/planes` (aprobado 2026-11-12); sin rutas nuevas | ✅ |
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails: PASS` | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails: PASS`; el ámbito usa hash con llave, sin ubicación ni id legibles | ✅ |
| critic B3 | Sin cambios de pipeline ni de infraestructura | `critic-guardrails: PASS`; `nightly.yml` solo suma una variable | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `critic-guardrails: PASS` | ✅ |
| Pruebas | Cinco suites verdes con servicios reales | Local: build, vet, Unit y `golangci-lint` v2.15.0 (0 issues). Integration/Security/Database/Contract en el CI del PR | CI |
| Cobertura | Mínimo 45 % | CI del PR | CI |
| Semgrep | 0 hallazgos | CI del PR | CI |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)`; `Memoria.md` al día | ✅ |
