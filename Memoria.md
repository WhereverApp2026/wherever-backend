# Memoria — Wherever Backend

> Constitución §10. Por fase o slice: estado, decisiones con porqué, errores a evitar. Máx 10000 líneas. Sin datos sensibles.

## Fase nv-00 — Estructura SDD (2026-10-28)
- Estado: estructura SDD creada (`.specify/memory/constitution.md` canónico + espejo raíz + templates + specs `nv-00…nv-03` + docs vivos + `.claude/` con 5 sub-agentes y skills). **Tareas sin ejecutar.**
- Decisión: canónico en `.specify/memory` (estándar Spec Kit) + espejo en raíz. Porqué: compatibilidad con herramientas y visibilidad.
- Decisión: backend en Go + Fiber, stateless multi-instancia (≥2), MongoDB + Neo4j + Redis. Porqué: requisitos del curso (NoSQL y grafos, balanceador, Redis).
- Decisión: JSON `snake_case`, colecciones en español plural. Porqué: coherencia con la app móvil y el equipo.
- Decisión: etapa recortada a arquitectura base. Porqué: pedido del docente; la funcionalidad llega con `nv-04+`.
- Deuda: observabilidad completa (`/metrics`, OTel, Grafana), Trivy/ZAP/RESTler, Terraform/AWS, CD, mutación como gate.
- A evitar: hash falso, token opaco, bases simuladas en integración, workaround sin issue, módulos de negocio en `nv-00…nv-03`.

## Alineación con el contrato de producto (2026-10-29)
- Estado: constitución 1.1.0; specs `nv-00…nv-03`, docs, skills y agentes alineados con `Wherever_Endpoints_CRUD.xlsx` y los dos documentos de diseño (app y backend). Sigue sin código; tareas **sin ejecutar**.
- Decisión: el Excel es el contrato de negocio y cada spec `nv-04+` copia sus filas a `docs/endpoints.md`. Porqué: el equipo ya lo diseñó por pantalla; evita inventar rutas.
- Decisión: llaves de Redis con los nombres del Excel (`jwt_revocado:`, `login_fallos:`, `rate:`) en lugar de `blacklist:`/`attempts:`/`lockout:`. Porqué: un solo vocabulario entre backend y documentación.
- Decisión: con Redis caído solo el rate limit usa memoria por instancia; revocados, bloqueo, idempotencia y candados son fail-closed (503). Porqué: en memoria cada instancia vería un estado distinto y un token revocado seguiría sirviendo en otra.
- Decisión: `/ready` sigue en 200 `degraded` sin Redis, aunque la hoja J9 dice 503 si falla cualquiera. Porqué: Redis es compartido; con 503 el balanceador sacaría a todas las instancias a la vez. Confirmado por el usuario el 2026-10-29. Falta corregir la fila `/ready` de la hoja J9 del Excel (dueño: Jorge Alberto Rodríguez Enriquez).
- Decisión: códigos 410/423/503 y `traza_id` en los errores; roles `consumidor/comerciante/administrador`; JWT 15 min + refresh 30 días; `APP_ENV`; Docker Compose y `Dockerfile` en `nv-01`.
- Deuda que piden los documentos y queda diferida: `/metrics`, Loki, alertas, `POST /api/v1/errores`, SonarCloud, SBOM CycloneDX, Trivy/Dockle/Cosign, RESTler/ZAP, Pact, caos con Toxiproxy/k6, quality gate GO/STOP, cola de tareas, servicio de ML, WebSocket, Terraform/AWS.
- A evitar: guardar en memoria del proceso cualquier estado de seguridad; usar los nombres de llaves anteriores.

