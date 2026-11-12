# plan.md — NV-17 Paginación por cursor y validadores HTTP

1. Mapeo
- Crear: `tests/integration/etag_test.go`; el resto vive en paquetes existentes.
- Modificar: `internal/shared/pagination/{cursor.go,cursor_test.go,cursor_fuzz_test.go}`, `internal/shared/validation/{registry.go,pagination_query.go,registry_test.go}`, `internal/arch/arch_test.go` (firma de `Position`), `internal/ciudades/handler.go`, `internal/usuarios/account/handler.go`, `internal/auth/handler.go`, `internal/http/openapi.json`, `docs/endpoints.md`, `Memoria.md`.
- Migrar (drift, aprobado 2026-11-12): `internal/negocios/{service.go,lugares_service.go,lugares_dto.go,lugares_repository.go,horario_test.go}`, `internal/planes/{service.go,dto.go,dto_test.go}`, `tests/integration/lugares_test.go`; `CURSOR_HMAC_KEY` en `internal/platform/config/{config.go,config_test.go}`, `internal/testutil/{testutil.go,authtest/authtest.go}`, `cmd/api/main.go` (`buildBusinesses`), `configs/.env.example`, `infra/aws-dev/{README.md,scripts/deploy-env.sh}` y `.github/workflows/nightly.yml` (solo la variable del contenedor de ZAP).
- Sin cambios: `internal/shared/etag` (se reutiliza de nv-16), el orden del pipeline y `internal/http/app.go`, `validation.New` (conserva su firma), las políticas de rate limit, la base, los índices, Redis y los endpoints.

2. Guardarraíles
- El cursor usa HMAC-SHA-256 con `CURSOR_HMAC_KEY`; la comparación es `hmac.Equal`, la firma se verifica antes de interpretar el JSON y el JSON es estricto (`DisallowUnknownFields`). `_id` debe ser ObjectId y `ambito` no vacío.
- El validador solo comprueba la forma; `PaginationQuery.Position` es el único punto que convierte el cursor en posición y el que devuelve 422 `cursor`. Ningún service lee `q.Cursor` directamente y no hay estado global.
- `CURSOR_HMAC_KEY` es obligatoria en config porque `/lugares` y `/planes` la usan; el operador la carga en SSM y nunca entra a Git. `buildModules` no gana otro `if err` (gocyclo 10): el `Signer` se arma en `buildBusinesses`.
- El ETag se calcula sobre una sola serialización y es débil; `If-None-Match` acepta `W/`, listas y `*`; el 304 no escribe cuerpo. `no-store` solo en datos del usuario.
- No hay caché nueva: el 304 no evita la consulta a Mongo y Redis no se toca (nv-18).

3. Secuencia
1. Aprobar la spec y las seis decisiones (T01).
2. Firma del cursor, `Valid` y `Position` (T02, T03).
3. ETag y `no-store` (T04, T05).
4. OpenAPI, `docs/endpoints.md` y `Memoria.md` (T06).
5. Suites y gates (T07).
6. Despliegue y comprobación por CloudFront (T08).

4. Verificación
- `go build ./...` y `go vet ./...`.
- Unit, Integration, Security, Database y Contract con `-count=1`; cobertura ≥ 45 %.
- `critic-guardrails.sh` PASS; `check_sdd.sh` SDD-OK; `check_endpoints.sh` OK sin rutas nuevas; Semgrep en 0.
- `go test ./internal/shared/pagination/...` demuestra ida y vuelta, manipulación, ámbito y fuzz. El 422 de cursor se prueba en Unit y en `Integration/Lugares`.
- Por CloudFront: `GET /api/v1/ciudades/autocompletar?q=guada` 200 con `ETag` y `Cache-Control: public, max-age=60`; repetir con `If-None-Match` da 304 sin cuerpo; `GET /api/v1/usuarios/yo` y `GET /api/v1/auth/sesiones` 200 con `Cache-Control: no-store`.
- Confirmar que `/wherever/dev/CURSOR_HMAC_KEY` existe antes de desplegar; `GET /lugares?limite=1` y su `siguiente_cursor` dan la página 2.
