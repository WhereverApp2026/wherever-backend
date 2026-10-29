# NV-00 — Constitución base

## T1 — Reglas y stack sin módulos
- **Crear**: `specs/nv-00/spec.md`, `plan.md`, `task.md`, `AGENTS.md §1`, `Memoria.md` inicial, `configs/.env.example` con `APP_ENV=dev, MONGO_URI=__MONGO__, REDIS_ADDR=__REDIS__, NEO4J_URI=__NEO4J__, JWT_KEY=PLACEHOLDER_MIN_32_BYTES, JWT_ISSUER=wherever-api, JWT_AUDIENCE=wherever-app, CORS_ALLOWED_ORIGIN=`.
- **Crear**: `internal/platform/config/config_example_test.go (3)`, `go.mod`, `Makefile`, `.golangci.yml` con `gosec + errcheck + staticcheck + gocyclo(10)`, `.gitignore`.
- **Verificar**: `go build ./... sin errores + ConfigTest 3/3 + critic PASS + cada NFR con su comando`.
- **No hacer**: módulos de negocio, Stripe, FCM, OTel, Terraform, hosting, entidades, endpoints de negocio.
