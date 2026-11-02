# NV-01 — Foundation + pipeline final + bases externas

## Contexto
Casa vacía pero con luz. Base ejecutable una sola vez, ya en forma de producción, sin módulos de negocio. **Depende de**: `NV-00`.

## Requisitos
1. Pipeline final día 1: `SecurityHeaders outermost → ErrorHandler → RequestID → ProxyHeaders → CORS 1 origen → RateLimiter antes de Auth → JWT → Roles → Handlers`.
2. `GET /health` liveness: 200 siempre con `{estado:"ok", version}` y sin consultar bases (`version` entra por `-ldflags` al compilar). `GET /ready` con check real de MongoDB, Redis y Neo4j, 1 s máximo cada una: `{estado: ok|degraded|error, dependencias:{mongodb, redis, neo4j}}`; 503 si cae MongoDB o Neo4j, 200 `degraded` si cae Redis; sin trazas ni cadenas de conexión en la respuesta. Contrato tomado de la hoja J9 del Excel de endpoints.
3. `GET /api/v1/ping → {estado:"pong"} + versionado estricto, sin versión → 404 + JSON snake_case + OpenAPI solo dev + ErrorResponse{codigo,mensaje,campos,detalle solo no-prod,traza_id} + sondas probe/{timeout,error,forbidden,conflict,invalid} gateadas no-prod`.
4. Config solo usada: `APP_ENV (dev|staging|prod; decide sondas, OpenAPI, HSTS y detalle), MONGO_URI, REDIS_ADDR, NEO4J_URI por env, JWT_KEY>=32B solo env + JWT_ISSUER/JWT_AUDIENCE configurados aunque sin usuarios, RATE_LIMIT_*, CORS_ALLOWED_ORIGIN, BODY_LIMIT 10MB, REQUEST_TIMEOUT 60s, PORT`.
5. La app se construye en `NewApp(cfg, deps)` para que los tests la reutilicen. Si Redis cae, solo el rate limit pasa a memoria por instancia (con aviso en el log); revocados, bloqueo, idempotencia y candados son fail-closed (`503`), según constitución §2.1.
6. Layout: `cmd/api + internal/{platform,http,shared,testutil} + tests/{integration,security} + deployments/docker-compose.yml (MongoDB, Redis, Neo4j con puertos fijos) + Dockerfile multi-etapa`. Sin carpetas de caos, rendimiento ni mutación. CI `build→critic→endpoints→semgrep` verde en vacío.
> Nota: el orden CI de este paso está acotado a nv-01; el canónico completo está en `ci-pr.yml` (`AGENTS.md §1` / `constitution.md §8`).

## Diseño
- `main.go` llama a `NewApp` con el orden fijo constitucional. `RateLimiter + Headers + JWT` ya configurados aunque sin roles usados: es base, no módulo.
- `SecurityHeaders`: `nosniff, DENY, strict-origin-when-cross-origin, X-XSS-Protection 0, CSP default-src 'none'`, exención de `/docs` en dev.
- `ErrorHandler`: mapeo de constitución §5 (`400/401/403/404/408/409/410/422/423/429/503`, otro → 500), `detalle` nulo en prod, `traza_id` siempre presente, sin `recover` en handlers.
- Redis con `DialTimeout/ReadTimeout` cortos (≤500ms). Con Redis caído la API sigue respondiendo: el limitador usa memoria por instancia y lo avisa en el log; nada de seguridad se guarda en memoria.
- `deployments/docker-compose.yml`: MongoDB, Redis y Neo4j con versiones fijas, puertos fijos y volúmenes; solo para desarrollo y tests. `Dockerfile` según la skill `docker-multistage` (usuario sin privilegios, sin `.env`).
- Cero cadenas de conexión fijas.

## Contratos
- `GET /health, GET /ready, GET /api/v1/ping, GET /api/v1/probe/* no-prod, /docs + /openapi.json solo dev`.
- Claves de configuración canónicas: `APP_ENV, MONGO_URI, REDIS_ADDR, NEO4J_URI, JWT_KEY, JWT_ISSUER, JWT_AUDIENCE, CORS_ALLOWED_ORIGIN, BODY_LIMIT, REQUEST_TIMEOUT`.

