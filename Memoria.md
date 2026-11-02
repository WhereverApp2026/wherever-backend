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
- Estado: T1 ejecutada. Creados `go.mod` (módulo `github.com/WhereverApp2026/Wherever-M-vil-2026`, `go 1.22`), `Makefile`, `.golangci.yml` (v2), `.gitignore`, `configs/.env.example`, `internal/platform/config/config_example_test.go` (3/3 verdes) y `scripts/critic-guardrails.sh` (PASS). Spec en Borrador con evidencia, sin firma.
- Decisión: ruta del módulo = remoto `origin`. Porqué: no había otra fuente; si el backend se muda a su propio repo hay que cambiarla.
- Decisión: `go 1.22` en `go.mod` aunque la máquina local tenga Go 1.27. Porqué: el stack pide 1.22+ y no hay que obligar al equipo ni a CI a la última versión.
- Decisión: `critic-guardrails.sh` nace en `nv-00` con B1/B2 activos y B3/B4 en `SKIP` mientras no haya `cmd/api/main.go` ni rutas; si aparecen sin que `nv-01` implemente la verificación, falla. Porqué: el criterio de `nv-00` exige critic PASS y no se vale un verde falso.
- Decisión: B2 busca el valor del token como argumento (quita los literales y busca el identificador) o como atributo de `slog`; un mensaje que solo dice "token inválido" pasa. Porqué: lo prohibido es filtrar el token, no nombrarlo.
- A evitar: golangci-lint < v2.15 con Go 1.27 falla con "export data version 4"; el `Makefile` lo fija con `go run ...@v2.15.0`.
- A evitar: gosec G304 con rutas armadas con `filepath.Join` en tests; usar constante.
- Pendiente: `scripts/check_coverage.sh` (nv-03), `govulncheck` y Semgrep en `ci-pr.yml` (nv-01). `make` no está instalado en esta máquina Windows; los comandos del `Makefile` se corrieron a mano.

## Pendientes de decidir
- Proveedor del catálogo de lugares (Google Places, Foursquare u OSM): licencias.
- Proveedor del modelo de lenguaje (API o modelo local).
- Ruta definitiva del módulo Go (hoy apunta al remoto `Wherever-M-vil-2026`).
- Nombre del revisor para firmar las specs (responsables en el Excel: Jorge Alberto Rodríguez Enriquez, hojas J; Alexander Dominguez Zarate, hojas A).
- Zona horaria por ciudad (documento de backend) contra `America/Mexico_City` fija (Excel): la constitución toma la zona de la ciudad con ese valor por defecto.

## Mapa de fases
- `nv-00` = constitución, stack y gobernanza · `nv-01` = foundation/pipeline/salud · `nv-02` = molde DTO→validator→service · `nv-03` = puerta de calidad incremental (advisory). Módulos de negocio desde `nv-04`, empezando por auth (J1) y usuarios (A1); el resto sigue el orden de producto del documento de backend: planes (A4), grupos (J3), social (J2), lugares (A2) y luego lo demás.