## Ejecución nv-00 (2026-11-02)
- Estado: T1 ejecutada. Creados `go.mod` (módulo `github.com/WhereverApp2026/wherever-backend`, `go 1.22`), `Makefile`, `.golangci.yml` (v2), `.gitignore`, `configs/.env.example`, `internal/platform/config/config_example_test.go` (3/3 verdes) y `scripts/critic-guardrails.sh` (PASS). Spec en Borrador con evidencia, sin firma.
- Decisión: ruta del módulo = `github.com/WhereverApp2026/wherever-backend`. Porqué: GitHub informó que el repositorio se movió de `Wherever-M-vil-2026` a `wherever-backend`; se cambiaron módulo y remoto antes de que nv-01 agregue imports internos.
- Decisión: `go 1.22` en `go.mod` aunque la máquina local tenga Go 1.27. Porqué: el stack pide 1.22+ y no hay que obligar al equipo ni a CI a la última versión.
- Decisión: `critic-guardrails.sh` nace en `nv-00` con B1/B2 activos y B3/B4 en `SKIP` mientras no haya `cmd/api/main.go` ni rutas; si aparecen sin que `nv-01` implemente la verificación, falla. Porqué: el criterio de `nv-00` exige critic PASS y no se vale un verde falso.
- Decisión: B2 busca el valor del token como argumento (quita los literales y busca el identificador) o como atributo de `slog`; un mensaje que solo dice "token inválido" pasa. Porqué: lo prohibido es filtrar el token, no nombrarlo.
- A evitar: golangci-lint < v2.15 con Go 1.27 falla con "export data version 4"; el `Makefile` lo fija con `go run ...@v2.15.0`.
- A evitar: gosec G304 con rutas armadas con `filepath.Join` en tests; usar constante.
- Pendiente: `scripts/check_coverage.sh` (nv-03), `govulncheck` y Semgrep en `ci-pr.yml` (nv-01). `make` no está instalado en esta máquina Windows; los comandos del `Makefile` se corrieron a mano.

## Ejecución nv-01 (2026-11-02)
- Estado: T1 ejecutada en la rama `nv-01-foundation` (sale de `nv-00-constitucion`, aún sin merge). Pipeline, salud, config, errores, limitador, JWT configurado, compose, Dockerfile, CI y scripts. Unit 30 + Integration 14 + Security 8 verdes; lint, critic, endpoints, sdd y semgrep en 0. Spec en Borrador con evidencia, sin firma.
- Decisión: `go 1.26` mínimo en `go.mod` (sube desde 1.22). Porqué: Testcontainers v0.44 lo exige. CI y el builder de Docker usan el último parche de 1.27 para que govulncheck no marque la librería estándar.
- Decisión: Fiber v2 aunque está marcado como deprecado en favor de v3. Porqué: la constitución §3 fija Fiber v2; migrar a v3 requiere enmienda.
- Decisión: etapa `request_timeout` (plazo de 60 s en el contexto) entre `request_id` y `proxy_headers`. Porqué: el NFR de timeout necesita un lugar en el pipeline; no altera el orden constitucional. El critic B3 compara la lista exacta.
- Decisión: la etapa `errors` también escribe la línea de acceso. Porqué: es la única que ve el código final de la respuesta (el error se traduce ahí).
- Decisión: JWT global y opcional: sin encabezado la petición sigue anónima, un token presente e inválido siempre es 401, y las rutas protegidas usan `RequireRoles`. La revocación `jwt_revocado:{jti}` es fail-closed (503). Porqué: hoy no hay rutas protegidas y `ping`/`health` son públicas.
- Decisión: limitador de escrituras por IP mientras no exista auth. Porqué: corre antes de JWT (constitución §4); con auth se puede llavear por usuario. Login 5/5min y Admin 200/min llegan con sus módulos.
- Decisión: tras un fallo de Redis el limitador no lo reintenta durante 5 s y go-redis queda con 1 reintento de conexión. Porqué: con los 5 reintentos por defecto cada petición tardaba ~0,5 s más con Redis caído.
- Decisión: credenciales de Neo4j en la URI y Redis acepta `redis://`. Porqué: no sumar claves de entorno; dev corre sin credenciales en 127.0.0.1.
- Decisión: `check_sdd.sh` exige 7 secciones `## ` más `## Aprobación`, sin nombres fijos. Porqué: nv-00/nv-01 no siguen los nombres de `.specify/templates/spec-template.md` y nv-02/nv-03 sí. Pendiente: unificar.
- Decisión: puertos del compose configurables (`MONGO_HOST_PORT`, etc.). Porqué: esta máquina ya tiene un servicio MongoDB en 27017.
- A evitar: en Docker Desktop para Windows Ryuk no arranca; `tests/integration` lo desactiva solo en Windows y apaga los contenedores en `TestMain`. En CI (Linux) sigue activo.
- A evitar: Windows (Control de aplicaciones) bloquea al azar algunos `*.test.exe`; reintentar o compilar con `go test -c -o bin/x.test.exe`.
- A evitar: heredocs largos con comillas en el Bash de la herramienta se rompen; escribir el archivo directo.
- A evitar: `semgrep --config=auto` no corre con `--metrics=off`.
- A evitar: govulncheck v1.1.4 (x/tools v0.29) entra en pánico con `unexpected expr: *ast.KeyValueExpr` al analizar como Linux con Go 1.27; en Windows pasaba porque se compilan otros archivos. Usar ≥ v1.9.0 y, para reproducir CI en local, instalar la herramienta nativa y correrla con `GOOS=linux`.
- Deuda: Go local 1.27.0 con 9 vulnerabilidades de la librería estándar (actualizar a ≥1.27.3); `make` no está instalado en esta máquina; `TRUSTED_PROXIES` vacío hasta tener balanceador.

