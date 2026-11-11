# plan.md — NV-17 Paginación por cursor y validadores HTTP

1. Mapeo
- Crear: `tests/integration/etag_test.go`; el resto vive en paquetes existentes.
- Modificar: `internal/shared/pagination/{cursor.go,cursor_test.go,cursor_fuzz_test.go}`, `internal/shared/validation/{registry.go,pagination_query.go,registry_test.go}`, `internal/arch/arch_test.go` (firma de `Position`), `internal/ciudades/handler.go`, `internal/usuarios/account/handler.go`, `internal/auth/handler.go`, `internal/http/openapi.json`, `docs/endpoints.md`, `Memoria.md`.
- Sin cambios: `internal/shared/etag` (se reutiliza de nv-16), el pipeline y `internal/http/app.go`, `validation.New` (conserva su firma), `internal/platform/config`, `cmd/api`, `internal/testutil`, las políticas de rate limit, la base, los índices, Redis y los endpoints. No hay rutas ni claves de configuración nuevas.

2. Guardarraíles
- El cursor usa HMAC-SHA-256 con `CURSOR_HMAC_KEY`; la comparación es `hmac.Equal`, la firma se verifica antes de interpretar el JSON y el JSON es estricto (`DisallowUnknownFields`). `_id` debe ser ObjectId y `ambito` no vacío.
- El validador solo comprueba la forma; `PaginationQuery.Position` es el único punto que convierte el cursor en posición y el que devuelve 422 `cursor`. Ningún service lee `q.Cursor` directamente y no hay estado global.
- `CURSOR_HMAC_KEY` no se agrega a la configuración en este corte: no hay lista que la use y sería una clave obligatoria sin consumidor. La primera lista posterior la agrega a config, `.env.example` y SSM antes de desplegarse.
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
- `go test ./internal/shared/pagination/...` demuestra ida y vuelta, manipulación, ámbito y fuzz. No hay ruta de lista todavía, así que el 422 de cursor se prueba en Unit; el primer consumidor real es la primera lista posterior.
- Por CloudFront: `GET /api/v1/ciudades/autocompletar?q=guada` 200 con `ETag` y `Cache-Control: public, max-age=60`; repetir con `If-None-Match` da 304 sin cuerpo; `GET /api/v1/usuarios/yo` y `GET /api/v1/auth/sesiones` 200 con `Cache-Control: no-store`.
- Confirmar que no se agregó ninguna clave de configuración ni parámetro de SSM en este corte.
