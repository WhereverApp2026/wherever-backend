# gates.md — NV-17 Paginación por cursor y validadores HTTP

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| Dependencia | `internal/shared/etag` de nv-16 existe y se reutiliza; no se duplica | `go build` + diff | pendiente |
| Cursor | Ida y vuelta con ámbito; alterado, otra llave u otro ámbito → 422 `cursor`; vacío = primera página | `Unit/Cursor` | pendiente |
| Firma | HMAC-SHA-256 con `CURSOR_HMAC_KEY`; `hmac.Equal`; el JSON se interpreta después de verificar | revisión de `cursor.go` + pruebas | pendiente |
| Ámbito | Un cursor de otra ruta, filtro o usuario no se acepta; `Position` devuelve 422 `cursor` | `Unit/Validación` | pendiente |
| Forma | El validador solo comprueba Base64/JSON/ObjectId; `validation.New` conserva su firma y no conoce la llave | diff + `Unit/Validación` | pendiente |
| Sin config nueva | No se toca `internal/platform/config`, `.env.example` ni SSM; `CURSOR_HMAC_KEY` entra con la primera lista posterior | diff | pendiente |
| ETag catálogo | Autocompletar 200 con ETag débil y `public, max-age=60`; `If-None-Match` vigente → 304 sin cuerpo; cambio → 200 con ETag nuevo | `Integration/ETag` + `curl` por CloudFront | pendiente |
| No-store | `/usuarios/yo` y `/auth/sesiones` con `Cache-Control: no-store`; el perfil nunca responde 304 | `Integration/ETag` + `curl` | pendiente |
| OpenAPI | Autocompletar documenta 304/ETag; las privadas documentan `no-store` | diff de `openapi.json` | pendiente |
| Endpoints | Sin rutas nuevas; `docs/endpoints.md` refleja las cabeceras y el 304 | `check_endpoints` | pendiente |
| Alcance | Sin lista de negocio ni consumidor de producción del `Signer`; el primero es la primera lista posterior | revisión de la spec + `go build` | pendiente |
| critic B1 | Sin secretos nuevos en Git ni Terraform | salida de `critic` | pendiente |
| critic B2 | Sin tokens ni datos personales en logs | salida de `critic` | pendiente |
| critic B3 | Sin cambios de pipeline ni de infraestructura | salida de `critic` | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints` | pendiente |
| Pruebas | Cinco suites verdes con servicios reales | salida de `go test` | pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` | pendiente |
| Semgrep | 0 hallazgos | salida de `semgrep` | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `check_sdd` | pendiente |