## Ejecución nv-02 (2026-11-02)
- Estado: T1 ejecutada en la rama `nv-02-molde` (sale de `nv-01-foundation`, aún sin merge). Paginación por cursor, registro único de validators (`go-playground/validator` v10.30.6), contrato `service.Lister[T]` y test de arquitectura; cero rutas nuevas. Unit 46 verdes, cobertura 48,3 %; lint, critic, endpoints, sdd y semgrep en 0. Spec en Borrador con evidencia, sin firma.
- Decisión: cursor = Base64 URL sin relleno de `{"_id","orden"}`; `_id` debe ser un ObjectId hexadecimal y se rechazan campos extra o basura. No va firmado. Porqué: es opaco pero no secreto; el service siempre filtra por dueño, así que un cursor forjado solo cambia el punto de inicio.
- Decisión: `NewPaginationQuery()` trae `limite=20` y el parser de query solo pisa lo que llega. Porqué: distinguir `limite` ausente (20) de `limite=0` explícito (422).
- Decisión: el validator vive en `Deps.Validator` y lo construye `cmd/api/main.go` con `validation.New()`, que falla al arrancar si una regla no se registra. Porqué: sin estado global y con el mismo validador para todos los handlers.
- Decisión: los nombres de campo en los errores salen de la etiqueta `query` o `json`. Porqué: el cliente ve `limite`/`cursor`, no los nombres de Go.
- Molde para `nv-04+`: el handler parsea a un DTO, llama `deps.Validator.Struct(dto)` y devuelve el error tal cual (422 con `campos`); el service se construye con sus dependencias y devuelve `apperr`; solo `repository.go` importa los drivers. `internal/arch/arch_test.go` lo vigila: handlers = archivos de `internal/http` (no middlewares) y `handler.go`/`*_handler.go` de cada módulo.
- A evitar: la cobertura de un test de otro paquete (p. ej. `internal/arch`) no cuenta para el paquete probado sin `-coverpkg`; por eso `validCursor` y `NewPageResult` salen en 0 % aunque se ejercitan.
- A evitar: semgrep no está instalado en esta máquina; correrlo con `docker run --rm -v "$(pwd -W):/src" -w /src semgrep/semgrep:1.95.0 ...` y `MSYS_NO_PATHCONV=1` en Git Bash.

## Pendientes de decidir
- Proveedor del catálogo de lugares (Google Places, Foursquare u OSM): licencias.
- Proveedor del modelo de lenguaje (API o modelo local).
- Nombre del revisor para firmar las specs (responsables en el Excel: Jorge Alberto Rodríguez Enriquez, hojas J; Alexander Dominguez Zarate, hojas A).
- Zona horaria por ciudad (documento de backend) contra `America/Mexico_City` fija (Excel): la constitución toma la zona de la ciudad con ese valor por defecto.

## Mapa de fases
- `nv-00` = constitución, stack y gobernanza · `nv-01` = foundation/pipeline/salud · `nv-02` = molde DTO→validator→service · `nv-03` = puerta de calidad incremental (advisory). Módulos de negocio desde `nv-04`, empezando por auth (J1) y usuarios (A1); el resto sigue el orden de producto del documento de backend: planes (A4), grupos (J3), social (J2), lugares (A2) y luego lo demás.
