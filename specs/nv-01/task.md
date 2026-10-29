# NV-01 — Foundation + pipeline + bases externas

## T1 — Casa vacía con luz, sin módulos
- **Modificar**: `main.go` con el orden constitucional, `Makefile` con `infra-up/infra-down`, `.golangci.yml`, `configs/.env.example` solo claves usadas, `.gitignore`, `ci-pr.yml` con `deps→build→unit→integration→security→critic→endpoints→semgrep`.
- **Crear**: `middleware/security_headers.go` con `CSP + exención de /docs en dev`, `errors.go` con `WriteError + detalle omitido en prod`, `request_id.go`, `ping.go`, `probe.go` gateado `no-prod`, `health.go` con `/health` y `/ready`, `docs/endpoints.md` con `ping/probe/health/docs`, `.semgrep/semgrep.yaml`, `scripts/critic-guardrails.sh + check_endpoints.sh + check_sdd.sh`, `deployments/docker-compose.yml` (MongoDB, Redis, Neo4j, puertos fijos), `Dockerfile` multi-etapa + `.dockerignore`.
- **Crear tests**: `Unit/Config 3 + Middleware 8 + Errors 14 + RateLimit 2 + Integration/Routing 3 + Health 4 + Errors 6 + Security/Headers 3 + Jwt 3`.
- **Verificar**: `build + vet + Unit/Security/Integration 100% en vacío + critic PASS + endpoints OK + semgrep 0 + /ready 200/503 + compose válido + docker build OK`.
- **No hacer**: colecciones de negocio, roles usados, emisión JWT, lockout, caché de entidades, Stripe, FCM, OTel, hosting.
