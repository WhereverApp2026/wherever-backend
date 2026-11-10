# plan.md — NV-16 Versión de la API y versión mínima de la app

1. Mapeo
- Crear: `internal/platform/versionado/{version.go,version_test.go}`, `internal/http/middleware/version.go` (+ `version_test.go`), `internal/http/middleware/cors_test.go`, `internal/shared/etag/{etag.go,etag_test.go}`, `internal/configuracion/{handler.go,service.go,dto.go,handler_test.go}`, `tests/contract/configuracion_contract_test.go` y `tests/contract/testdata/configuracion/*.json`.
- Modificar: `internal/platform/config/config.go` (+ `config_test.go`, `config_example_test.go`), `internal/shared/apperr/apperr.go` (+ `apperr_test.go`), `internal/testutil/testutil.go` (defectos de `Config`), `internal/testutil/authtest/authtest.go` (módulo de configuración en la app de pruebas), `internal/http/middleware/cors.go`, `internal/http/app.go` (+ `app_test.go`), `internal/http/openapi.json`, `cmd/api/main.go`, `scripts/routes/main.go`, `scripts/critic-guardrails.sh`, `infra/aws-dev/README.md` (encender y apagar el mantenimiento), `configs/.env.example`, `.specify/memory/constitution.md` y `constitution.md`, `docs/endpoints.md`, `Memoria.md`.
- Sin cambios: `/health`, `/ready`, las políticas de rate limit, JWT, las bases y los índices; `usuarios.privacidad.version_terminos` y la aceptación de términos (nv-19).

2. Guardarraíles
- Servicios reales en las pruebas, sin secretos en Git, sin datos personales en logs, roles validados en cada endpoint y cada ruta nueva en `docs/endpoints.md`.
- `X-App-Version` solo bloquea con un semver válido y menor; un valor ausente, vacío o ilegible se ignora.
- El 426 es compatibilidad, no autorización; nunca se usa para decidir acceso a un recurso.
- La etapa nueva no reordena las existentes: entra después de los límites y antes de JWT, y solo actúa sobre `/api/v1/`.
- El cuerpo del 426/503 sale del manejador uniforme de errores, con `campos: []` y `traza_id`; no se escribe JSON a mano en la etapa.
- La respuesta de configuración se serializa una sola vez: el ETag y el cuerpo salen de los mismos bytes.

3. Secuencia
1. Aprobar la spec y la enmienda §4/§5 (1.7.0).
2. Configuración y errores tipados (T03, T04).
3. `versionado`, ETag y etapa del pipeline (T05, T06).
4. Módulo de configuración y registro de rutas (T07).
5. Documentos y parámetros del servidor (T08).
6. Pruebas y gates (T09, T10).

4. Verificación
- `go build ./...` y `go vet ./...`.
- Unit, Integration, Security, Database y Contract con `-count=1`; cobertura ≥ 45 %.
- `critic-guardrails.sh` PASS con la lista nueva del pipeline; `check_sdd.sh` SDD-OK; `check_endpoints.sh` OK con la fila de `/api/v1/configuracion`; Semgrep en 0.
- En el servidor de desarrollo (CloudFront): `GET /api/v1/configuracion` 200 con ETag y 304 con `If-None-Match`; `X-App-Version` menor en una ruta de negocio 426 y en `/api/v1/auth/login` 200; con `MODO_MANTENIMIENTO=true`, aplicado con el paso manual de Systems Manager, una ruta de negocio 503 con `Retry-After: 60` y la configuración 200; volver a `false` al terminar.
- Confirmar que las cinco claves están en `/wherever/dev/` antes de instalar la imagen.
