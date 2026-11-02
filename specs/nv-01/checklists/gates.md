# Checklist — nv-01 Foundation

> Trazabilidad: `spec R1-R6 ↔ plan §3-§5 ↔ task T1`. Estado: Borrador con evidencia (2026-11-02), sin firma.

- [x] Pipeline en el orden constitucional (R1), verificado por `TestPipeline_ConstitutionalOrder` y por critic B3
- [x] `/health` 200 y `/ready` con check real de las tres bases (R2) — `tests/integration/health_test.go`
- [x] `ping`, versionado estricto, `snake_case`, OpenAPI solo dev, sondas gateadas (R3) — `routing_test.go`, `errors_test.go`
- [x] Configuración solo con claves usadas; `JWT_KEY` corta falla al arrancar (R4) — `config_test.go`, `config_example_test.go` contra `config.Keys()`
- [x] `NewApp` reutilizable en tests; con Redis caído solo el limitador usa memoria, lo demás fail-closed (R5) — `ratelimit_test.go`, `roles_test.go` (revocación 503), `/ready` degraded
- [x] Layout sin carpetas de caos, rendimiento ni mutación; `deployments/docker-compose.yml` y `Dockerfile` creados (R6)
- [x] Tests: Unit 30 + Integration 14 + Security 8 verdes (`-count=1`; la spec pedía 27 + 13 + 6)
- [x] `go build` y `go vet` sin errores; `golangci-lint` 0 issues
- [x] `critic PASS`, `endpoints OK` (10 rutas), `SDD-OK` (4 specs), `semgrep 0`
- [x] `curl /ready` 200 con las tres bases y 503 sin MongoDB; `/health` trae `estado` y `version`
- [x] `make infra-up` (equivalente `docker compose up -d --wait`) levanta las tres bases y `docker build .` OK
- [x] `docs/endpoints.md` con las filas de nv-01
- [ ] Aprobación firmada: (pendiente de revisor)

## Evidencia (2026-11-02, Go 1.27.0 local, Docker 29.8.1, Windows + Git Bash)
- `go test ./internal/... ./cmd/... -count=1` → 30 PASS; `./tests/security/...` → 8 PASS; `./tests/integration/... -p 1` → 14 PASS con MongoDB 7.0.14, Redis 7.4.1 y Neo4j 5.26.0 reales en Testcontainers (puertos 27018/6380/7688). Sin contenedores huérfanos al terminar.
- `golangci-lint@v2.15.0 run ./...` → 0 issues. `govulncheck@v1.9.0` con Go 1.27.3 → 0 vulnerabilidades alcanzables, analizando como Windows y como Linux (v1.1.4 entraba en pánico en CI) (con el Go 1.27.0 local quedan 9 de la librería estándar, corregidas en 1.27.2/1.27.3).
- `semgrep 1.95.0 scan --config=auto --config=.semgrep/semgrep.yaml --error` → 0 (se fijaron las acciones de GitHub a SHA).
- `yaml.safe_load` OK en `ci-pr.yml`, `semgrep.yaml`, `.golangci.yml` y `docker-compose.yml`. `docker compose config` válido.
- Binario real (`-ldflags -X main.version=v0.1.0-nv01`, staging): `/health` 200 con HSTS y CSP, `/ready` 200 → 503 al detener MongoDB → 200 `degraded` al detener Redis, `/api/v1/ping` sigue en 200 sin Redis, `/ping` 404, token basura 401; el log no contiene la clave JWT.
- Imagen `wherever-api:nv01`: 25 MB, `nonroot`, sin shell; sin variables falla rápido; con variables `/health` y `/ready` 200 y la sonda da 404 en prod.
- Pruebas negativas del critic: renombrar una etapa del pipeline → B3 FAIL; cambiar una fila de `docs/endpoints.md` → `check_endpoints` FAIL en ambos sentidos.
