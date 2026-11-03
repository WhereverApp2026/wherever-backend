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
- Deuda: `make` no está instalado en esta máquina; `TRUSTED_PROXIES` vacío hasta tener balanceador.

## Ejecución nv-02 (2026-11-02)
- Estado: T1 ejecutada en la rama `nv-02-molde` (sale de `nv-01-foundation`, aún sin merge). Paginación por cursor, registro único de validators (`go-playground/validator` v10.30.6), contrato `service.Lister[T]` y test de arquitectura; cero rutas nuevas. Unit 46 verdes, cobertura 48,3 %; lint, critic, endpoints, sdd y semgrep en 0. Spec en Borrador con evidencia, sin firma.
- Decisión: cursor = Base64 URL sin relleno de `{"_id","orden"}`; `_id` debe ser un ObjectId hexadecimal y se rechazan campos extra o basura. No va firmado. Porqué: es opaco pero no secreto; el service siempre filtra por dueño, así que un cursor forjado solo cambia el punto de inicio.
- Decisión: `NewPaginationQuery()` trae `limite=20` y el parser de query solo pisa lo que llega. Porqué: distinguir `limite` ausente (20) de `limite=0` explícito (422).
- Decisión: el validator vive en `Deps.Validator` y lo construye `cmd/api/main.go` con `validation.New()`, que falla al arrancar si una regla no se registra. Porqué: sin estado global y con el mismo validador para todos los handlers.
- Decisión: los nombres de campo en los errores salen de la etiqueta `query` o `json`. Porqué: el cliente ve `limite`/`cursor`, no los nombres de Go.
- Molde para `nv-04+`: el handler parsea a un DTO, llama `deps.Validator.Struct(dto)` y devuelve el error tal cual (422 con `campos`); el service se construye con sus dependencias y devuelve `apperr`; solo `repository.go` importa los drivers. `internal/arch/arch_test.go` lo vigila: handlers = archivos de `internal/http` (no middlewares) y `handler.go`/`*_handler.go` de cada módulo.
- A evitar: la cobertura de un test de otro paquete (p. ej. `internal/arch`) no cuenta para el paquete probado sin `-coverpkg`; por eso `validCursor` y `NewPageResult` salen en 0 % aunque se ejercitan.
- A evitar: semgrep no está instalado en esta máquina; correrlo con `docker run --rm -v "$(pwd -W):/src" -w /src semgrep/semgrep:1.95.0 ...` y `MSYS_NO_PATHCONV=1` en Git Bash.

