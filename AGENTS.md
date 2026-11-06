# Agents.md — Wherever Backend

> Rector: `.specify/memory/constitution.md` manda ante conflicto. Espejo: `./constitution.md`.
> Este archivo dice **cómo** trabajar. La constitución dice **qué** no se negocia.

## 1. Forma de trabajo
> Stack: ver `constitution.md §3`. Nada fuera sin excepción documentada.
- Setup: `configs/.env.example → .env` (`APP_ENV=dev`); `MONGO_URI`, `REDIS_ADDR`, `NEO4J_URI` apuntan a las bases de `deployments/docker-compose.yml` (`make infra-up`, `make infra-down`); `go mod download`. Docker solo para infraestructura de desarrollo, tests y la imagen (`Dockerfile`).
- Comandos: `go mod download`, `go build ./...`, `go test ./... -count=1` por suite (Unit, Integration, Security, Database, Contract), `go run ./cmd/api`, `go build -o bin/api ./cmd/api`.
- CI `ci-pr.yml` (ubuntu-latest, Testcontainers exige Linux), jobs en paralelo: changes, guardrails (critic → endpoints → sdd), semgrep, go-security, image-scan, pipeline (deps → build → unit → integration → security → database → contract → coverage → delta-check → mutation) y pr-gate. Sin deploy. Un cambio solo de documentos omite go-security, image-scan y pipeline.
- PR = 1 slice (módulo); `CHECKLIST_PR.md` + 1 revisor + `Memoria.md` al día.
- Naming: inglés en código; colecciones en español plural minúsculas; campos `snake_case`; JSON `snake_case`.
- Sin secretos, sin TODO sin issue, sin imports muertos, sin código muerto.

## 2. Memoria
- Plantilla canónica `.specify/templates/spec-template.md` (alias `specs/_template.md`), DoD por módulo en `nv-04+`.
- `AGENTS.md`: manual operativo vivo.
- `Memoria.md`: estado, decisiones y lecciones aprendidas por fase. Actualizar al cerrar cada fase.
- Al empezar, lee `Memoria.md` para conocer el estado del proyecto y las decisiones tomadas.
- Al terminar una tarea, actualízalo: estado actual, decisiones importantes (con su porqué) y errores a evitar.
- Al cerrar una fase, concilia su `spec.md`/`plan.md` con lo implementado (rutas de tests, criterios, nombres reales de archivos) y resuelve el bloque de aprobación: Borrador con evidencia, o firmado solo si el usuario indicó revisor.
- Mantenlo breve (máximo ~10000 líneas): resume o elimina lo que ya no aporte.
- Si algo se convierte en regla permanente, propón moverlo a `AGENTS.md`.
- No guardes nunca datos sensibles (claves, tokens, datos personales).

## 3. Límites — no hacer
- No bases simuladas para negocio en integración; solo mocks de interfaces externas (Stripe, FCM, S3, LLM).
- No estado global sin `Reset()` y mutex.
- No hardcodear connection strings ni llaves.
- Secretos locales solo en `.env` (git-ignored); la imagen nunca los contiene.
- No hash débil: Argon2id 64MB/3iter. Sin MD5/SHA simple para contraseñas.
- No tocar el pipeline sin el orden de la constitución §4.
- No relajar los límites de peticiones en prod.
- No leer configuración de forma eager en el arranque de tests con overrides: la configuración se resuelve por constructor.
- No duplicar tablas vivas: lo canónico está en `docs/`.
- No incluir módulos de negocio en `nv-00…nv-03`.

## 4. Verificación por módulo
- `go build ./...` y `go vet ./...` sin errores antes de test.
- `Unit/Integration/Security/Database/Contract` 100% verdes (`-count=1`).
- `scripts/critic-guardrails.sh → PASS`, `scripts/check_sdd.sh → SDD-OK`, `scripts/check_endpoints.sh → OK`, `scripts/check_coverage.sh --min 45`, `check_delta_coverage.py --min 80 --advisory` informa (no bloquea), `semgrep scan --metrics=off --error --severity=ERROR --config=p/golang --config=p/owasp-top-ten --config=p/secrets` y `semgrep scan --metrics=off --error --config=.semgrep/semgrep.yaml` 0 nuevos.
- Mutación PR advisory (solo archivos cambiados, `continue-on-error`) + nightly opcional; tras ella siempre `go build ./...`.
- Contract con `CONTRACT_CAPTURE=1` para regenerar; sin la variable solo valida.
- `curl /ready` 200 con las tres bases, 503 si cae MongoDB o Neo4j o si falta un índice declarado (correr `make indexes`); Redis caído = 200 `degraded`.
- Mutación: `scripts/mutation.sh` trabaja en una copia temporal del árbol, nunca en el de trabajo; en PR solo `MUTATION_SCOPE` (service y tokens de auth), la completa en `nightly.yml`.

