# gates.md — NV-16 Versión de la API y versión mínima de la app

| Gate | Criterio | Evidencia | Estado |
|---|---|---|---|
| Enmienda | §4 incluye la etapa `version` y el 426; §5 incluye `UpgradeRequired`; constitución 1.7.0 en los dos archivos | `.specify/memory/constitution.md` 1.7.0 (§4 pipeline, 304 y 426; §5 `UpgradeRequired→426`); `constitution.md` es un enlace al original y no cambia | ✅ |
| critic B1 | Sin secretos nuevos en Git ni Terraform | `critic-guardrails` en el host: sin FAIL de B1 (2026-11-12) | ✅ |
| critic B2 | Sin tokens ni datos personales en logs | `critic-guardrails` en el host: sin FAIL de B2 | ✅ |
| critic B3 | Pipeline igual a la lista constitucional, con `version` entre los límites y JWT | `critic-guardrails` sin FAIL de B3 + `TestPipeline_ConstitutionalOrder` verde | ✅ |
| critic B4 | Endpoints coinciden con `docs/endpoints.md` | `check_endpoints: OK (68 rutas)` en Docker (en el host falta Go) | ✅ |
| Configuración | `GET /api/v1/configuracion` 200 con los cinco campos, ETag débil y `Cache-Control: no-cache`; 304 con `If-None-Match` | `Unit/ConfiguraciónHTTP` verde; `curl` por CloudFront después de fusionar | 🟡 falta servidor |
| OpenAPI | La ruta aparece en `internal/http/openapi.json` (solo dev) | ruta y esquema `Configuracion` en `internal/http/openapi.json` | ✅ |
| 426 | Semver válido y menor en ruta de negocio = 426; igual o mayor, ausente o ilegible = pasa; fuera de `/api/v1` con versión vieja = 404 | `Unit/MiddlewareVersión` y `Unit/Versionado` verdes | ✅ |
| Exenciones | `/health`, `/ready`, `/api/v1/ping`, `/api/v1/configuracion` y `/api/v1/auth/*` no dan 426 ni 503 de mantenimiento | `Unit/Versionado` (`/health`, ping, configuración, auth) y `Unit/MiddlewareVersión`; `curl` después de fusionar | ✅ |
| Mantenimiento | `MODO_MANTENIMIENTO=true`: no exentas 503 con `Retry-After: 60`; configuración 200/304 | `Unit/MiddlewareVersión` + `Contract/Configuración`; prueba en el servidor después de fusionar | 🟡 falta servidor |
| Comparación | `1.10.0` se considera mayor que `1.9.0` | `TestVersionado_ComparaNumerico` | ✅ |
| CORS | El preflight permite `X-App-Version` e `If-None-Match`; la respuesta expone `ETag` | `TestCORS_PreflightPermiteVersionYCondicional` y `TestCORS_ExponeETag` | ✅ |
| Parámetros | Las cinco claves están en `/wherever/dev/` antes de instalar la imagen | las cinco claves tienen defecto; cargarlas en `/wherever/dev/` antes de instalar la imagen (operador) | 🟡 operador |
| Pruebas | Cinco suites verdes con servicios reales | build, vet, Unit y `golangci-lint` v2.15.0 (0) en local; Integration, Security, Database y Contract en el CI del PR (AGENTS §4) | 🟡 CI |
| Cobertura | Mínimo 45 % | `check_coverage` en el CI del PR | 🟡 CI |
| Semgrep | 0 hallazgos | job `semgrep` del CI del PR | 🟡 CI |
| SDD | `check_sdd` OK y `Memoria.md` actualizado | `SDD-OK (54 specs)`; `Memoria.md` actualizado | ✅ |
