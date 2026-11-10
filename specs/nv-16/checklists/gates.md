# gates.md — NV-16 Versión de la API y versión mínima de la app

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| Enmienda | §4 incluye la etapa `version` y el 426; §5 incluye `UpgradeRequired`; constitución 1.7.0 en los dos archivos | diff de `.specify/memory/constitution.md` y `constitution.md` | pendiente |
| critic B1 | Sin secretos nuevos en Git ni Terraform | salida de `critic` | pendiente |
| critic B2 | Sin tokens ni datos personales en logs | salida de `critic` | pendiente |
| critic B3 | Pipeline igual a la lista constitucional, con `version` entre los límites y JWT | salida de `critic` + `TestPipeline_ConstitutionalOrder` | pendiente |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints` | pendiente |
| Configuración | `GET /api/v1/configuracion` 200 con los cinco campos, ETag débil y `Cache-Control: no-cache`; 304 con `If-None-Match` | pruebas y `curl` por CloudFront | pendiente |
| OpenAPI | La ruta aparece en `internal/http/openapi.json` (solo dev) | diff del archivo + `/openapi.json` | pendiente |
| 426 | Semver válido y menor en ruta de negocio = 426; igual o mayor, ausente o ilegible = pasa; fuera de `/api/v1` con versión vieja = 404 | `Unit/MiddlewareVersión` | pendiente |
| Exenciones | `/health`, `/ready`, `/api/v1/ping`, `/api/v1/configuracion` y `/api/v1/auth/*` no dan 426 ni 503 de mantenimiento | `Unit/MiddlewareVersión` + `curl` | pendiente |
| Mantenimiento | `MODO_MANTENIMIENTO=true`: no exentas 503 con `Retry-After: 60`; configuración 200/304 | prueba y `curl` por CloudFront con el paso manual de Systems Manager, luego volver a `false` | pendiente |
| Comparación | `1.10.0` se considera mayor que `1.9.0` | `Unit/Versionado` | pendiente |
| CORS | El preflight permite `X-App-Version` e `If-None-Match`; la respuesta expone `ETag` | `Unit/CORS` | pendiente |
| Parámetros | Las cinco claves están en `/wherever/dev/` antes de instalar la imagen | `aws ssm get-parameter` por clave (el deployer no tiene `ssm:DescribeParameters`) | pendiente |
| Pruebas | Cinco suites verdes con servicios reales | salida de `go test` | pendiente |
| Cobertura | Mínimo 45 % | `check_coverage` | pendiente |
| Semgrep | 0 hallazgos | salida de `semgrep` | pendiente |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `check_sdd` | pendiente |