## 5. Convenciones técnicas
- Rutas `/api/v1/<recurso>`; DTOs + validator; `version` e `If-Match` en Update/Delete.
- Health `/health` y `/ready`. OpenAPI solo en desarrollo. Sin `/metrics` completo por ahora.
- Claves de Redis (constitución §6): `jwt_revocado:{jti}` (TTL = vida remanente del token), `login_fallos:{correo}` (TTL 15 min), `rate:{politica}:{sujeto}`, `idem:{llave}`, `lock:{recurso}`, `cuota:{usuario}:{fecha}`. Todas con TTL. La contraseña nunca va a caché.
- Comandos de Redis (nv-11): Upstash Free da 500 000 al mes y cobra `MULTI`/`EXEC`. El limitador usa `Pipeline` y `/health` no pasa por él. `TestPresupuestoRedis_*` fija el costo por operación: si sube, rehaz la estimación de `specs/nv-11`. No uses `/ready` como chequeo frecuente: gasta cuota.
- Redis caído: solo el rate limit usa memoria por instancia; revocados, bloqueo, idempotencia y candados son fail-closed (`503`).
- Roles: `consumidor`, `comerciante`, `administrador`. "Dueño" = rol + el recurso es suyo (se valida en el service).
- Contratos de negocio: `Wherever_Endpoints_CRUD.xlsx` (hojas A1–A7, J1–J9); cada spec `nv-04+` copia sus filas a `docs/endpoints.md` antes de programar.
- Índices: script idempotente en `scripts/indexes`.
- El primer módulo de auth entrega `AdminPolicy`/roles; los demás módulos la consumen. Un solo `auth` handler por slice (login + logout).
- Molde `nv-02`: `shared/pagination` (cursor) + validators espejo registrados por una sola función + `service` por constructor.
- OpenAPI de un solo v1.

## 6. Seguridad DevSecOps
- JWT HS256 real siempre (sin flag legacy), access 15 min; `jwt_revocado:{jti}` revisado en el middleware.
- IP del cliente: entrada de `X-Forwarded-For` a `TRUSTED_PROXY_HOPS` del final (1 con Caddy, 2 con CloudFront y balanceador); si no alcanza o no es IP, la conexión. Si cambia la cadena de proxies, cambia este valor.
- Headers outermost + HSTS 365d no-dev + CORS 1 origen.
- Rate 4 políticas + 429 uniforme + limitador antes de Auth.
- Timeout 60s + body 10MB + `slog` con filtro de secretos.
- SAST Semgrep, gosec y govulncheck bloqueantes; imagen con Dockle y Trivy bloqueantes (nv-09); ZAP pasivo cada noche; Sonar informativo; `Dependabot` activo. `RESTler` diferido, documentado, no exigido.