## Tests
> Conciliado con lo implementado (2026-11-02). Conteo de funciones `Test*`; las tablas cuentan como una.
- `Unit/Config` (`internal/platform/config/config_test.go`, 4): carga válida con valores por defecto, falta clave obligatoria → error, `JWT_KEY` corta o marcador → error, valores inválidos (APP_ENV, CORS con comodín o dos orígenes, cuerpo >10MB, timeout >60s, rate relajado en prod). Más `config_example_test.go` (3) de nv-00, ahora contra `config.Keys()`.
- `Unit/Middleware` (`security_headers_test.go`, 8): headers base, CSP estricta, `/docs` y `/openapi.json` exentas solo en dev, headers puestos antes de `next` (también en errores), `request_id` único, entrante válido conservado, entrante inválido reemplazado.
- `Unit/Errors` (`errors_test.go`, 8, con la tabla de 11 mapeos de §5): otro → 500 sin filtrar la causa, `campos` nunca `null` y `detalle` omitido si vacío, `detalle` oculto en prod, visible fuera de prod, campos de validación + `Retry-After`, pánico → 500, ruta inexistente → 404 uniforme.
- `Unit/RateLimit` (`ratelimit_test.go`, 3): `429` uniforme con `Retry-After`; con Redis caído el limitador sigue en memoria y avisa una sola vez; escrituras concurrentes sobre el límite → `429`.
- `Unit/Roles+JWT` (`roles_test.go`, 3): anónimo → 401, rol desconocido falla al construir, revocación fail-closed (`503`) con Redis caído.
- `Unit/Pipeline` (`internal/http/app_test.go`, 1): orden constitucional.
- `Integration/Routing` (`tests/integration/routing_test.go`, 3): v1 resuelve + `snake_case`, sin versión o mal escrita 404, docs dev 200 / staging y prod 404.
- `Integration/Health` (`health_test.go`, 5): `/health 200` con `estado` y `version` aunque las bases estén caídas, `/ready 200` con las tres, `503` sin MongoDB (sin cadenas de conexión en la respuesta), `200 degraded` sin Redis, `503` sin Neo4j.
- `Integration/Errors` (`errors_test.go`, 3) con `APP_ENV=staging`: 404/403/409/422/408/500 reales con `traza_id`, `detalle` visible en staging, en prod sondas 404 y sin `detalle`.
- `Integration/Auth+Rate` (`auth_ratelimit_test.go`, 3): token válido pasa y revocado en Redis → 401, rol equivocado → 403, límite global compartido entre dos instancias sobre el mismo Redis y con TTL.
- `Security` (`tests/security/security_test.go`, 8): `/health` anónimo con headers, `/probe/forbidden` 403 con headers, HSTS 365d en prod, `alg:none` → 401, sin firma → 401, vencido → 401, basura/sin Bearer/firma alterada → 401, CORS solo para el origen configurado.

## Criterios
- `go build ./... → sin errores` y `go vet ./... → limpio`.
- `go test ./... -count=1 → 100% verdes en vacío`.
- `curl /health 200 + /ready 200 con las tres bases, 503 sin MongoDB`.
- `scripts/critic-guardrails.sh → PASS exit 0`.
- `docker compose -f deployments/docker-compose.yml config → válido`, `make infra-up` levanta las tres bases y `docker build .` termina sin errores.
- `scripts/check_endpoints.sh → OK exit 0` solo `ping/probe/health/docs`.
- `semgrep scan --config=auto --config=.semgrep/semgrep.yaml --error → 0 nuevos`.
> Nota: `Database/Contract` se activan con el primer módulo; en vacío `CONTRACT_CAPTURE=0`.

## Límites
Sin colecciones de negocio, sin roles usados, sin Stripe/FCM/OTel/`/metrics` completo. JWT configurado pero sin emisión: la emisión llega con el módulo auth (`nv-04+`). Sin hosting: `go build` y `docker build` listos, no desplegados.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (ejecutada, sin firma)
- **Revisores:** (pendiente de nombre)
- **Fecha:** 2026-11-02 (ejecución)
- **Detalle:** T1 ejecutada. Evidencia en `specs/nv-01/checklists/gates.md` + `Memoria.md`. Unit 30 + Integration 14 + Security 8 verdes. Desvíos documentados: etapa `request_timeout` entre `request_id` y `proxy_headers`; limitador de escrituras por IP hasta que exista auth; puertos del compose configurables.