## Ejecución nv-03 (2026-11-02)
- Estado: T2 ejecutada en la rama `nv-03-puerta` (sale de `nv-02-molde`; `main` ya tiene nv-01 y nv-02 por el merge del PR #3). Cobertura global bloqueante, delta y mutación advisory, en CI y en el `Makefile`. Spec en Borrador con evidencia, sin firma.
- Piloto (código de nv-02): cobertura global 79,6 % (Unit + Integration + Security con `-coverpkg`; solo Unit 56,1 %), delta 93,8 % (60/64 líneas), mutación 0,56 (23/41) en 107 s. Porqué se registra: el umbral de mutación no se fija hasta tener más slices medidos (§2.5); el 80 % de delta sigue informativo.
- Decisión: cada suite escribe su perfil con `-coverpkg=./internal/...,./cmd/...` y `check_coverage.sh` los une. Porqué: no repetir las suites para medir y contar lo que cubre un test de otro paquete (p. ej. `internal/arch`). `go tool cover` suma los bloques repetidos.
- Cifras de cobertura, para no confundirlas (las tres pasan el 45 %): el gate del CI une Unit + Integration + Security y midió 80,2 % en el PR #4 (Linux); en local la misma unión dio 79,6 %; `make cover` o `check_coverage.sh` sin perfiles mide solo Unit y da 56,1 %. Comparar siempre la cifra del CI con la unión, no con la de Unit.
- Primera corrida en CI (PR #4): `check_coverage` OK 80,2 %, 9 pruebas de scripts OK, delta vacío y mutación sin archivos (el PR solo cambió scripts y CI), como se esperaba.
- Go local actualizado a 1.27.3 (2026-11-02): `govulncheck` 0 vulnerabilidades y Unit verde.
- Decisión: `delta-check` y `mutation` son pasos del job `pipeline`, no jobs aparte. Porqué: respetan el orden §8 (antes de `semgrep`) y reutilizan `bin/coverage.out` sin artifacts.
- Decisión: go-mutesting = fork `avito-tech` con pseudo-versión fijada (no publica tags) y solo en `pull_request`. Porqué: el original de zimmski está abandonado; en `push` a main el delta es vacío.
- A evitar: go-mutesting deja `report.json` en la raíz aunque `json_output` sea false; `mutation.sh` lo borra y `.gitignore` lo ignora.
- A evitar: en go-mutesting "PASS" = mutante muerto (bueno) y "FAIL" = mutante vivo. El score solo usa los tests del paquete del archivo, así que lo que prueba `internal/arch` no cuenta.
- A evitar: Python en la consola de Windows imprime mal los acentos; el script reconfigura stdout a UTF-8.

## Ejecución nv-04 (2026-11-02)
- Estado: aprobada por Osmar Reséndiz el 2026-11-02 (PR #6), salvo T7. T1–T6 y T8 ejecutadas en la rama `nv-04-auth` (sale de `main` con nv-03). Once endpoints de J1, colecciones `sesiones` y `tokens_un_uso`, `usuarios` mínimo, mailer en memoria y de log, índices por script y suites Database y Contract nuevas. T7 (Google) diferida. Spec en Borrador con evidencia, sin firma.
- Decisión D-1 (pendiente, no tomada aquí): Google sin `fecha_nacimiento`. Hasta que el responsable de A1 (Alexander Dominguez Zarate) apruebe la propuesta de la spec, no hay `/auth/google` ni la dependencia `google.golang.org/api/idtoken` (excepción de stack, Apache-2.0, mantenida por Google) ni `GOOGLE_CLIENT_ID`.
- Decisión: llaves `jwt_revocado:{jti}` y `login_fallos:{correo}` aunque la spec nv-04 decía `blacklist:/attempts:/lockout:`. Porqué: la constitución §6 y la alineación del 2026-10-29 fijan esos nombres; nv-01 ya usa `jwt_revocado:`. El bloqueo es la misma llave con 5 o más y su TTL da los minutos del 423.
- Decisión: índices declarados por módulo (`Indexes()`), reunidos en `internal/indexes` y creados solo por `scripts/indexes` (`make indexes`). Porqué: constitución §5; el plan decía "en el arranque".
- Decisión: `sesiones` guarda `jti_vigente` (subido a la hoja J1 en la v3 del Excel junto con `refresh_hash_previo`; su vencimiento se calcula con `ultimo_uso` + vida del access, sin campo extra); al cerrar una, todas, por reutilización o al cambiar contraseña se revoca el access vigente de cada sesión, y al rotar el refresh el access anterior. Porqué: sin eso un teléfono "cerrado" seguía entrando hasta 15 min.
- Decisión: los módulos montan sus rutas con `httpapi.Module` (`Routes(v1, limiter)`) y reciben el validador por constructor; se quitó `Deps.Validator` de nv-02. Porqué: el handler necesita su service y el limitador para sus políticas por ruta; `internal/http` sigue sin importar drivers.
- Decisión: el service se prueba en `tests/integration` (bases reales) y el armado del módulo para pruebas va en `internal/testutil/authtest` (aparte de `testutil` para no crear un ciclo con los tests internos de `auth`). Security, Database y Contract usan `testutil.RunWithStack`.
- Decisión: login 5 por 5 min por IP; `refresh`, `recuperar`, `restablecer` y `verificar-correo` 10 por min por IP, cada una con su llave. Valor provisional (spec), medir antes de endurecer.
- Decisión: `202` sin cuerpo (`c.Status`), porque `SendStatus` escribe "Accepted" como texto.
- Deuda: proveedor real de correo (hoy `mailer.Log` en todos los entornos: los enlaces no salen); `MAIL_FROM` llega con él. Para nv-05: `auth.Service.CreateSession` y `SendVerification` son los puntos de entrada (su spec dice `CrearSesion`).
- A evitar: `require.Equal(..., readAll(t, resp))` consume el cuerpo antes de decodificarlo (EOF); leer primero y luego afirmar.
- A evitar: en Windows, Control de aplicaciones bloquea siempre el `cover.exe` de Go 1.27.3 recién instalado; la cobertura se mide en `golang:1.27.3` con `-v //var/run/docker.sock:/var/run/docker.sock --network host` (Testcontainers funciona desde el contenedor). No se cambió la política.
- A evitar: gosec G101 marca nombres como `tokens_un_uso` o `recuperar_password`; se anota `#nosec G101` con motivo y en `_test.go` se excluye por configuración.
- A evitar: heredocs largos con comillas en el Bash de la herramienta siguen fallando; escribir el archivo directo.
- A evitar: go-mutesting reemplaza el archivo original mientras prueba cada mutante (deja un `.go.tmp`). No hacer `git add`, compilar ni editar `.go` durante `scripts/mutation.sh`: aquí se llegó a stagear un mutante de `password.go`; se corrigió re-stageando tras la corrida.
- Decisión: `/ready` da 503 (`indices_mongodb: error`) si falta un índice declarado; lo verifica `mongo.IndexVerifier` sin crearlo. Porqué: si alguien arranca la API sin `make indexes`, sin `ux_correo` dos registros en carrera crearían cuentas duplicadas; el balanceador no le manda tráfico hasta que existan.
- Decisión: `scripts/mutation.sh` copia el árbol (versionados + nuevos no ignorados) a un directorio temporal y muta ahí; en PR solo `MUTATION_SCOPE` (por defecto `internal/auth/{service,tokens}.go` cambiados) y la completa en `.github/workflows/nightly.yml` (`--full`, 120 min). Línea base: 0,62 sobre los 18 archivos de nv-04.
- Límite conocido de la mutación: con `skip_without_test: true` go-mutesting solo muta archivos con un `_test.go` del mismo nombre, y solo corre las pruebas de su paquete. De 36 candidatos de `internal/` muta 12 (también en `--full`); salta `auth/service.go`, `handler.go`, `repository.go`, `usuarios/repository.go` y todo lo que se prueba en `tests/`. Por eso la línea base 0,62 solo mide 7 de los 18 archivos de nv-04 (`password`, `tokens`, `app`, `roles`, `config`, `mailer`, `registry`) y el 0,79 del alcance de PR solo mide `tokens.go`: ninguno dice nada de la calidad de las pruebas del service. Para medirlo: un `--exec` en el nightly que corra las pruebas de integración del paquete (lento). Un `service_test.go` con repositorios simulados daría el número pero va contra la regla de bases reales.
- Verificado (2026-11-02): la constitución del repo dice `jwt_revocado:{jti}` y `login_fallos:{correo}` desde el primer commit (980dff8), igual que la hoja J1 v3; el critic B2 no tiene excepción por `blacklist:`. `specs/nv-03` sin diferencias con `main`.
- Medición nv-04: cobertura unida 83,7 % (solo Unit 36,4 %; el gate de CI une las cinco suites), delta 81,4 %, mutación 0,62 en 31 min (línea base de 7 archivos, ahora nightly) y 0,79 en 3 min 43 s con el alcance de PR (solo `tokens.go`; ver el límite conocido).
- A evitar: `check_delta_coverage.py` en Windows leía `git diff` como cp1252 y caía con acentos; ahora fija UTF-8. Y medirlo siempre después de unir los cinco perfiles: `check_coverage.sh` sin perfiles sobrescribe `bin/coverage.out` solo con Unit.

## Ejecución nv-05 (2026-11-03)
- Estado: T1–T8 ejecutadas en la rama `nv-05-usuarios` (sale de `main` con nv-04). 16 endpoints de A1, `usuarios` completo con sus 7 índices, `ciudades` con importación, `exportaciones` y el trabajo nocturno de privacidad. Aprobada por Osmar Reséndiz el 2026-11-03 (PRs #7 y #8; firma aparte porque los dos ya estaban fusionados).
- Decisión (revisor, 2026-11-03): D-2 = "mis datos" por enlace de un solo uso de 24 h (`descargar_exportacion` en `tokens_un_uso`) y `POST /usuarios/exportaciones/descargar`. Porqué: no mandar datos personales completos por correo. El JSON vive en `exportaciones.datos_json` solo hasta que se descarga: la descarga lo borra en la misma operación (`TakeReady`, estado `descargada`, `descargada_en`) y el documento queda como registro del límite de una por día hasta que lo borra `ttl_exportacion`.
- Límite conocido: `datos_json` va dentro del documento y un documento de MongoDB no pasa de 16 MB. Hoy solo exportan `usuarios` y `sesiones` (pocos KB); cuando planes, reseñas, check-ins y postales registren su `Exporter` puede crecer. Antes de eso: guardar el archivo en S3 (o GridFS) y dejar en `exportaciones` solo la referencia.
- Decisión (revisor): `gocron` v2.22.0 (MIT) como excepción de stack; corre dentro de la API a las 03:00 de America/Mexico_City con el candado `lock:borrado_nocturno` (SET NX, TTL 30 min, se suelta con script que compara el token). Se usó v2.22.0 y no v2.23.0 porque esta tenía una semana de publicada.
- Decisión: D-1 sigue pendiente; nv-05 no agrega `perfil_completo`.
- Decisión: `internal/usuarios` = modelo y repositorio; `internal/usuarios/account` = service y handler. Porqué: auth usa el repositorio de usuarios y el service de usuarios usa a auth (abrir sesión, verificación, cerrar sesiones); en un solo paquete sería un ciclo.
- Decisión: versión vieja en `If-Match` = 409 y sin cabecera = 400 (la spec decía 412/428). Porqué: la constitución §5 no tiene esos códigos y define el choque de versión como 409.
- Decisión: una cuenta en eliminación pendiente puede iniciar sesión (`User.CanSignIn`). Porqué: al pedir la eliminación se cierran todas las sesiones y entrar es la única forma de cancelar; los tokens de antes sí quedan inválidos. Refuerzo pedido por el revisor: con la cuenta pendiente solo responden `GET /usuarios/yo` y `cancelar-eliminacion`; el resto de rutas con JWT dan 403 con `usuarios.RequireActive`, que lee el estado en cada petición protegida. `logout` y `logout-todos` quedan abiertos (2026-11-03, revisor): cerrar sesión no cambia el estado de la cuenta y la persona debe poder hacerlo.
- Decisión: el código de amigo lo genera el repositorio (31 caracteres sin 0/O/1/I/L, 8 de largo, `crypto/rand`) y reintenta hasta 5 veces si choca con `ux_codigo_amigo`. Porqué: toda cuenta lo trae, también las que crea auth en pruebas; el índice no es parcial.
- Decisión: `jti_expira_en` no existe; nv-04 calcula el vencimiento con `ultimo_uso` + vida del access.
- Decisión: ARCO por registro. Orden del borrado: sesiones → exportaciones → usuario (el documento del usuario al final; si algo falla la cuenta sigue pendiente y se reintenta la noche siguiente). Exportan `usuarios` (perfil sin hashes) y `sesiones` (dispositivo, IP y fechas).
- Excel v3 actualizado (2026-11-03): `PUT /usuarios/yo/gustos` ya dice que no suma puntos y que los 50 se dan al registrarse (hojas A1 y Resumen endpoints; respaldo previo en el scratchpad). El total de `Índice!G27` (=SUM) se recalcula al abrir.
- Pendiente de subir al Excel (hoja A1): colección `exportaciones` y sus 3 índices, tipo `descargar_exportacion`, ruta de descarga, `privacidad.buscable_por_telefono` y los catálogos provisionales de movilidad, tema y unidades.
- Deuda: `Purger`/`Exporter` de planes, check-ins, reseñas y señales llegan con sus módulos; hoy "Borrar mi historial" no borra nada porque no existe historial. Proximidad en el autocompletado de ciudades. Proveedor de correo (sin él, ni la verificación ni el enlace de exportación salen).
- Decisión: el teléfono se normaliza antes de validar (`account.NormalizePhone` quita espacios, guiones, puntos y paréntesis) y lo que quede debe ser E.164; sin `+` y lada del país es 422. Porqué: la spec pide "se normaliza a E.164" y el mismo número escrito distinto debe dar el mismo HMAC (si no, `ux_telefono_hash` no detecta el duplicado).
- Spec nv-05 actualizada por el revisor (2026-11-03) con los 16 endpoints, el guardia de eliminación pendiente, D-2 aprobada y el límite de 16 MB. Se agregaron las pruebas que pedía y faltaban: PATCH concurrente con la misma versión (gana una), dos instancias procesan cada cuenta una sola vez, el enlace de descarga vence a las 24 h y el archivo se borra al descargarlo (en Security).
- A evitar: insertar un usuario sin `codigo_amigo` choca en `ux_codigo_amigo` con el segundo que tampoco lo tenga (el índice no es parcial); insertar siempre con `Repository.Create` o con código.
- A evitar: el validador de `gtefield` reporta el nombre del campo de Go; el mensaje es fijo ("mayor o igual que el mínimo").

## Pendientes de decidir
- Proveedor del catálogo de lugares (Google Places, Foursquare u OSM): licencias.
- Proveedor del modelo de lenguaje (API o modelo local).
- D-1: cuenta nueva por Google sin `fecha_nacimiento` (propuesta en `specs/nv-04/spec.md`; dueño A1).
- Proveedor de correo (SMTP o servicio) y su remitente (`MAIL_FROM`).
- Firma de `nv-00…nv-03`: siguen en Borrador con evidencia. Desde nv-04 el revisor es Osmar Reséndiz (único integrante de backend; el Excel nombra a Jorge Alberto Rodríguez Enriquez para las hojas J y a Alexander Dominguez Zarate para las A como responsables del diseño).
- Zona horaria por ciudad (documento de backend) contra `America/Mexico_City` fija (Excel): la constitución toma la zona de la ciudad con ese valor por defecto.

## Mapa de fases
- `nv-00` = constitución, stack y gobernanza · `nv-01` = foundation/pipeline/salud · `nv-02` = molde DTO→validator→service · `nv-03` = puerta de calidad incremental (advisory). Módulos de negocio desde `nv-04`, empezando por auth (J1) y usuarios (A1); el resto sigue el orden de producto del documento de backend: planes (A4), grupos (J3), social (J2), lugares (A2) y luego lo demás.
