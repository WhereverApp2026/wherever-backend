# task.md — NV-16 Versión de la API y versión mínima de la app

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec y la enmienda de la constitución §4/§5 (1.7.0); rama `nv-16`.

## Fase 1 — Implementación
- [ ] T02 Enmienda §4/§5: etapa `version`, código `426`, lista del pipeline igual al código real; actualizar `scripts/critic-guardrails.sh` (B3) y `TestPipeline_ConstitutionalOrder`.
- [ ] T03 Configuración: claves `APP_VERSION_MINIMA`, `APP_VERSION_ACTUAL`, `TERMINOS_VERSION`, `AVISO_VERSION` y `MODO_MANTENIMIENTO` con defectos y validación al arrancar; `.env.example`.
- [ ] T04 Errores: `apperr.KindUpgradeRequired` (426, `actualizacion_requerida`) y reutilizar `Unavailable` con `RetryAfter` para el 503 de mantenimiento.
- [ ] T05 `internal/platform/versionado`: `Version` (semver numérico, `v` opcional) y `Politica` con rutas exentas y precedencia del mantenimiento.
- [ ] T06 Etapa `version` en `internal/http/middleware/version.go`, conectada entre `rate_limit_writes` y `jwt` y solo sobre `/api/v1/`; 426 y 503 con el cuerpo uniforme. CORS: permitir `X-App-Version` e `If-None-Match` y exponer `ETag`.
- [ ] T07 `internal/configuracion`: `GET /api/v1/configuracion` con los cinco campos, ETag débil, `Cache-Control: no-cache` y 304; registrar el módulo en `cmd/api` y en `scripts/routes`.
- [ ] T08 Documentos: fila y nota en `docs/endpoints.md`, ruta en `internal/http/openapi.json`, claves en `/wherever/dev/`, procedimiento de mantenimiento en `infra/aws-dev/README.md`, `Memoria.md`; actualizar la línea de paquetes de nv-17 para que reutilice `internal/shared/etag`.

## Fase 2 — Cierre
- [ ] T09 Pruebas: `Unit/Versionado`, `Unit/Configuración`, `Unit/ConfiguraciónHTTP`, `Unit/MiddlewareVersión`, `Unit/ETag` y `Contract/Configuración` con sus casos de 426, exenciones, mantenimiento y 304.
- [ ] T10 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`, cobertura ≥ 45 %, Semgrep en 0; probar en el servidor de desarrollo (426, 304 y 503) y anotar la evidencia en `gates.md`; PR.
