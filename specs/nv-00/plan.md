# plan.md — NV-00 Constitución base

1. Mapeo
- Crear: `specs/nv-00/*`, `AGENTS.md §1`, `Memoria.md` inicial, `configs/.env.example` mínimo, `go.mod`, `Makefile`, `.golangci.yml`, `.gitignore`.
- Modificar: ninguno, es la primera spec.

2. Guardarraíles
- `critic B1` secretos `key/password/secret/uri con credenciales` en Git → FAIL.
- `critic B2` `token` en logs/errores → FAIL, única excepción el nombre de llave `jwt_revocado:{jti}`.
- `golangci-lint` con `gosec` y `errcheck` como error desde el día 1.
- `govulncheck` en CI desde el día 1.

3. Pruebas
- `internal/platform/config/config_example_test.go (3)`: `.env.example válido + sin patrones sensibles + sin claves futuras (STRIPE_*, FCM_*, AWS_*, PERF_*)`.
- Manual: `AGENTS.md §1` cita el stack, `Memoria.md` existe y cada NFR tiene su comando.

4. Secuencia
1. Fijar el stack en `spec.md` (tabla por capa).
2. Redactar gobernanza SDD + DoD por slice + trazabilidad + 1 revisor.
3. Fijar NFR con comando: `timeout 60s, body 10MB, go build, coverage >=45%`.
4. Registrar las reglas no negociables (`real-first`, sin globals).
5. Crear `AGENTS.md §1 + Memoria.md + .env.example` y verificar gates en vacío.

5. Verificación
- `go build ./... → sin errores`
- `go test ./internal/platform/config -count=1 → 3/3`
- `scripts/critic-guardrails.sh → PASS exit 0`