## 7. Testing quirks
- `testing` + `testify`; mocks solo de interfaces externas; Integration y Security usan la app real en proceso.
- `Security` reutiliza el helper `internal/testutil` (tokens válidos/vencidos/sin firma).
- `scripts/ci/cambios.sh` decide qué jobs corren y `resumen-gate.sh` escribe el gate del PR. Dockle y Trivy se fijan por digest en `ci-pr.yml`; ZAP por versión en `nightly.yml`.
- Fuzz (nv-10): los objetivos `Fuzz*` corren como pruebas normales con sus semillas; `scripts/ci/fuzz.sh` hace el fuzz real de noche. Un fallo deja un caso en `testdata/fuzz/`: se versiona y se corrige en el mismo PR. No usan repositorios simulados: `authtest.Build` con dependencias caídas.
- Testcontainers con puerto fijo; suites con contenedor nunca en paralelo local (colisionan). Desde nv-04 Integration, Security, Database y Contract levantan contenedores: cada una con `-p 1` y una a la vez (`testutil.RunWithStack`). Desde nv-06 el stack incluye Mailpit (SMTP 1026, API 8026) y `testutil.Mailpit` lee los correos.
- Correo (nv-06): las peticiones encolan con `mailer.Async` y nunca esperan al servidor; el trabajo nocturno envía directo con `mailer.Retrying`. Enlaces siempre con el token en el fragmento (`#token=`).
- Paquetes de usuarios (nv-05): `internal/usuarios` es modelo, repositorio y el guardia `RequireActive`; `internal/usuarios/account` es service, handler y DTO. Van separados porque auth usa el repositorio de usuarios y el service de usuarios usa a auth. El test de arquitectura trata `internal/usuarios/account/handler.go` como handler: no puede importar drivers ni usar `recover`.
- El service de un módulo se prueba en `tests/integration` contra bases reales (no en `internal/<modulo>`, que es Unit sin Docker); el armado del módulo para pruebas va en `internal/testutil/<modulo>test`.
- Los tests de integración que reinician un contenedor deben fijar el puerto del host; si no, el puerto cambia y la conexión queda obsoleta.
- El proceso local se mata por nombre del binario, no por `go`.
- JSON `snake_case` medido; sin mezclas.

## 8. Observabilidad mínima
- `slog` JSON + consola, sin secretos. `request_id` por petición. El logger oculta llaves sensibles, correos y credenciales de URI en cualquier texto.
- Logs en CloudWatch (nv-13): los filtros de métricas de `infra/aws-dev/observabilidad.tf` dependen de `level`, `estado`, `duracion_ms`, `dependencia` y `evento`. Renombrar un campo apaga su alarma en silencio. Detalle en `docs/observabilidad.md`.
- Sin OTel/Prometheus/Grafana completos por ahora.

## 9. Operación manual — sin hosting aún
- Bases de desarrollo con `make infra-up`; índices con `scripts/indexes`; esquema y demostración de Neo4j con `make siembra`.
- Servidor de AWS: Redis en Upstash y Neo4j en Aura (nv-11). Antes de una demostración, comprueba que Aura no esté en pausa.
- Imagen lista con `docker build`, no desplegada. Cuando haya hosting se añade runbook con variables y verificación post-subida.
- Una sola instancia local para desarrollo; la multi-instancia se valida en la spec de infraestructura.

## 10. Lecciones
- Fuente: `Memoria.md`. Si se vuelve regla, mover a `AGENTS.md`.

## 11. Sub-agentes
- `foundation-builder` ejecuta `nv-00/nv-01`; `slice-scaffolder` desde `nv-04`; `security-reviewer` gate sin editar; `traceability-clerk` reporta drift sin editar; `readme-keeper` edita solo `**/*.md` (`bash: deny`).
- En CI solo el `critic` ligero con bloqueantes B1-B4.

## Solution structure
| Carpeta | Tipo |
|---|---|
| `cmd/api` | Punto de entrada de la API (`main.go`) |
| `cmd/worker` | Trabajos programados (posterior) |
| `internal/platform` | Config, logger, Mongo, Redis, Neo4j |
| `internal/http` | Servidor Fiber, middlewares, errores, rutas base |
| `internal/shared` | Paginación por cursor, validación |
| `internal/<modulo>` | Un módulo por funcionalidad (`nv-04+`); si su service usa a otro módulo que a su vez usa su repositorio, el service va en un subpaquete (`internal/usuarios/account`) |
| `internal/privacidad` | ARCO: registro de `Purger`/`Exporter` y trabajo nocturno (`nv-05`) |
| `internal/testutil` | Helpers de prueba (tokens, datos, contenedores) |
| `tests/` | integration, security, database, contract |
| `scripts/` | critic, endpoints, sdd, coverage, delta, indexes |
| `deployments/` | Docker compose de desarrollo; Terraform (posterior) |
| `docs/` | endpoints, rate-limit-matrix, asvs |
| `.golangci.yml`, `.semgrep/` | Análisis estático |
