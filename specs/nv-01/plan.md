# plan.md — NV-01 Foundation + pipeline + bases externas

1. Mapeo
- Modificar: `cmd/api/main.go` con `NewApp`, `Makefile` (`infra-up`, `infra-down`, `build`, `test`), `.golangci.yml`, `configs/.env.example`, `.gitignore`, `.github/workflows/ci-pr.yml`.
- Crear: `internal/http/app.go`, `internal/http/middleware/{security_headers,request_id,errors,ratelimit,jwt,roles}.go`, `internal/http/health.go`, `internal/http/ping.go`, `internal/http/probe.go`, `internal/platform/{config,logger,mongo,redis,neo4j}`, `internal/testutil`, `docs/endpoints.md` inicial, `scripts/{critic-guardrails,check_endpoints,check_sdd}.sh`, `.semgrep/semgrep.yaml`, `deployments/docker-compose.yml`, `Dockerfile`, `.dockerignore`.
- Registrar: respaldo en memoria solo para el rate limit, 0 `cache:{entidad}`.

2. Guardarraíles
- Orden constitucional fijo: `SecurityHeaders outermost → ErrorHandler → RequestID → ProxyHeaders → CORS → RateLimiter antes de Auth → JWT configurado → Roles`.
- `SecurityHeaders`: `nosniff + DENY + strict-origin-when-cross-origin + X-XSS-Protection 0 + CSP default-src 'none'`; HSTS 365d solo no-dev.
- `ErrorHandler`: mapeo de constitución §5 `+ detalle nulo en prod + traza_id + sin recover en handlers`.
- `JWT` configurado con `clave>=32B por env + leeway 0 + métodos [HS256] + iss/aud` aunque sin emisión.
- `RateLimiter` Global 1000/min + Login 5/5min + Admin 200/min + Writes 10 concurrentes, con `429` uniforme y `Retry-After`; Redis con tiempos cortos; respaldo en memoria por instancia solo para el limitador.

3. Pruebas
- Las de `spec.md §Tests`: Unit (Config 3, Middleware 8, Errors 14, RateLimit 2), Integration (Routing 3, Health 4, Errors 6), Security (Headers 3, Jwt 3).

4. Secuencia
1. `go.mod + Makefile + .golangci.yml + .env.example + .gitignore + semgrep.yaml + deployments/docker-compose.yml`.
2. `config + logger` y `NewApp` con `SecurityHeaders` outermost.
3. `ErrorHandler + probe` gateado.
4. `Routing v1 + ping + OpenAPI solo dev`.
5. `Health + conexiones (mongo/redis/neo4j) + RateLimiter + JWT configurado + CORS + HSTS`.
6. `docs/endpoints.md + ci-pr.yml + scripts + Dockerfile` y gates en vacío.

5. Verificación
- `go mod download; go build ./... → sin errores; go vet ./... → limpio`
- `go test ./... -count=1 → 100% verdes en vacío`
- `scripts/critic-guardrails.sh → PASS`
- `scripts/check_endpoints.sh → OK`
- `semgrep scan --config=auto --config=.semgrep/semgrep.yaml --error → 0 nuevos`
- `curl /health 200 + /ready 200/503 + /api/v1/ping pong`
- `docker compose -f deployments/docker-compose.yml config` y `docker build .` sin errores
