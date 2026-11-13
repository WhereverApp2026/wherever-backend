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

## Ejecución nv-06 (2026-11-03)
- Estado: T1–T8 ejecutadas en la rama `nv-06-correo` (sale de `main` con nv-05 cerrada, tras fusionar el PR #8). Adaptador SMTP con la librería estándar, cola en segundo plano, tres plantillas en español, Mailpit en el compose y en las pruebas, aviso de exportación con reintento. Aprobada por Osmar Reséndiz el 2026-11-03 (PR #9).
- Decisión (ajustes del revisor antes de ejecutar): `MAIL_FROM` nuevo (en nv-04 no existía) y obligatorio con `SMTP_HOST`; `Name` en `mailer.Message` para el saludo; en prod la API no arranca sin `SMTP_HOST`, con `SMTP_TLS=none` o con `APP_LINK_BASE` sin `https://`; `SMTP_USER` con `SMTP_TLS=none` falla en cualquier entorno.
- Decisión: dos caminos de correo. Las peticiones usan `mailer.Async` (cola acotada, nunca bloquea, descarta y cuenta si se llena) sobre `mailer.Retrying` (2 intentos, 1 s, `MAIL_TIMEOUT` por intento); el trabajo nocturno usa `mailer.Retrying` directo porque necesita saber si el aviso salió (`auth.Service.WithSyncMailer`). Al apagar, la cola se vacía antes de cerrar las bases.
- Decisión (revisor): cada reintento del aviso de exportación invalida el enlace anterior (`usado_en`) y crea uno nuevo, porque solo guardamos el hash. Tope de 72 h desde `lista_en` (el TTL la borra aunque el aviso no salga); las 24 h de descarga cuentan desde `aviso_enviado_en`, sin pasar de las 72 h.
- Decisión: la autenticación SMTP se niega sin TLS aunque el servidor sea localhost (`smtp.PlainAuth` sí la mandaría ahí). Los errores del envío no llevan la respuesta cruda del servidor; el log solo dice tipo, destinatario enmascarado, resultado ("tiempo agotado", "sin conexión", "rechazado") y duración.
- Para la spec móvil: los enlaces de los correos llevan el token en el **fragmento** (`{APP_LINK_BASE}/verificar#token=…`, `/restablecer#token=…`, `/exportacion#token=…`). La página o la app debe leerlo de `location.hash` (o del intent del App Link), no de la consulta, y mandarlo por `POST` a la API (`verificar-correo`, `restablecer`, `exportaciones/descargar`). Así el token no queda en los logs del servidor que sirve la página ni en el `Referer`. No usar `wherever://` en producción: otra app podría registrar el mismo esquema e interceptar el token.
- Mailpit fijado en `axllent/mailpit:v1.21.8`; en pruebas SMTP 1026 y API 8026 (dev: 1025 y 8025).
- Deuda: proveedor sin elegir (SES, Resend o Brevo; SMTP lo hace cambio de configuración), dominio propio con SPF/DKIM/DMARC, Android App Links, aviso de privacidad con el proveedor como encargado (LFPDPPP), plantillas de marca, rebotes y quejas. Pendiente de subir al Excel: `exportaciones.aviso_enviado_en`.
- Decisión: `// nosemgrep` en el import de `text/template` de `render.go`, con su motivo. Porqué: la regla de Semgrep avisa de XSS, pero ese paquete solo arma la parte `text/plain`; el HTML del correo usa `html/template` y su escape tiene prueba.
- A evitar: Mailpit lista los correos del más nuevo al más viejo; elegir por enlace o asunto, no por posición.
- A evitar: en el Bash de la herramienta, un heredoc de Python con `"\r\n"` se escribe como salto de línea real; para cadenas con escapes usar el editor o un archivo de script.

## Ejecución nv-07 (en curso, 2026-11-03)
- Estado: rama `nv-07-aws-dev` desde `main` (con nv-06 y la firma de nv-05). `infra/aws-dev` escrito y validado (`fmt`, `validate`, Trivy, critic). Perfil `wherever-dev` = `arn:aws:iam::816101201001:user/wherever-deployer` (sin `iam:ListUsers`). Permisos con cuenta comprobados antes del `apply` (IAM, Scheduler, Budgets y SSM responden "no existe", no "acceso denegado") y `run-instances --dry-run` → `DryRunOperation` (ya sin `OptInRequired`). `terraform apply` hecho el 2026-11-03 (21 recursos; un segundo `plan` sin cambios): instancia `i-056aba7efc9f57205`, IP elástica `18.227.230.50`, `https://18-227-230-50.sslip.io`. Arranque terminado (Docker 25, Compose v2.39.4, swap 2 GB), SSM `Online`, `verificar.sh` → OK. Desplegado el mismo día: IP en Atlas, parámetros en `/wherever/dev/` (`APP_ENV=staging`, `TRUSTED_PROXIES=172.16.0.0/12` para confiar solo en Caddy por la red de Docker, llaves JWT/HMAC nuevas distintas de las locales), `/ready` 200 por HTTPS con las cuatro dependencias. Falta: ciclo de apagado/encendido, `verificar.sh` con la instancia apagada, `destroy` y firma. En `/wherever/dev/` hay `HMAC_SECRET` y `JWT_SECRET` (creados a mano a las 15:44, nombres de otra guía) que nada en el repo lee: borrarlos (el permiso automático para escribir en el almacén de secretos se negó; lo corre el usuario).
- Cuentas AWS (nueva experiencia: cada proyecto es una cuenta). `051904434716` = "IntegradOps", la que se creó el 2026-11-03 con el plan gratuito (US$100 de crédito, **vence el 2027-05-02**; al vencer, o se cierra el entorno o se actualiza el plan). El entorno se despliega en el proyecto **"Ctrl Shift Create" = `816101201001`** (creado el 2026-11-02).
- Decisión: MongoDB en Atlas (la EC2 solo corre API, Redis, Neo4j y Caddy); la IP elástica va al Network Access de Atlas. Imagen construida en local y copiada por SSH (repo privado, sin registro). `APP_ENV=staging`.
- Decisión: nada se aplica con el usuario raíz ni con un usuario administrador. Usuario IAM `wherever-deployer` en `816101201001` con `wherever-dev-deployer-policy` (`infra/aws-dev/iam/`: región `us-east-2` y prefijos `wherever-dev-*` / `/wherever/dev/*`, más `ec2:ImportKeyPair` y `ec2:DeleteKeyPair`). El `dev-user` del proyecto tiene `AdministratorAccess`: no se usa para Terraform ni para el MCP.
- Decisión: el presupuesto no descuenta créditos (`include_credit = false`). Porqué: en el plan nuevo todo sale del crédito y, contándolo, la alerta nunca sonaría. La IP elástica (IPv4 pública) cuesta unos US$0.005/h ≈ US$3.6 al mes del crédito aunque la instancia esté apagada.
- Excepción Trivy AWS-0104: salida a 0.0.0.0/0 solo por 443, 80 y 27017 (las IPs de Atlas, Let's Encrypt y repositorios cambian).
- Deuda: la salida no abre SMTP (587/465). Al elegir proveedor de correo, se agrega ese puerto a `egress_ports` en `main.tf`; hasta entonces el correo no sale desde la EC2.
- Lección: `terraform plan` solo lee EC2 y no detecta permisos de IAM, Scheduler, Budgets o SSM atados a otra cuenta; se prueban con lecturas de nombres inexistentes antes del `apply`.
- Lección: la descripción de una regla de Security Group solo acepta `a-zA-Z0-9. _-:/()#,@[]+=&;{}!$*` (sin apóstrofo ni acentos); `plan` no lo valida y el `apply` falló a la mitad en dos reglas.
- Lección: en Windows la CLI de AWS con `--output text` termina en CRLF; `verificar.sh` lo quita con `tr -d '\r'`.
- Entorno local para demos (2026-11-04): el 27017 lo ocupa un `mongod.exe` (servicio de Windows, no es del proyecto) y Atlas rechaza a la API local cuando cambia la IP pública, que alterna entre dos. `.claude/launch.json` (`api-local`, sin commit) arranca la API con Mongo de Docker en 27018 (`MONGO_HOST_PORT=27018 docker compose ... --profile dev up -d --wait`) y Mailpit como SMTP, sin editar `.env`. Esa base local trae índices y 5 ciudades de ejemplo.
- Diagnóstico del 429 del front (2026-11-04): ningún intento llegó a crear cuenta; el registro no mandaba `version_terminos` (422) y el login no mandaba `dispositivo` o enviaba JSON ilegible (400). Cada intento fallido cuenta para el límite.
- Excepción de desarrollo (2026-11-03): la imagen desplegada en la EC2 no salió del `Dockerfile`; es el binario compilado en local desde `d2dc808` (código Go de `main` tras nv-06; nv-07 no toca Go) con la caché de módulos local, sobre `distroless/static-debian12:nonroot`. No es reproducible; el siguiente despliegue vuelve al `Dockerfile` (CI lo construye bien).
- Decisión pendiente del usuario: usuario de Atlas propio del servidor (`wherever-dev-ec2`, `readWrite` solo en `wherever`) y `MONGO_URI` del servidor con él; hoy el servidor usa la misma cadena que el `.env` local, cuya contraseña se mostró en el chat (rotarla también).
- Cuenta `816101201001` creada el 2026-11-02 (plan gratuito nuevo): `t3.small` es elegible (`describe-instance-types --filters free-tier-eligible=true`), sale de los créditos como lo demás.
- Estado de Terraform respaldado fuera del repo en `%USERPROFILE%/wherever-dev-terraform-respaldo/` (con `terraform.tfvars`); repetir tras cada `apply`.
- Lección: `docker build` en esta máquina se colgó dos veces en `go mod download` (más de una hora, `unexpected EOF` del proxy) aunque `curl` al proxy respondía. Alternativa usada: `CGO_ENABLED=0 GOOS=linux go build` en local con la caché de módulos y una imagen de una sola etapa sobre la misma base distroless. Si se repite, conviene una opción en `desplegar.sh`.
- Lección: `MSYS_NO_PATHCONV=1` rompe las rutas `/d/...` que se pasan a programas de Windows (`git -C`); úsalo solo en el comando `aws ssm`.
- Memoria de la `t3.micro` con todo arriba: ~96 MB libres y swap en uso; si la API crece, pasar a `t3.small`.
- Lección: en Git Bash, `aws ssm ... --name /wherever/dev/X` se convierte en ruta de Windows (`parameter/C`). Usar `MSYS_NO_PATHCONV=1`.
- Si cambia la IP de casa se pierde SSH (solo `ssh_cidr`); queda Systems Manager. Se actualiza `ssh_cidr` y se vuelve a aplicar.
- Agent Toolkit de AWS configurado: 25 skills en `~/.claude/skills` y `aws-mcp` en `~/.claude.json` y OpenCode con `AWS_MCP_PROXY_PROFILES=wherever-dev`. Reglas de AWS fuera de CLAUDE.md (decisión del usuario).

## Spec nv-08 (2026-11-04)
- Estado (2026-11-05): nv-08 y nv-09 fusionados (PR #12, #13, #15). `cd-dev` publica en GHCR: run 41988428645 en verde con aviso de sin conexión; `dev` = `7433d30`; se baja con el PAT. El servidor sigue en `d2dc808` porque el timer no está instalado. Lo que sigue en esta entrada es previo. Antes: rama `nv-08-cd`, implementación local sin desplegar. Boundary y política recortados activos, y boundary aplicado a los dos roles (2026-11-04). Falta: ver el ciclo real de 22:00 y 08:00 con el boundary, PAT en `/wherever/dev/GHCR_TOKEN`, variable `API_URL`, PR, `instalar-actualizador.sh` y pruebas reales.
- Decisión: se cancela OIDC + ECR + Systems Manager. Una SCP de la organización (`o-97o0ojcjt7`, cuenta `988573432780`, política `p-d16tlddp`) niega `iam:GetOpenIDConnectProvider` y Organizations no está disponible para el proyecto. Diseño por sondeo: GHCR y timer de systemd en la EC2. GitHub queda sin ningún acceso a AWS.
- Decisión: `deploy-env.sh` excluye `GHCR_TOKEN`; vuelca toda la ruta al `.env` y si no el token llegaría a la API.
- Decisión: antes de actualizar se exige `/ready` 200 de la versión actual, y un digest se registra como fallido solo si la anterior queda sana. Porqué: al arrancar, Neo4j tarda más que `OnBootSec=1min`, y sin esto una imagen buena quedaría marcada para siempre.
- Lección: `persist-credentials: false` impide cualquier `git fetch` posterior en un repo privado; con `fetch-depth: 0` no hace falta. Las dos primeras corridas de `cd-dev` fallaron por eso. `cd-dev` solo se puede probar desde `main`: `workflow_run` usa el workflow de la rama por defecto.
- Límite: `actualizar.sh`, las unidades de systemd y `deploy-env.sh` viven en el servidor y su mantenimiento en `infra/` es manual (`instalar-actualizador.sh`, por SSH desde la IP vigente).
- Límite: `main` no se puede proteger en este plan de GitHub (la API pide Pro o repositorio público); riesgo aceptado solo para desarrollo. El PAT caduca: al vencer, el servidor deja de actualizarse y el workflow termina en rojo.
- Límite: GitHub no distingue una instancia apagada de una caída; ambas dan verde con aviso.
- Pruebas intermitentes sin causa confirmada: en una corrida del usuario fallaron `TestMailer_RegisterSendsVerification`, `TestExport_NoticeWindowNeverPasses72h` y `TestMailerSecurity_TemplatesEscapeHTML`, sin guardar el mensaje; no se reprodujo en otras 3 corridas completas ni repitiendo las dos primeras 15 veces. Con carga, `TestMailer_SlowOrDownServerDoesNotChangeResponses` excede su límite de 500 ms (547–806 ms). Si CI las hace fallar, el CD no publica.
- PR #11 de nv-07 ya fusionado; su firma sigue pendiente (falta `destroy` y retirar `wherever-sin-limites`).

## Ejecución nv-09 (2026-11-05)
- Estado: rama `nv-09-ci-seguridad`, sale de `nv-08-cd`. CI de seguridad en jobs paralelos con gate, y ZAP nocturno. Probado en Actions: PR #13 en verde en unos 5 min, PR #14 con hallazgos plantados en rojo (cerrado sin fusionar), ZAP con 0 altas. Falta ver la tabla en Summary y, tras fusionar nv-08 y nv-09, el PR solo de `docs/`.
- Línea base sin hallazgos: `specs/nv-09/checklists/gates.md`.
- Decisión: se movieron a jobs propios lint, govulncheck, semgrep y `docker build`, en lugar de repetirlos. `critic`, `endpoints` y `sdd` van en `guardrails` porque aplican también a documentos.
- Decisión: ZAP con `.zap/plan.yaml` y el OpenAPI, no `zap-baseline.py`: la API no tiene páginas y el baseline solo vería la raíz. Vio 33 rutas.
- Lección: `critic B3` revisa el orden de los middlewares HTTP, no CI; no detecta cambios del workflow.
- Lección: `ci-pr` solo corre en PR hacia `main`; un PR apilado sobre otra rama no corre CI.
- Lección: el proxy de ZAP usa el 8080 por defecto; con `--network host` choca con la API.
- Lección: un `.semgrepignore` reemplaza las exclusiones por defecto de Semgrep.
- Límite: ZAP no ve respuestas autenticadas. Las reglas `p/` de Semgrep no tienen versión fija.

## Ejecución nv-10 (2026-11-05)
- Estado: rama `nv-10-fuzzing`, PR #16. 11 objetivos de fuzz y 7 pruebas de bordes. Actions en verde: cobertura 83,4 %, job `fuzz` en 15 min 6 s sin fallos. Falta la firma.
- Límite: en Actions, con 2 CPU, `FuzzHandlers` da unas 6 000 ejecuciones por noche; cubre poco comparado con los demás.
- Encendido con el boundary comprobado el 2026-11-05 a las 08:00:47 (nv-08). Timer instalado el mismo día: el servidor pasó solo a `7433d30` en 11 s. Falta probar el regreso automático con una imagen rota y un arranque con el timer activo.
- Lección: `actualizar.sh` corre como root y `deploy-env.sh` dejaba `.env` a su nombre, lo que rompía `docker compose` manual como ec2-user. Ahora hace `chown`.
- Hallazgo corregido: `verify` leía `m`, `t` y `p` del hash sin topes. `t=0` y `p=0` causaban pánico y una `m` enorme mataba el proceso (exit 137, probado con 512 MB). Solo se dispara con un hash alterado en la base. `parseHash` lo valida; los hashes actuales siguen verificando.
- Hallazgo corregido por el fuzz: `NormalizePhone` solo recortaba espacios en los extremos antes de quitar separadores; `"0\n("` dejaba el salto. Sin riesgo: E.164 lo rechazaba después.
- Decisión: sin repositorios simulados, que la spec pedía: no hay interfaces y `AGENTS.md` limita los mocks. `FuzzHandlers` usa la app real con las bases caídas.
- Decisión pendiente del usuario: con `APP_ENV=staging`, `detalle` trae el texto del driver; ante una caída de Atlas, el servidor público mostraría nombres de host. Por diseño (constitución §2), sin cambiar.
- Lección: `app.Test` no reproduce el rechazo por `BodyLimit`; hace falta un servidor real.
- Lección: `go test` con Docker limitado a 512 MB mata al compilador; se compila con `-c` y se ejecuta con el límite.

## Estado de CI/CD (2026-11-05)
- Firmas: nv-07 a nv-10 aprobadas por Osmar Reséndiz el 2026-11-05, con pendientes aceptados en cada `spec.md`. La de nv-07 se dio sin el `destroy`, que antes era condición.
- `wherever-sin-limites` retirado el 2026-11-05: límites y bloqueo de login vuelven a aplicar en staging.
- CD funciona: del merge a `/health` en unos 9 min. Regreso automático probado con una imagen rota: 2 min de caída, vuelve sola y no reintenta. Evidencia en `specs/nv-08/checklists/gates.md`.
- Hallazgo: cada publicación crea 2 versiones en GHCR (`imagetools create` envuelve la imagen en un índice), así que `min-versions-to-keep: 5` guardaba unos 2 o 3 despliegues. Subido a 10.
- En GHCR queda la etiqueta `prueba-rota` hasta que la limpieza la borre; su digest sigue en `failed_digest` y no estorba.

## Ejecución nv-15 (2026-11-05)
- Corrige tres hallazgos de la auditoría: trabajo nocturno que nunca corría en AWS (`JOB_HOUR=21`), alergias sin consentimiento (`privacidad.datos_sensibles`, 422 en gustos) y `detalle` y sondas públicos (`HIDE_INTERNALS=true`). Parámetros en Parameter Store; los toma `deploy-env.sh` al instalar la imagen.
- Se numeró nv-15 porque el usuario agregó nv-11 a nv-14 (Aura y Upstash, balanceador, CloudWatch, infraestructura en el pipeline), sin ejecutar.
- Cambio para la app: pedir `datos_sensibles` antes de mandar alergias.
- Prueba intermitente con dato nuevo: `TestMailer_RegisterSendsVerification` sin correo en 10 s en el primer envío de la suite; 1 de 4 corridas locales.
- Lección: la IP pública de casa alterna y cierra el SSH; lo que se pueda configurar por Parameter Store no depende de ella.

## NV-11 — Datos gestionados externos
- Estado (2026-11-05): el servidor ya usa Upstash y Aura; Redis y Neo4j salieron de la instancia, con sus volúmenes. `verificar.sh` OK y `terraform plan` sin cambios. El servidor corre `f1a28f8` con la reducción de comandos: 5 `/health` no mueven `rate:global`, y un `/ready` lo sube en 1. Falta firmar y contrastar el 40 % con la consola de Upstash.
- Latencia desde el servidor: `/ready` con las tres bases 88 ms en caliente y 352 ms en frío, bajo el límite de 1 s; el 503 del arranque que se vio desde la máquina local no se repitió ahí.
- Medición con `scripts/medir-redis.sh` (comandos que Upstash cobra), antes → después: `/health` 5 → 0; `/ready` 5 → 3; lectura autenticada 6 → 4; escritura autenticada 11 → 7; registro 15 → 9; login correcto 17 → 11; login fallido 18 → 12; refresh 16 → 10; logout 12 → 8.
- Estimación: 2000 peticiones por día laborable (22 días, supuesto, sin tráfico real medido) y 4.6 comandos de promedio dan 202 000 al mes, el 40 % de la cuota. El tope del 60 % equivale a unas 65 000 peticiones al mes. Dos APIs no suben el costo por petición.
- Decisión: `Pipeline` en vez de `TxPipeline` en el limitador. Porqué: Upstash cobra `MULTI` y `EXEC`; el TTL se repara solo porque cada golpe vuelve a mandar `ExpireNX`.
- Decisión: `/health` fuera del límite global. Porqué: el chequeo del balanceador de nv-12 agotaría la cuota por sí solo. **nv-12 debe chequear `/health`, no `/ready`**: serían 887 000 comandos al mes, y una pausa de Aura sacaría a las dos instancias.
- Decisión: `REDIS_ADDR` y `NEO4J_URI` en vez de las cuatro claves de la spec original. Porqué: la API ya las lee y acepta TLS; no hace falta código.
- Aura (2026-11-05): `make siembra` idempotente, 9 nodos y 9 relaciones. Latencia desde la máquina local: 164 ms de mediana y 1 s en la primera conexión, justo en el límite de `/ready`; falta medirla desde el servidor.
- API local contra Upstash y Aura (2026-11-05): registro 201, login 200, perfil 200, refresh 200, logout 204 y token revocado 401; `/ready` 200 en 235-351 ms. Sin credenciales en los logs. Neo4j con clave errónea o inalcanzable: `/ready` 503, `/health` 200.
- Redis desde la máquina local: 85 ms de mediana y 545 ms en la primera conexión. Upstash quedó en `us-east-2`, según su IP.
- Lección: Upstash rechazó con `WRONGPASS` varias claves hasta copiar la cadena de Connect → TCP; no se aclaró la causa. No armes la URL a mano.
- Lección: con una clave mala, `go-redis` muestra `i/o timeout` y no `WRONGPASS`, porque Upstash deja de responder tras el error de `AUTH`. Para diagnosticar, usa `redis-cli`.
- `main` falló en `govulncheck` tras fusionar el PR 21, con el del PR en verde minutos antes: GO-2026-6888 (`brotli`) y GO-2026-6865 (`klauspost/compress`), ambas indirectas. Se subieron a v1.2.7 y v1.20.2. Sin CI verde en `main` no se publica imagen.
- Lección: el grupo de seguridad limita la salida; los puertos de un servicio externo nuevo se abren en `egress_ports` o la API no llega.
- Riesgo aceptado hasta decidir otra cosa: una sola IP dentro del límite global agota la cuota en menos de 3 horas.

## NV-12 — Dos APIs, balanceador y CloudFront (en curso)
- Estado (2026-11-06): todo recreado con los scripts de NV-14 y probado: despliegue escalonado, regreso, compresión, petición larga y límite por cliente con `TRUSTED_PROXY_HOPS=2`. URL final en `cloudfront_url` y en la variable `API_URL`. Faltan que la app compile con ella, `cd-dev` con todo apagado y el encendido de las 08:00.
- Lección: en Git Bash, `export MSYS_NO_PATHCONV=1` también rompe el `curl -o` a `/tmp`: da cuerpos vacíos y parece un fallo de la API. Úsalo solo por comando.
- Decisión: recrear todo y cambiar la URL en una sola ventana. Porqué: la URL de CloudFront cambia al recrear, y la app tendría que cambiarla dos veces.
- Lección: una prueba de regreso debe durar más que el retraso de B; en la primera corrida se restauró `dev` antes de que B llegara a su comprobación.
- Lección: `tls: internal error` al conectar con Atlas suele ser una IP no permitida. Una instancia sin Mongo sigue sana para el balanceador, porque `/health` no consulta bases: revisa `/ready` en cada instancia tras tocar Atlas.
- Decisión: CloudFront da el HTTPS, sin dominio. Porqué: ACM no valida `sslip.io` y no hay dominio propio.
- Decisión: puerto 22 cerrado e instalación por Systems Manager. Porqué: la IP del operador cambia con frecuencia. Exige `ssm:SendCommand` en la política del deployer.
- Decisión: la IP del cliente sale de `X-Forwarded-For` a `TRUSTED_PROXY_HOPS` del final, y no de la primera entrada. Porqué: Fiber sin `EnableIPValidation` devuelve la cabecera tal cual; hoy no se explota porque Caddy la limpia, pero el balanceador la conservaría y un cliente evadiría el límite de login.
- Prueba (T03, 2026-11-05): la cuenta permite crear balanceador y CloudFront; se borraron y quedan 0. La política necesitó además `ec2:GetSecurityGroupsForVpc`.
- Decisión: la compresión tampoco toca `POST /usuarios`. Porqué: el registro también devuelve tokens, no solo `/auth`.
- Pendiente de nv-15: el trabajo de las 21:00 del 2026-11-05 quedó sin revisar: la sesión se reinició antes y la instancia se apagó a las 22:00. Sus logs viven en el contenedor: se leen después de las 08:00 y **antes de fusionar otro PR**, porque instalar una imagen recrea el contenedor y los borra.
- Lección: `critic-guardrails.sh` guarda su propia lista del pipeline y `check_endpoints` exige fila hasta para las sondas: al cambiar el pipeline o una ruta hay que tocarlos.
- Decisión: varias cabeceras `X-Forwarded-For` cuentan como una lista. Porqué: la primera la puede escribir el cliente.

## NV-13 — Observabilidad en CloudWatch (en curso)
- Estado (2026-11-06): logs en CloudWatch funcionando en el servidor. Las alarmas `errores`, `respuestas` y `dependencia` sonaron a los 50 s con un contenedor aparte; Insights devuelve resultados; 0 datos sensibles en 31 líneas reales. Correo de alarma recibido el 2026-11-06. Faltan ver el evento dentro de una instalación real con la siguiente imagen y contrastar el volumen estimado (unos 14 MB al mes).
- NV-15: el trabajo de las 21:00 corrió el 2026-11-05 sin fallos y sin pendientes.
- Decisión: el logger también oculta correos y credenciales de URI dentro de cualquier texto. Porqué: solo miraba el nombre del campo, y el texto de un error de MongoDB puede traer un correo; ahora esos logs salen de la instancia.
- Decisión: timer `wherever-sonda` que consulta `/ready` cada 5 minutos. Porqué: nadie más lo consulta con frecuencia, y la alarma nunca sonaría. Cuesta el 2 % de la cuota de Redis por instancia.
- Decisión: controlador `awslogs` en modo `non-blocking`. Porqué: una caída de CloudWatch no debe impedir que la API arranque.
- Lección: `log_event` tragaba su error con `|| true` y los eventos no se registraban, así que la alarma de regreso no podía sonar. Lo que no debe detener un flujo igual debe avisar: ahora escribe el fallo en el journal.
- Lección: cambiar el controlador o instalar una imagen recrea el contenedor y borra sus logs locales.
- Lección: al adjuntar una política, IAM tarda unos minutos en propagar y la misma llamada pasa y falla alternadamente. Antes de un `apply`, repite una lectura hasta verla pasar varias veces seguidas.
- Lección: una política administrada no pasa de 6144 caracteres; la del deployer ya no admite más, y lo nuevo va en políticas aparte.

## NV-14 — Crear y eliminar la infraestructura (en curso)
- Estado (2026-11-06): el docente aceptó scripts versionados, según Osmar Reséndiz. Ciclo real hecho: 55 recursos borrados y 53 creados, y la API instalada sola en las dos instancias. Falta firmar.
- Decisión: scripts y no workflow. Porqué: el docente lo acepta y evita claves de AWS de larga duración en GitHub, lo que respeta la decisión de NV-08.
- Decisión: el estado de Terraform sigue local, con respaldo automático. Porqué: el bucket de S3 solo hacía falta para que corriera en GitHub; un solo operador no lo necesita. Si se pierde esa máquina, se pierde el estado: copia el respaldo a otro lado.
- Lección: una instancia recreada no se instalaba sola: no tenía archivos, y `actualizar.sh` exigía `/ready` 200 de una versión que aún no existe.

## Ruta hacia la evaluación (nv-16 a nv-53)
- Estado (2026-11-08): 38 specs en borrador, revisadas contra el código. 19 en la ruta prioritaria y 19 pospuestas hasta después del 2 de diciembre. Ninguna ejecutada.
- Decisión (2026-11-10, Osmar Reséndiz): se ejecutan **todas** las specs, no solo la ruta prioritaria. Las olas 1 a 3 siguen primero porque responden a la auditoría; las olas 4 a 7 siguen después, en orden de dependencias. Riesgo: son unas 240 tareas y quedan unos 22 días; nv-11 a nv-14 tomaron cerca de dos días cada una. Si el tiempo no alcanza, lo que quede sin hacer el 2 de diciembre es de las olas 4 a 7.
  - Ola 1, catálogo y recomendaciones: nv-20, 21, 23, 24.
  - Ola 2, planes con IA (hallazgo C1): nv-26 a nv-33, con Gemini (ver nv-26).
  - Ola 3, amistades, ML y grafo: nv-34, 35, 36, 50, 51, 52, 53. nv-34 a 36 entran porque el grafo de nv-52 y 53 necesita amistades.
  - Ola 4, plataforma y catálogo completo: nv-16 → 17 → 18 → 19, y nv-22 y 25. Va antes que las demás porque nv-42 y nv-46 dependen de la caché de nv-18, y nv-19 publica el aviso con el proveedor de IA.
  - Ola 5, grupos y notificaciones: nv-37 → 38 → 39, nv-40 → 41.
  - Ola 6, imágenes y moderación: nv-42 → 43, nv-44 → 45.
  - Ola 7, sin conexión y cifrado: nv-46 → 47, nv-48 → 49. nv-48 y 49 solo dependen de nv-05: si se adelantan, se cifran menos datos ya guardados.
- Costos nuevos fuera de la ruta prioritaria: S3 y la distribución de imágenes (nv-44) y Firebase (nv-41, gratis en su capa básica). Revisar el presupuesto de US$50 antes de la ola 6.
- Falta en la ruta: el correo (C4) sigue sin proveedor y ninguna spec lo resuelve.
- Estimación de Redis con la ruta prioritaria: la base sigue en unos 40 % de la cuota. Generar planes suma unos 11 comandos por petición (límite 3, revocación 1, límite de IA 3, caché 2, gasto 2); con 50 al día, menos del 2 %. Las demás rutas nuevas no usan Redis fuera del limitador. Total estimado: unos 42 %. Con todas las specs hay que rehacerla: el cierre de votaciones de nv-39 por sí solo suma cerca del 7 % (unos 49 %) y el tope diario de IA lleno otro 16 % (unos 65 %). La caché de nv-18 (medida el 2026-11-12) suma 1 comando por acierto y 2 por fallo en fichas y categorías: cerca del 3 % más (ver nv-18). Con todas las specs la estimación ya pasa del tope del 60 %: revisar plan B de nv-11 antes de la ola 5.
- Hallazgos de la revisión de las olas 4 a 7 (aplicarlos al ejecutar cada una):
  - nv-17: ya existe `internal/shared/pagination`; lo que falta es firmar el cursor y reutilizar `internal/shared/etag` de nv-16.
  - nv-19: `version_terminos` y la fecha de aceptación ya viven en la privacidad del usuario (nv-05).
  - nv-22: el índice de texto de Mongo no tolera errores de escritura; hace falta Atlas Search u otra técnica.
  - nv-37: sus pruebas incluyen unirse por enlace, que es de nv-38.
  - nv-38: los enlaces de la app dependen de la URL de CloudFront, que cambia al recrear.
  - nv-39: el cierre de votaciones cada minuto con dos instancias gasta cerca del 7 % de Redis.
  - nv-41: el JSON de Firebase tiene varias líneas y `deploy-env.sh` escribe una variable por línea; el nombre `fcm-service-account` tampoco es válido como variable.
  - nv-44: `destruir.sh` falla con un bucket con objetos si Terraform no lo vacía (`force_destroy`).
  - nv-45: la API tiene 192 MB y una foto de 4000x4000 decodificada ocupa unos 64 MB; la biblioteca estándar de Go no lee ni escribe WebP.

## NV-26 — Cliente de IA con Gemini (en curso)
- Estado (2026-11-09): rama `nv-26`. Interfaz `ia.Modelo`, adaptador de Gemini (`generateContent` con `responseJsonSchema`), validación estricta con un reintento, anonimización de la frase y configuración (`GEMINI_API_KEY`, `IA_MODELO_ID`, `IA_TIMEOUT`). Unit verde. Prueba en vivo con `gemini-3.5-flash-lite`: 20 de 20 válidas, 0 429, p50 870 ms y p95 1169 ms, unos 150 tokens de entrada por frase. Falta la firma.
- Hallazgo de la prueba en vivo: cabe de sobra en el plazo de 8 s de nv-33 con dos llamadas. Calidad mejorable: «con la banda» salió sin compañía; se afina en las instrucciones de nv-31, no en el adaptador.
- Operación: la llave vive en SSM (`/wherever/dev/GEMINI_API_KEY`). La prueba en vivo la toma de ahí al entorno del contenedor sin imprimirla; no hace falta copiarla a `.env`.
- Decisión (revisor): Gemini, capa gratuita, en lugar de Bedrock. Porqué: la cuenta de AWS tiene cuota cero para los modelos de Bedrock. El adaptador de Bedrock queda como opción futura detrás de la misma interfaz; no se agregaron permisos de Bedrock a IAM.
- Decisión: un 429 de Gemini es `ErrLimiteProveedor`, no se reintenta y responde el motor de reglas. Nuestros límites (nv-28) son el 80 % de los que AI Studio muestra para el proyecto con `gemini-3.5-flash-lite` (2026-11-09: 15 por minuto, 500 por día, 250 000 tokens por minuto): 12 llamadas por minuto y 400 por día, con el día en hora del Pacífico como el del proveedor. La documentación pública ya no publica cifras de la capa gratuita; si cambia el modelo, se vuelven a leer en AI Studio.
- Redis: con el tope diario de IA lleno (200 generaciones) la estimación sube de unos 42 % a unos 58 % de la cuota de Upstash.
- Decisión: la llave va en el encabezado `x-goog-api-key`, nunca en la URL, para que no salga en errores de `net/http`; el logger oculta además cualquier `AIza…` en texto libre.
- Privacidad: la capa gratuita usa lo enviado para mejorar los productos de Google y permite revisión humana. Solo viaja texto anonimizado y datos públicos de negocios; el aviso lo dice en `docs/aviso-privacidad.md`. Sus términos exigen 18 años o más (el registro ya lo exige) y prohíben la capa gratuita para personas del EEE, Suiza y Reino Unido.
- Red: sin cambios; las instancias ya salen por 443 a cualquier destino.
- A evitar: en Windows, Control de aplicaciones bloquea `gofmt`, `vet` y los `*.test.exe`; correr todo en `golang:1.27.3` con Docker.

## NV-27 — Tope de gasto y caché de la IA (en curso)
- Estado (2026-11-11): rama `nv-27` desde `main`. `ia.Guardia` envuelve el modelo con el tope mensual y la caché; `usos_ia` registra cada llamada. Unit, lint, critic, SDD y endpoints en verde; Integration en el CI del PR. Falta la firma. Sin cableado: lo arma quien llame a la IA (nv-31, nv-33).
- Decisión: la guardia es a su vez un `ia.Modelo`, así el reintento de `ia.GenerarValidado` también pasa por el tope. La caché va en `Guardia.GenerarValidado` y solo guarda salidas ya validadas; si guardara antes de validar, el reintento leería la misma salida inválida.
- Decisión: colección `usos_ia` (la spec decía `ia_uso`; la regla pide plural en español). Llaves `cuota:ia_gasto:{AAAA-MM}` (micro-USD, mes en UTC) y `cache:ia:{huella}` (1 h).
- Decisión: sin la llave del mes se reconstruye desde `usos_ia` con `SET NX`; con Redis caído se lee `usos_ia`; sin los dos se cierra (`ErrGastoDesconocido`). La caché no cierra.
- Privacidad: ARCO quita `usuario_id` de `usos_ia` en lugar de borrar, para no bajar el gasto del mes; la exportación lista los usos sin ids. TTL de 400 días.
- Configuración: `IA_TOPE_USD_MES` (5, máximo 50), `IA_PRECIO_ENTRADA_USD_MTOK` e `IA_PRECIO_SALIDA_USD_MTOK` (0 en la capa gratuita). Con costo cero el gasto cuesta un solo `GET` por llamada.
- A evitar: escribir en los logs campos `dependencia` o `evento` fuera de su uso: alimentan alarmas de CloudWatch. La guardia usa `parte`.

## NV-28 — Límites y protección de datos en la IA (en curso)
- Estado (2026-11-11): rama `nv-28` desde `nv-27`, PR contra `nv-27`. Límite por usuario y global dentro de `ia.Guardia` (`internal/ia/limite.go`), `ia.SoloDeLaLista` para los ids y `IA_LIMITE_HORA/MIN/DIA`. Build, vet, Unit, lint, critic, SDD y endpoints en verde; Integration (`ia_limite_test.go`) y Security (`ia_security_test.go`) en el CI del PR. Falta la firma. Sin cableado: lo arma nv-31/nv-33.
- Corrección de drift: la spec decía `ia_uso` (es `usos_ia`) y la tarea 10 por hora (son 5 generaciones, hasta 10 llamadas); los límites no eran una pieza aparte sino parte de la guardia.
- Decisión: el global va en `Guardia.Generar` (cuenta cada llamada, también el reintento) y el del usuario en `GenerarValidado` después de la caché (cuenta generaciones; un acierto no cuenta). Sin usuario en el contexto se usa la cubeta `rate:ia_usuario:sin_usuario`: olvidar `ia.ConUsuario` no quita el límite.
- Decisión: ventanas fijas con `SET NX` + `INCR` + `PTTL` en un Pipeline (3 comandos por ventana, `TestPresupuestoRedis_LimitesIA`). La hora del usuario empieza con su primera generación y `Regreso` sale del TTL real. Las llamadas rechazadas también cuentan. Sin Redis se cierra (`ErrLimiteDesconocido`).
- Decisión: el día global es el de `America/Los_Angeles`, como la cuota del proveedor; `cmd/api` ya importa `time/tzdata`. Los máximos de configuración (15 por minuto, 500 por día) son los de la capa gratuita.
- Privacidad: `Security/IA` prueba con el adaptador real contra un servidor local que la frase solo va en el turno de usuario, que un id ajeno se descarta, que ni correo, teléfono, coordenadas, enlaces ni el id del usuario viajan, y que ni `usos_ia` ni la caché guardan la frase.
- A evitar: `resultadoDe` con un `switch` por error superaba el gocyclo de 10; ahora es una tabla. Agregar un error nuevo de la guardia es una fila.

## Spec nv-16 — Versión de la app y mantenimiento (2026-11-09)
- Estado (2026-11-12): aprobada por Osmar Reséndiz con la enmienda 1.7.0 e implementada. Local: build, vet, Unit, lint 0, critic, `check_endpoints` y `check_sdd`; el resto lo corre el CI del PR. Falta cargar las cinco claves en `/wherever/dev/` y probar 426, 304 y 503 por CloudFront. Define `GET /api/v1/configuracion` (cinco campos, ETag/304), `X-App-Version` con 426 y `MODO_MANTENIMIENTO` con 503.
- Decisión: la etapa `version` entra entre `rate_limit_writes` y `jwt`, y el 426 se agrega a `apperr`. Solo actúa sobre `/api/v1/`, para que una ruta inexistente siga dando 404. Exige enmienda de la constitución §4/§5 (1.7.0; la 1.6.0 ya la usó nv-12), actualizar `critic B3` y `TestPipeline_ConstitutionalOrder`. Sin la enmienda, nv-16 no se fusiona.
- Decisión propuesta: cabecera ausente o ilegible no bloquea; solo un semver numérico válido y menor da 426. Es aviso de compatibilidad, no control de acceso.
- Decisión propuesta: auth queda exenta del 426 para poder cerrar sesión, refrescar y recuperar la cuenta durante la actualización; `configuracion` queda exenta para que la app sepa qué instalar. En mantenimiento, la configuración sigue en 200/304 y el resto no exento da 503 con `Retry-After: 60`.
- Decisión propuesta: los valores viven en `APP_VERSION_MINIMA`, `APP_VERSION_ACTUAL`, `TERMINOS_VERSION`, `AVISO_VERSION` y `MODO_MANTENIMIENTO` (Parameter Store en el servidor); se leen al arrancar. El actualizador no reinicia por un cambio de parámetro: encender el mantenimiento es un paso manual por Systems Manager en cada instancia.
- Decisión propuesta: ETag débil (`W/`), porque la compresión puede mandar el cuerpo en gzip y un ETag fuerte debería cambiar con la codificación.
- Corrección de drift: la lista del pipeline en la constitución §4 omitía `request_timeout`; la enmienda de nv-16 la deja igual al código real.
- A evitar: comparar versiones como texto (`1.10.0` quedaría antes que `1.9.0`); crear otro tipo de error con estado 503 para mantenimiento (`KindFromStatus` recorre un mapa y el resultado no sería determinista).
- Lección: `configuracion.NewService` no devuelve error (la respuesta solo tiene textos y un booleano): con un `if err` más, `buildModules` pasaba el gocyclo de 10.
- Lección: los scripts de Python en Windows escriben CRLF si no se abre con `newline=''`; un `.sh` con CRLF rompe bash en el CI. Los gates que usan `git` (`critic`, `check_sdd`) no corren dentro de Docker en un worktree: correrlos en el host y `check_endpoints` (necesita Go) en Docker.

## Spec nv-20 — Negocios aprobados, categorías y carga semilla (2026-11-10)
- Estado (2026-11-10): implementada en la rama `nv-20`, con las cinco suites verdes, lint, gosec y Semgrep en 0. Falta la firma y cargar el CSV real de 20 a 30 negocios del piloto (T21): todavía no existe. Crea `negocios` (A2/A5), `POST /api/v1/admin/negocios/importar` y `GET /api/v1/categorias`; corre antes que nv-17 y nv-18 (ola 4).
- Corrección de drift: el stub hablaba de 15 gustos y estados `pendiente/suspendido`; el Excel A2/A5 define 6 giros (`categoria` singular), estados `borrador/en_revision/aprobado/rechazado/pausado`, `slug` único, `nombre_normalizado`, `nivel_precio` y `calificacion` como objeto. Los gustos de A1 son texto libre (3–10) y no se tocan.
- Decisión propuesta: no hay colección ni paquete `categorias`; el catálogo de 6 giros es una constante en `internal/negocios` y los conteos salen de `negocios` aprobados. `GET /categorias` devuelve giros, etiquetas/servicios con conteo y filtros estáticos; no ETag/304 y `no-store` hasta nv-18.
- Decisión propuesta: semilla idempotente por `slug` (CSV o JSON, 1–100 filas), valida fila por fila, éxito parcial y deja `aprobado`. `propietario_correo` opcional; si falta, el admin que importa es el dueño provisional. `qr_checkin` se genera al crear y no cambia; reimportar no pisa calificación, cupones ni sello.
- Decisión propuesta: extensiones al modelo A2: `version` (futuro If-Match de A5), `horario_min[]` (filtro de nv-21) y `alergenos[]` (filtro duro de nv-23). Los 9 índices salen de A2/A5 y solo los crea `scripts/indexes`.
- A evitar: usar `categorias[]`/`precio_nivel`/`nombre_norm` del stub; contar negocios no aprobados; crear `internal/categorias` o `scripts/indices_negocios`; duplicar al reimportar; resetear campos de control; agregar ETag sin nv-18.
- Lección: la validación por fila supera el gocyclo de 10 si va en una sola función. Se parte en ayudantes (`csvRow`, `checkHorario`, `validateIdentity`, `activeAdmin`, `owner`, `newBusiness`); correr `golangci-lint` v2.15.0 en Docker antes de abrir el PR.
- Lección: `version` va solo en `$inc` (MongoDB rechaza el mismo campo en `$setOnInsert` e `$inc`); en un alta queda en 1.
- Pendiente para nv-23: `alergenos[]` lista lo que el negocio contiene y una lista vacía no distingue «sin alérgenos» de «sin dato». Antes del filtro duro, definir cómo se marca un negocio sin dato para no recomendarlo a alguien con alergias.
- Proceso: PR 36 se fusionó en `specs-nv-16` después de que esa rama entró a `main`, así que su commit llega a `main` con el PR de nv-20. Revisar la base de un PR apilado antes de fusionarlo.

## Spec nv-17 — Paginación firmada y validadores HTTP (2026-11-10)
- Estado (2026-11-12): aprobada e implementada en la rama `claude/nv-17-af35a1`. Local verde (build, vet, Unit, lint, critic, sdd, endpoints); Integration, Security, Database y Contract en el CI del PR. Pendiente del operador: cargar `/wherever/dev/CURSOR_HMAC_KEY` (SecureString, 32+ bytes) antes de desplegar y probar 200/304 y `no-store` por CloudFront.
- Corrección de drift: la spec suponía que no había lista que migrar, pero `/lugares` (nv-21) y `/planes` (nv-30) ya paginaban sin firma. El usuario aprobó migrarlas aquí, así que `CURSOR_HMAC_KEY` entra ya como obligatoria (config, `.env.example`, `deploy-env.sh`, `nightly.yml`, `testutil.CursorKey`).
- Decisión: cursor `base64url(JSON{_id,orden,ambito}).base64url(HMAC-SHA-256)`; `hmac.Equal` antes de leer el JSON. `Signer.Scope(nombre, partes…)` usa un HMAC truncado con llave, así el ámbito no deja leer la ubicación ni el id. Lugares: orden, categoría, precio, abierto, radio y lat/lng. Planes: dueño y pestaña.
- Decisión: un solo `Signer` en `buildBusinesses` (`cmd/api`), inyectado con `WithCursor` en negocios y planes; un service sin `Signer` falla cerrado. `validation.Position(signer, scope, raw)` es el único camino a una posición y da 422 `cursor`; la regla `cursor` del validador solo mira la forma (`pagination.Valid`).
- Decisión: ETag/304 en `GET /ciudades/autocompletar` con `public, max-age=60` (una sola serialización con el `JSONEncoder` de Fiber); `no-store` en `/usuarios/yo` y `/auth/sesiones`, sin 304.
- A evitar: probar una ruta pública con `sendWithHeaders` (manda `Bearer` vacío = 401); `max=200` en el `cursor` de un DTO (el firmado mide ~220; se subió a 300); poner datos legibles en el ámbito; agregar otro `if err` en `buildModules` (gocyclo 10) o un tercer chequeo de llave en `Config.validate` (se movieron a `validateKeys`).
- Efecto: los cursores de `/lugares` y `/planes` emitidos antes del despliegue dan 422; la app reinicia en la primera página. Rotar la llave hace lo mismo.

## Spec nv-18 — Caché en Redis (2026-11-12)
- Estado (2026-11-12): ejecutada en la rama `claude/nice-almeida-3fcaaa`, firma de Osmar Reséndiz pendiente. Local verde (build, vet, Unit, lint 0, critic, sdd, endpoints); Integration, Security, Database y Contract en el CI del PR.
- Corrección de drift: la spec pedía cachear la configuración, pero nv-16 ya la lee en memoria al arrancar; en Redis solo gastaría comandos. Se cachean categorías (1 h) y fichas de negocio (5 min). Listas, mapa y autocompletar no: dependen de la consulta.
- Decisión: paquete `internal/platform/cache`, `cache.Obtener(ctx, c, clave, ttl, fn)` genérico con `singleflight` (una consulta por clave e instancia) y llaves `cache:categorias:todas` y `cache:negocio:{id}`. La carga compartida usa `context.WithoutCancel` con plazo de 10 s, para que cancelar la primera petición no tumbe a las demás.
- Decisión: cualquier error de Redis (caído o cuota de Upstash agotada) apaga la caché 30 s y responde MongoDB, con un solo aviso por pausa. Así se cumple «desactivar la caché antes de que falle la API» sin otra variable de configuración.
- Decisión: la ficha guarda las ventanas de horario y «abierto ahora» se recalcula en cada petición. `Upsert` pasó a `FindOneAndUpdate` (documento anterior, solo `_id`) para borrar la ficha del negocio actualizado; la importación hace un solo `DEL`.
- Presupuesto medido (`TestPresupuestoRedis_CacheDeFicha`): fallo = limitador + 2 (`GET` + `SET`), acierto = limitador + 1. Las cifras de las operaciones de nv-11 no cambian. Estimación: si una de cada cuatro peticiones (11 000 al mes) es ficha o categorías, con 70 % de aciertos, suma unos 14 300 comandos, cerca del 3 % de la cuota (base ruta prioritaria ~43 %; con todas las specs ~68 %). Peor caso, todas las peticiones fallando la caché: +88 000, el 18 %.
- A evitar: guardar «abierto ahora» en la caché (se congela 5 min); usar la llave en el aviso del log (puede llevar un id); modificar el valor que devuelve `Obtener` (lo comparten las peticiones simultáneas).
## Spec nv-21 — Lista, ficha y mapa de lugares (2026-11-10)
- Estado (2026-11-10): implementada sobre `main` (ya trae nv-20); falta la firma. Crea `GET /api/v1/lugares`, `/lugares/mapa` y `/lugares/{id}` dentro de `internal/negocios` (sin paquete nuevo ni índices nuevos).
- Corrección de drift: la spec decía «minutos de la semana»; nv-20 guarda `horario_min[]` como `{dia, desde, hasta}` por día con `hasta` > 1440 al cruzar medianoche. Abierto = ventana de hoy con `desde ≤ m < hasta` o la de ayer con `hasta > m + 1440`, en `America/Mexico_City`. Misma regla en Go (`OpenAt`) y en Mongo (`openAtFilter`, `$elemMatch`).
- Decisión: todos los filtros van en `$geoNear.query` o `$match` antes del `$limit`, unidos en `$and` para que los dos `$or` (abierto ahora y cursor) no choquen. Por distancia, la página siguiente usa `minDistance` y un `$match` quita los empates ya entregados por `_id`.
- Decisión: el cursor compartido guarda `orden:valor` en `orden`; un cursor de otro orden es 422. Radio 5 km por defecto y 20 km máx.; mapa con 100 pines y luego celdas de `360/2^zoom/4` grados (máx. 400).
- Privacidad: el error de `List` no lleva el texto de MongoDB (podría repetir la ubicación); un test revisa que `lat`/`lng` no lleguen a los logs.
- A evitar: escribir `` `lat`/`lng` `` en `docs/endpoints.md`: `check_endpoints` lee `` `/` `` como la ruta `GET /`. Los scripts con `git rev-parse` fallan en Docker desde un worktree; montar el `.git` principal y usar `GIT_DIR=/gitmain/worktrees/<nombre>` con `GIT_WORK_TREE=/src`.
- A evitar: editar con Python en Windows sin `newline=''`: deja CRLF y `gofmt -l` marca el archivo entero.

## Spec nv-23 — Filtros duros y puntuación de recomendaciones (2026-11-10)
- Estado (2026-11-10): implementada en la rama `nv-23` sobre `main` (ya trae nv-21); falta la firma. Paquete `internal/recomendador` sin endpoints: `Recomendar(ctx, fuente, pesos, contexto, n)` para nv-24 y nv-31.
- Corrección de drift: el plan creaba `internal/clima` (es nv-25) e `internal/inicio` (es nv-24) y tocaba `cmd/api`, `.env.example` y `docs/endpoints.md`; no hacen falta. Los pesos van embebidos en `internal/recomendador/pesos_recomendador.json` (la imagen no copia `configs/`) y se validan al llamar `Recomendar` (`ErrPesos`); nv-24 los valida al arrancar cuando haya consumidor.
- Decisión (cierra el pendiente de nv-20): `alergenos: ["ninguno"]` (`negocios.AllergenNone`, va solo) = declaró que no contiene ninguno; lista vacía = sin dato. Con alergias pasa solo quien tiene dato y no contiene ninguna; si `gustos.alergias` (texto libre) trae algo fuera del catálogo, pasa solo `["ninguno"]`. Los negocios ya cargados con lista vacía dejan de recomendarse a quien tiene alergias hasta que declaren.
- Decisión: filtros duros en `$geoNear.query` vía `negocios.Repository.Candidates` (reusa `List` de nv-21 por distancia, sin índices nuevos) y otra vez en Go (`Filtro.Cumple`) antes de puntuar. Dieta: solo las `restricciones` que están en el catálogo. «No me interesa» llega como ids: aún no hay colección.
- Decisión: pesos presupuesto 0,20, distancia 0,20, horario 0,10, clima 0,10 (fijo 0,5 hasta nv-25), gustos 0,25, calificación 0,15; total a un decimal para que el orden no dependa del redondeo; empates por calificación y luego id.
- A evitar: tratar una lista vacía de `alergenos` como «sin alérgenos»; poner cifras, alergias o dieta en `por_que`.

## Spec nv-29 — Modelo de plan, paradas y traslados (2026-11-11)
- Estado: implementada en la rama `nv-29` sobre `main`; falta la firma y el CI del PR (Integration, Security, Database, Contract). Paquete `internal/planes` sin endpoints (llegan con nv-30): modelo, `Recalcular`, `Calculadora`, repositorio con `UpdateIfVersion` y ARCO.
- Corrección de drift: movilidades con el catálogo de nv-05 (`a_pie`, `bicicleta`, `auto`, `transporte_publico`), no «bici/transporte». Velocidades en `planes.DefaultVelocidades()` por constructor, no en `.env.example` (sin consumidor que las ajuste). `Contract/Planes 3` pasa a `Unit/Plan_FormaJSON`: sin rutas no hay contrato HTTP. Sin cambios en `docs/endpoints.md` ni `scripts/routes`.
- Decisión: `Recalcular(plan, lugares, velocidades)` es pura y es la única que escribe `traslados` y costos; si falla no deja cálculos a medias. `Calculadora.Aplicar` carga los negocios con `negocios.Repository.FindVisibleMany` (aprobados y no eliminados, solo ubicación y horario). Abierto = `OpenAt` al llegar y en el último minuto de la estancia.
- Decisión propuesta: `precio` de la parada es por persona; `costo_total` = suma × `personas`. Km a 2 decimales y minutos hacia arriba.
- Decisión: versión vieja = `ErrVersionConflict` (409) y plan ajeno o borrado = `ErrNotFound` (404), como nv-05. Ojo nv-30: su tabla dice 412/428; usar 409/400 como el perfil.
- Decisión: ARCO registra `planes.Privacy` antes de `usuarios` (el usuario se borra al final). «Borrar mi historial» no borra planes (contenido, no historial).
- A evitar: guardar un plan sin pasar por `Recalcular`; `type assertion` sin comprobar en pruebas (errcheck de golangci-lint): decodificar a un struct.

## Spec nv-30 — Guardar, leer, editar y repetir planes (2026-11-11)
- Estado: implementada en la rama `nv-30` sobre `nv-29` (PR contra `nv-29`); falta el CI del PR y la firma. DTO, service y handler en `internal/planes`, 10 rutas bajo `/api/v1/planes`, todas con JWT.
- Corrección de drift: la tabla decía 412/428 y 403; se usa 400 sin `If-Match`, 409 con versión vieja y 404 para plan ajeno (no revela que existe), como nv-05. `If-Match` también en las rutas de paradas: cada una cambia la versión del plan.
- Decisión: toda edición pasa por `Service.edit` (carga del dueño, compara versión, cambia, `Calculadora.Aplicar`, `UpdateIfVersion`). «Reordenar» reparte las mismas horas de llegada en el orden pedido, porque el plan siempre va en orden de llegada (nv-29).
- Decisión: `fecha` es un día de `America/Mexico_City` (00:00 local guardado en UTC, `negocios.MexicoCity()`); cambiarla o clonar mueve cada parada los mismos días. Clonar crea ids de parada nuevos y no copia reservas ni integrantes.
- Decisión: pestañas `proximos` (desde hoy, ascendente, por defecto), `pasados` (antes de hoy, descendente) y `guardados` (todos los activos, descendente) con cursor `pestana:fecha` + id; cursor de otra pestaña = 422. Firmado desde nv-17 (ámbito: dueño y pestaña).
- Decisión: las escrituras pasan por `usuarios.RequireActive` (cuenta en eliminación pendiente = 403); los errores de MongoDB salen como 503 sin texto (podría repetir nombre o notas).
- A evitar: devolver 403 para un plan ajeno; usar `apperr` como variable de paquete (se crea en cada llamada).

## Spec nv-31 — Interpretar la frase y armar candidatos (2026-11-11)
- Estado: implementada en la rama `nv-31` sobre `nv-29` (PR contra `nv-29`); falta la firma y el CI del PR. Paquete `internal/asistente` con `POST /api/v1/planes/generar`; doble de IA en `internal/testutil/iatest` y en `authtest.Env.IA`.
- Corrección de drift: el flujo de la spec incluía `armarItinerario` con IA y verificar, que son de nv-32; nv-31 arma por reglas (hasta 3 paradas de los 15 candidatos, giros distintos primero, abierta al llegar y al salir, dentro del presupuesto) y `Recalcular`. No existe `docs/ia.md` y `Recomendar` ya era pública. Sin colección ni índices nuevos.
- Decisión: depende solo de `ia.Modelo`; sin llave la IA queda apagada y la ruta da 503 hasta el respaldo de nv-33. Proveedor sin cupo = 429 con `Retry-After`; lento, bloqueado o inválido tras el reintento = 503; sin candidatos = 422.
- Decisión: al modelo van las instrucciones fijas (`prompts/interpretar.txt`, embebido), el día y la hora y la frase anonimizada (el asistente también llama `ia.Anonimizar`, no solo el adaptador). Nada del perfil, ni ids ni ubicación. Alergias y dieta del perfil son filtro duro siempre; el ánimo se devuelve solo con `privacidad.estado_animo`.
- Decisión propuesta: estancias por giro (restaurante 90, cafetería 60, bar 90, entretenimiento 120, servicios 60, comercio 45); `presupuesto` del plan = por persona × personas; `pareja` = 2 personas.
- Riesgo: la ruta no tiene límite propio (los de IA viven en `ia.Guardia`, nv-28). No fusionar a `main` sin nv-33 o un límite equivalente.
- A evitar: el anonimizador también tapa cifras largas (un ObjectId en la frase sale como `[teléfono]`); en pruebas comparar contra `ia.Anonimizar(frase)`. `staticcheck` S1016 pide convertir structs con los mismos campos (`Parametros(in)`).

## Spec nv-32 — Itinerario con IA y verificación contra el catálogo (2026-11-11)
- Estado: implementada en la rama `nv-32` sobre `nv-31` (PR contra `nv-31`); falta la firma y el CI del PR. Local: build, vet, Unit y `golangci-lint` verdes.
- Corrección de drift: la spec no decía el flujo, los motivos, los campos de `ia_descartes` ni qué viaja al modelo; ahora sí. `Unit/Verificar 10` ya existe con nombre real (`verificar_test.go`).
- Decisión: segundo paso de IA (`GenerarItinerario`, `prompts/itinerario.txt`) con 2 a 4 paradas `{id, llega, minutos}`. `Validar` revisa solo la forma (se reintenta una vez); ids, horas y presupuesto los revisa `Verificar`, que reemplaza en lugar de reintentar para poder medir la alucinación.
- Decisión: `Armar` y `Verificar` comparten `armado` (cabe, agregar, completar). Motivos: `fuera_de_candidatos` (incluye negocios filtrados por alergia: nunca están en la lista), `repetido`, `cerrado`, `presupuesto`, `orden_horas` (antes de poder llegar o más de 60 min de espera). El reemplazo va al final del plan.
- Decisión: `ia_descartes` (`motivo`, `negocio_id`, `modelo`, `creado_en`, `expira_en`), TTL con `expira_en` y `ExpireAfterSeconds(0)` como el resto del proyecto (la prueba de índices lo exige). Sin id ni texto de la persona: fuera de ARCO. Si falla la escritura, el plan sale igual.
- Privacidad: al paso 2 van nombre, giro, precio y horario del día de los candidatos, inicio y presupuesto; en el turno de usuario solo compañía, ánimo (con consentimiento) y palabras clave. Nunca la frase, ubicación ni perfil.
- A evitar: heredocs con comillas y acentos en Bash fallan aunque el delimitador vaya entre comillas; escribir el script de Python en el scratchpad. Las pruebas de integración fijan `fecha` (ahora + 2 h) para poder escribir las horas de la IA con el reloj real.
## Spec nv-24 — Recomendaciones y «No me interesa» (2026-11-11)
- Estado (2026-11-11): implementada en la rama `nv-24` sobre `nv-23` (PR apilado; nv-23 aún sin fusionar). Local: build, vet, Unit, lint, critic, `check_sdd` y `check_endpoints` en verde; Integration, Security y Database las corre el CI. Falta la firma.
- Corrección de drift: el plan decía «ver Diseño» sin archivos; el paquete es `internal/recomendaciones` (no `internal/inicio`). Contratos con 401/403/503 además de 200/422 y 204/404.
- Decisión: `cmd/api` valida los pesos al arrancar (`recomendaciones.NewService`): cierra el pendiente de nv-23. `lat` y `lng` son obligatorios; `hora` HH:MM de hoy en `America/Mexico_City`; la compañía suma términos a los gustos, no filtra.
- Decisión propuesta (privacidad): alergias solo con `datos_sensibles` (regla de nv-05) y solo como filtro duro; dietas siempre filtro duro; gustos y presupuesto guardados solo con `aprendizaje_personalizado`. El ánimo todavía no entra al motor.
- Decisión: `descartes_usuario` con índice único `ux_usuario_negocio`; POST idempotente (404 si el negocio no es público); DELETE de un descarte inexistente o ajeno = 404. Se excluyen los 1000 más recientes en `$geoNear.query`. Entra en ARCO (exportar, borrar historial y cuenta).
- A evitar: olvidar `scripts/routes/main.go` al agregar un módulo (`check_endpoints` no ve sus rutas); leer el cuerpo con `readAll` dentro de los argumentos de `require` y luego decodificarlo (ya se consumió).
- Pendiente: sin prueba Contract propia (el golden se genera con `CONTRACT_CAPTURE=1` y contenedores).

## Spec nv-22 — Búsqueda y autocompletar (2026-11-12)
- Estado (2026-11-12): implementada en la rama `nv-22` sobre `main`; local en verde (build, vet, Unit, lint, critic, `check_sdd`, `check_endpoints`); Integration, Security, Database y Contract las corre el CI. Falta la firma. Paquete `internal/busquedas`: `GET /lugares/autocompletar`, `GET /lugares/buscar` (auth opcional) y `GET/DELETE /busquedas/recientes`.
- Corrección del hallazgo: el índice de texto de MongoDB no tolera errores y Atlas Search no corre en Testcontainers. Primero `$text` en español; si no llena la página, se compara en Go contra nombre, etiquetas y giro de hasta 1000 aprobados (Damerau: 1 letra de 4 a 7, 2 de 8 o más; prefijos con error; todas las palabras) y se marca `aproximada: true`. Con mucho más catálogo: trigramas o Atlas Search.
- Decisión: `q` se reduce a letras y dígitos (`busquedas.Palabras`) antes de `$text`; el autocompletar escapa con `regexp.QuoteMeta`. Así ningún operador ni comodín llega a MongoDB.
- Decisión propuesta (privacidad): `busquedas_recientes` se guarda al buscar con sesión sin pedir `aprendizaje_personalizado` (historial visible y borrable); 10 por usuario, 90 días (TTL), en ARCO. Sin comandos nuevos de Redis.
- Lección (CI del PR 60): `$text` en español no encontró «pizzeria» en «Pizzería Nápoles» (la raíz se calcula con el acento). La comparación en Go lo encuentra, así que `aproximada` solo es `true` si alguna palabra necesitó un error, no por venir de Go.
- A evitar: registrar `busquedas` después de `negocios` (`/lugares/:id` atiende «buscar»); devolver o registrar el error de MongoDB (puede repetir lo escrito).

## Spec nv-25 — Clima e Inicio (2026-11-12)
- Estado: implementada desde `main`, PR contra `main`. Local: build, vet, Unit, `golangci-lint`, critic, `check_sdd` y `check_endpoints` en verde; Integration (`tests/integration/inicio_test.go`) en el CI. Falta la firma.
- Corrección de drift: el plan decía «ver Diseño» sin archivos; ahora nombra `internal/clima`, `internal/inicio` y lo que cambia en `internal/recomendador` y `internal/recomendaciones`. Contratos con 401/422/503 en Inicio y `ciudad_id` en clima.
- Decisión: caché del clima **en memoria por instancia** (30 min por celda de 0,01°, fallo recordado 1 min), no en Redis: el clima no depende del usuario y así no suma comandos de Upstash (`TestPresupuestoRedis_*` sin cambios). A Open-Meteo solo sale la coordenada redondeada.
- Decisión: factor clima con `Contexto.ClimaAdverso` (nil = neutro). Mal clima = lluvia, llovizna, nieve, tormenta, ≥ 0,2 mm, ≥ 32 °C o ≤ 8 °C; aire libre = servicio o etiqueta `terraza`, `jardin`, `patio`, `aire libre`. Solo Inicio pasa el clima; `GET /recomendaciones` sigue sin él.
- Decisión: Inicio con sesión opcional; sin sesión o con cuenta no activa, recomendaciones generales (sin gustos, alergias ni descartes). La franja sale del momento de las recomendaciones (`hora` o el reloj de `recomendaciones.Service`).
- A evitar: un handler no puede importar `bson` (test de arquitectura): la resolución de `ciudad_id` va en `internal/clima/ubicacion.go`. `gocyclo` > 10 con un `switch` de rangos WMO: usar tabla.

## NV-33 — Respaldo, cancelación y «otra idea» (2026-11-11)
- Estado: rama `nv-33` desde `origin/nv-32` con `origin/nv-28` fusionada (trae nv-27 y nv-28); PR contra `nv-32`, se reduce al fusionar #44 y #46. Build, vet, Unit, lint y las cuatro suites con contenedor en verde en local. Falta la medición con IA real (20 frases) y la firma.
- Decisión: toda falla de la IA (tiempo, 429, falla, inválida, bloqueada, tope, gasto o límites desconocidos, límite global o del usuario, IA apagada) responde el motor de reglas con `fuente: reglas` y aviso. Ya no hay 429 ni 503 por la IA. El respaldo no llama al modelo: no cuenta contra el tope ni los límites.
- Decisión: plazo de 7 s para las dos llamadas + 1 s de reserva = 8 s. Respaldo secuencial, no en paralelo como decía la spec: reutiliza la interpretación si el paso 1 respondió y no duplica la consulta al catálogo. `authtest.PlazoIA` = 2 s para las pruebas.
- Decisión: `cmd/api` y `authtest` arman `ia.Guardia` (Redis + `usos_ia`); el asistente usa `Guardia.GenerarValidado` (caché y límite del usuario) con `ia.ConUsuario`. Un plan son 2 generaciones: `IA_LIMITE_HORA` pasa de 5 a 10.
- Decisión: `POST /planes/generar/otra-idea` = cuerpo de generar + `excluir` (1 a 30 ids) sobre `recomendador.Contexto.Excluidos`.
- Límite: Fiber (fasthttp) no avisa cuando el cliente corta; la cancelación llega por el contexto de la petición (60 s o apagado) y el plazo de 7 s acota el gasto.
- Corrección de la pila: `ia.Decodificar` deja el destino en cero si falla (un id inyectado quedaba en la salida; fallaba `Security/IA` de nv-28). En `asistente_test.go` `require.Equal(..., readAll(t, resp))` consumía el cuerpo antes de decodificar (fallaba en nv-32): usar `exigirEstado`.
- A evitar: el CI solo corre en PRs a `main`; los PRs de la pila no prueban Integration. Se pueden correr en local con `docker run --network host -v //var/run/docker.sock:/var/run/docker.sock` (las suites usan `127.0.0.1` con puerto fijo). En PowerShell 5.1 las comillas dobles dentro de `sh -c` se pierden: escribir el script en el scratchpad y montarlo. `Set-Content -Encoding utf8` mete BOM: usar `UTF8Encoding $false`.

## NV-50 — Embeddings de negocios (2026-11-11)
- Estado: rama `claude/friendly-mahavira-0a9451` desde `main`, PR contra `main`. Build, vet, Unit, lint, critic, SDD y endpoints en verde; Integration (`embeddings_test.go`) en el CI. Falta la prueba en vivo con los 30 negocios y la firma.
- Corrección de drift: vectores en la colección `vectores_negocios` (`_id` = negocio), no en un campo de `negocios`: las listas y fichas leen el documento completo. El comando va en `scripts/recalcular-embeddings` (`make embeddings`, `TODOS=1`), como `scripts/indexes`. Los límites no son los de nv-28: el modelo de embeddings tiene su propia cuota en Gemini.
- Decisión: `ia.Vectorizador` (`Documentos`, `Consulta`, `Nombre`) con `ia.GeminiVectores` (`batchEmbedContents`, lotes de hasta 50, 768 dimensiones, renormaliza). `gemini-embedding-2` no acepta `task_type`: documento `title: … | text: …` y consulta `task: search result | query: …` con la frase anonimizada. `Gemini` y `GeminiVectores` comparten `postGemini` (llave en `x-goog-api-key`).
- Decisión: `embedding_version` = plantilla + modelo + huella del texto; el recálculo solo manda los que cambiaron y borra los de negocios no visibles. Si falla a la mitad, lo guardado queda y el siguiente sigue.
- Decisión: `IA_EMB_LIMITE_MIN` 80 (máx. 100) e `IA_EMB_LIMITE_DIA` 800 (máx. 1000), `rate:ia_emb_min/dia`, cada texto cuenta uno (`LimitadorRedis.Sumar` con `INCRBY`, mismos 3 comandos). AI Studio no publica la cuota gratuita de `gemini-embedding-2`: se tomó la de `gemini-embedding-001`; confirmarla en AI Studio.
- Decisión: la importación llama `Refrescar` con plazo propio de 30 s y sin la cancelación de la petición; si falla solo se registra. `IA_EMBEDDINGS_ID` sin valor por defecto: sin él no hay vectores.
- Operación pendiente: crear `/wherever/dev/IA_EMBEDDINGS_ID` (`gemini-embedding-2`) en SSM antes de usarlo en AWS.
- A evitar: asignar un `*Recalculador` nil a la interfaz `negocios.Embeddings` (queda no nil): `cmd/api` lo revisa antes. `ia.Normalizar` ya existía (texto): el de vectores es `ia.NormalizarVector`.

## NV-51 — Búsqueda semántica y evaluación (2026-11-11)
- Estado: rama `nv-51` desde `main` (ya trae nv-50, #53), PR contra `main`. Local: build, vet, Unit, lint, critic, `check_sdd` y `check_endpoints` en verde. Integration (`semantica_test.go` y los puntajes nuevos de `recomendador_test.go`) en el CI. Falta la medición con IA real y la firma.
- Corrección de drift: el plan decía «ver Diseño» y el Diseño solo hablaba de la caché. La consulta no cuenta contra el tope de gasto de nv-27 (ese es del modelo de texto), sino contra `IA_EMB_LIMITE_MIN/DIA` de nv-50.
- Decisión: `internal/semantica`, producto punto en la API detrás de `semantica.Indice` (el MongoDB local no tiene `$vectorSearch`). La afinidad va de 0 a 1 entre el menos y el más parecido (mín–máx), porque los cosenos caen en una franja angosta. El vector de la frase se guarda 24 h en `cache:ia:emb_{huella}` (reusa `ia.CacheRedis`; la huella no lleva usuario ni frase).
- Decisión: `afinidad_semantica` pesa 0,20 y los seis pesos de nv-23 se multiplicaron por 0,8. Sin semántica el factor es 0,5 para todos y el orden queda igual. Los puntajes absolutos bajan: `0,8 × antes + 10`.
- Decisión: el respaldo de la búsqueda puntúa con los factores del motor sin sus filtros duros y sin ubicación, con las palabras de la frase como gustos. `recomendador.Ordenar` filtra los cerrados: de noche la lista quedaba vacía.
- Decisión: el asistente pide la afinidad con `WithSemantica` dentro del plazo de la IA. Por defecto usa un `sinSemantica` (no nil), así `generar` no suma ramas (gocyclo). `embeddings.Vectorizador` arma el vectorizador limitado para el recálculo y la búsqueda (mismos contadores).
- Ruta: `semantica.NewHandler` va antes que `negocios` en `cmd/api`, `authtest` y `scripts/routes`. Si va después, `/lugares/:id` atiende «semantica» con 404.
- Evaluación: `scripts/evaluar-semantica` con 30 negocios ficticios y 30 frases (el CSV del piloto aún no existe). Mide top-5 y top-1 de reglas, semántica y reglas + afinidad. Pendiente: el perfil `wherever-dev` no tiene `ssm:GetParameter` sobre la llave. Hay que correrlo con un perfil que sí lo tenga (comando en `docs/ml-evaluacion.md`).
- A evitar: heredocs de Bash con acentos y comillas fallan; usar Write o un script de Python en `$TEMP`. Docker Desktop está en `%LOCALAPPDATA%\Programs\DockerDesktop`.
## NV-34 — Nombre de usuario y privacidad de búsqueda (2026-11-11)
- Estado: implementada desde `main`; local build, vet, Unit, lint, critic, `check_sdd` y `check_endpoints` en verde. Las suites con contenedor las corre el CI del PR. Falta la firma.
- Drift corregido: el plan creaba `internal/amistades` y `internal/personas` (son de nv-35/36) y tocaba el purger y la matriz de límites (no hay colección nueva).
- Decisión: `nombre_usuario` solo minúsculas, números y guion bajo (3 a 20); las mayúsculas son 422, no se convierten, y así `ux_nombre_usuario` (único parcial) ya es único sin importar mayúsculas. Lista corta de reservados en la regla `username`.
- Decisión: `privacidad.buscable_por_usuario` es `*bool` con `omitempty`; ausente = encendido (`Privacy.SearchableByUsername`). Para buscar en Mongo (nv-35) usa `{$ne: false}`, nunca `true`.
- Contract: `nombre_usuario` y `buscable_por_usuario` entran en `yo_200`, `yo_patch_200` (el PATCH del contrato ahora manda `nombre_usuario`), `privacidad_200`, `cancelar_eliminacion_200` y `exportacion_descargar_200`.

## NV-35 — Buscar personas (2026-11-11)
- Estado: rama `nv-35` desde `main`, PR contra `main`. Local: build, vet, Unit, lint, critic, `check_sdd` y `check_endpoints` en verde; Integration (`personas_test.go`) y Security (`personas_security_test.go`) las corre el CI.
- Drift corregido: el límite de 20 solicitudes por hora, ocultar por bloqueo entre personas y "no aceptar solicitud ajena" pasan a nv-36 (no existe `amistades` todavía). nv-36 debe filtrar `GET /personas/buscar` con las relaciones `bloqueada`.
- Decisión: `internal/personas` con consultas en `internal/usuarios/search.go`; `usuarios.HashPhone` es el único HMAC del teléfono (account lo usa también). `q` = teléfono si empieza con `+` o trae ≥10 dígitos (10 = +52); si no, prefijo de usuario (minúsculas, `@` opcional). Solo `activo`, nunca quien busca, sin correo ni teléfono en la respuesta.
- Decisión: `Limiter.WindowUser` cuenta por `sub` del token (`rate:personas_buscar:{usuario}`, 30/h); va después de `RequireAuth`. nv-36 lo reutiliza para las solicitudes.
- A evitar: escribir archivos con Python en Windows sin `newline=''` los deja en CRLF y gofmt los marca.

## NV-36 — Amistades (2026-11-11)
- Estado: rama desde `main`, PR contra `main`. Local: build, vet, Unit, lint, critic, `check_sdd` y `check_endpoints` en verde; Integration (`amistades_test.go`), Security, Database (índices) y Contract (`amistades: []` en la exportación) las corre el CI.
- Decisión: un documento por par (`par` = ids ordenados, `ux_par`); la solicitud cruzada que pierde la carrera relee y acepta. En `bloqueada`, `de` es quien bloqueó. Rechazar borra el documento sin avisar.
- Decisión: quien fue bloqueado ve 404 en todo (como si no existiera); quien envió recibe 404 al intentar aceptar su propia solicitud. Orden de la lista por `actualizada_en` de la relación: no hay campo de actividad del usuario.
- Decisión: `personas.NewService(users, blocks, llave, log)` recibe `personas.Blocks` (lo cumple `amistades.Repository`); las consultas de `usuarios/search.go` reciben `exclude []bson.ObjectID` con `$nin`. El bloqueo oculta en ambos sentidos.
- A evitar: en `cmd/api/main.go` `buildModules` está al límite de gocyclo (10): los módulos nuevos se arman en una función aparte (`buildSocial`).

## NV-52 — Proyección de amistades y planes a Neo4j (2026-11-12)
- Estado: rama desde `main`, PR contra `main`. Local: build, vet, Unit, lint, critic, `check_sdd` y `check_endpoints` en verde; Integration (`grafo_test.go`), Security, Database (`grafo_pendiente`) y Contract las corre el CI.
- Decisión: la cola `grafo_pendiente` guarda a quién recalcular (un documento por usuario), no eventos; el trabajador lee Mongo y deja `AMIGO_DE`/`FUE_A` iguales. Porqué: idempotente, sin problemas de orden ni eventos perdidos; recalcular a quien actuó basta porque `AMIGO_DE` es una por par (se consulta sin dirección).
- Decisión: el trabajador corre dentro de la API (`startGraph` en `cmd/api/main.go`), no en `cmd/worker`, porque no hay hosting de trabajadores. La cola reparte entre instancias con `lease` de 1 min.
- Decisión: `make reconstruir-grafo` va en `scripts/`, como `make siembra`. Tras fusionar, correr `make siembra` o `make reconstruir-grafo` contra Aura para crear la restricción `negocio_id`.
- Para nv-53: `(:Usuario {id})-[:AMIGO_DE]-(:Usuario)` sin dirección y `(:Usuario)-[:FUE_A]->(:Negocio {id})`, ids en hex. Los nodos de demostración de nv-11 (`Lugar`, `Categoria`, ids `demo-*`) siguen ahí.
- A evitar: `Touch` nunca cambia la respuesta (`_ =`); las operaciones de la cola no usan el plazo de Neo4j, o un reintento vencido no se reagenda.

## NV-53 — Sugerencias y señal social (2026-11-12)
- Estado: rama desde `main`, PR contra `main`. Local: build, vet, Unit, lint, critic, `check_sdd` y `check_endpoints` en verde; Integration (`sugerencias_test.go`), Security (`sugerencias_security_test.go`, ahora con Neo4j) y Contract las corre el CI.
- Decisión: `internal/sugerencias` (service + handler) lee Neo4j; Mongo manda en relaciones (`ListByUser`: toda relación se excluye de sugerencias; solo `aceptada` cuenta como amigo en la señal social) y en `buscable_por_usuario` (`usuarios.FindSuggestableByIDs`). El grafo da hasta 100 candidatos y Mongo filtra hasta 20.
- Decisión: sin la caché de 10 min de la spec. Porqué: nv-18 es de la ola 4 y la caché mostraría hasta 10 min a alguien recién bloqueado; además añadiría una clave de Redis fuera de la constitución §6.
- Decisión: Neo4j caído o lento (3 s) responde 200 con `degradado: true` en las dos rutas. El módulo va antes que amistades: el grupo `/amistades` ya pide JWT.
- A evitar: `buildModules` sigue en el límite de gocyclo; lo social se arma en `buildSocial(socialDeps)` y devuelve sus módulos.

## Spec nv-19 — Aceptación de términos y aviso (2026-11-12)
- Estado (2026-11-12): implementada, pendiente de la firma del revisor. Local verde (build, vet, Unit, lint, critic, sdd, endpoints); Integration, Security, Database y Contract en el CI del PR.
- Decisión: se aplica el hallazgo de la revisión. `version_terminos`, `fecha_aceptacion` y `promociones_correo` ya vivían en `usuarios.privacidad` (nv-05); solo se agrega `privacidad.version_aviso` y el historial va aparte en `consentimientos` (solo agregar, `ix_usuario_fecha`). Repositorio y ARCO en `internal/usuarios/consentimientos.go`; service y handler en `internal/usuarios/account/consents.go`.
- Decisión: `POST /usuarios/yo/consentimientos` solo acepta las versiones vigentes de `config.VersionApp` (las mismas de `GET /configuracion`); otra da 422. `privacidad.consentimiento_vigente` sale en el perfil y en `PATCH /privacidad`, no en la exportación.
- Decisión: "Borrar mi historial" conserva los consentimientos (prueba del consentimiento); el borrado de la cuenta los elimina.
- Efecto: las cuentas previas y las nuevas nacen sin `version_aviso` y con `consentimiento_vigente` falso: la app debe pedir la aceptación en el primer inicio tras el despliegue. El registro y `PATCH /privacidad` siguen aceptando `version_terminos` sin dejar fila en el historial.
- Sin Redis: no cambia `TestPresupuestoRedis_*`. La dependencia con nv-18 no era real.
- A evitar: correr `gofmt -w` sobre todo `internal/` con Go 1.27.3 reformatea `internal/shared/apperr/apperr.go` (alineación de un mapa); limitarlo a los archivos tocados.

## NV-37 — Grupos e integrantes (2026-11-12)
- Estado (2026-11-13): fusionada en `main` (PR #69) con el CI completo. Local: build, vet, Unit, lint 0, critic, `check_sdd` y `check_endpoints` (83 rutas) en verde. Falta la firma.
- Corrección de drift: unirse por enlace es de nv-38, votaciones y su cierre de nv-39 y notificaciones de nv-40/41; la spec y el plan ya no los piden aquí.
- Decisión: un grupo por plan (`ux_plan`); el organizador va en `integrantes` (`organizador`/`confirmado`) y cuenta en los 12. `ocupados` = integrantes que no declinaron y solo cambia con un `$inc` en la misma operación que cambia `integrantes`, con `ocupados < 12` en el filtro. Probado con 20 invitaciones simultáneas al repositorio.
- Decisión: solo amigos `aceptada` con cuenta activa; un solo 422 para no amigo, inactivo o uno mismo (no revela cuál). Quien declina libera su lugar, sigue viendo el grupo y puede volver a confirmar si hay cupo; los demás integrantes no lo ven.
- Decisión: `planes.Service.WithMembers(grupos.Repository)`: `GET /planes/{id}` lo leen invitados y confirmados; ediciones, borrado y clonado pasan por `getOwned` (dueño). `planes` no importa `grupos`.
- Decisión: `grupos` va antes que `planes` en la lista de módulos (`cmd/api`, `authtest`, `scripts/routes`): atiende `POST /planes/{id}/invitaciones` sin pasar por el grupo `/planes`.
- Límite nuevo: 30 invitaciones por hora por usuario (`rate:grupos_invitacion:{usuario}`), +3 comandos de Redis por invitación (limitador).
- A evitar: probar la concurrencia del tope por HTTP con un solo organizador; el límite de 10 escrituras simultáneas da 429 antes.

## NV-38 — Enlace de invitación y vista pública (2026-11-12)
- Estado (2026-11-13): fusionada en `main` (PR #70). Local: build, vet, Unit, critic, `check_sdd` y `check_endpoints` (87 rutas). Falta la firma.
- Corrección de drift: el código iba en la ruta (`/planes/compartidos/{codigo}`, `/grupos/unirse/{codigo}`), pero `Errors` registra `ruta` en cada línea de acceso (y CloudFront también). Ahora va en el cuerpo (`POST {codigo}`) y en el fragmento del enlace (`{APP_LINK_BASE}/plan#codigo=…`), como los tokens de correo de nv-06. Se agregó `DELETE /planes/{id}/compartir` para revocar.
- Decisión: un código por grupo en `grupos.codigo_hash` (SHA-256, `ux_codigo_hash` único parcial) y `codigo_vence`; crear otro reemplaza al anterior. Compartir crea el grupo si falta.
- Decisión: unirse no pide amistad (el enlace es la invitación) pero sí cuenta activa y que no haya bloqueo con el organizador (404). Entra o vuelve confirmado con `ocupados < 12` en la misma operación. Vencido: vista 404, unirse 410.
- Decisión: `POST /planes/{id}/compartir` va en `carriesTokens` (sin compresión): responde el código.
- Límites nuevos: vista pública 30/min por IP y unirse 30/h por usuario; +3 comandos de Redis por petición (limitador).
- Para la app: leer `codigo` de `location.hash` y mandarlo por `POST`; no ponerlo en la URL. `APP_LINK_BASE` cambia si se recrea CloudFront.

## NV-39 — Votaciones (2026-11-12)
- Estado (2026-11-13): fusionada en `main` (PR #71). Local: build, vet, Unit, critic, `check_sdd` y `check_endpoints` (91 rutas). Falta la firma.
- Decisión (cierra el hallazgo de Redis): el vencimiento lo cierra un `Closer` por instancia cada minuto que solo consulta MongoDB (`ix_estado_cierra`). No hace falta `lock:` porque cerrar es un `UpdateOne` condicionado a `estado = abierta` y `votos_version`: 0 comandos de Redis en lugar del ~7 % estimado. Listar o votar una vencida también la cierra.
- Decisión: `votos` es un mapa `{usuario_hex: negocio_id}` con `votos_version` que sube en cada voto. No condicionar el cierre al mapa: Go codifica los mapas en orden aleatorio y la igualdad de documentos en MongoDB depende del orden.
- Decisión: la ganadora se aplica con `planes.Service.ReplaceBusiness` (lee la versión y reintenta una vez si otra edición gana). Si el plan o la parada no existen o la ganadora no abre a esa hora, `aplicada: false` con `motivo`; si falla la base, `aplicada` queda sin valor y el `Closer` lo reintenta.
- Decisión: `votaciones` va antes que `grupos` en la lista de módulos (`/grupos/:id/votaciones`).
- A evitar: votar quien declinó (403); decir quién votó qué en la respuesta.
- Lección: la prueba de dos instancias encontró que cerrar era único pero aplicar no: las dos veían «cerrada sin aplicar» y cambiaban el plan. `ClaimApply` reserva la aplicación (`aplicando_hasta`, 1 min) con un `UpdateOne` condicionado.

## NV-40 — Dispositivos y centro de notificaciones (2026-11-12)
- Estado (2026-11-13): fusionada en `main` (PR #72). Local: build, vet, Unit, critic, `check_sdd` y `check_endpoints` (98 rutas). Falta la firma.
- Corrección de drift: el token de FCM iba en la ruta (`/dispositivos/{fcm_token}`); ahora `PUT/DELETE /dispositivos` con `{token}` en el cuerpo, por el log de acceso.
- Decisión: `notificaciones.Notifier` (`Emit`) lo reciben amistades, grupos y votaciones con `WithNotifier` (por defecto `Noop`). `Emit` guarda y, si falla, solo registra el tipo: nunca cambia la respuesta de la acción.
- Decisión: `datos` solo lleva ids; la app arma el texto. TTL de 90 días (`expira_en`).
- Decisión: el token se guarda (nv-41 lo necesita) y se busca por SHA-256 (`ux_token_hash`). Si era de otra cuenta, pasa a la nueva. El sexto teléfono es 422.
- Para nv-41: el push sale del mismo `Emit` (envolverlo, no llamar a FCM desde los módulos); falta `RemoveDeviceByHash` para los tokens `UNREGISTERED`.
- A evitar: heredocs largos de Python con comillas y acentos en Bash (fallan); escribir el script en el scratchpad y correrlo con `python -I archivo.py "$(pwd -W)"`.

## NV-41 — Push con Firebase (2026-11-12)
- Estado (2026-11-13): fusionada en `main` (PR #73). Implementada y probada contra `internal/testutil/fcmtest` (servidor local con llave propia). Falta la prueba en vivo: el equipo crea el proyecto de Firebase y carga las credenciales (nunca por el chat). Falta la firma.
- Decisión: cliente propio de FCM HTTP v1 (`internal/platform/push`) que firma el JWT RS256 con `golang-jwt` y cambia el token en `token_uri`; sin librerías de Google. Token de acceso en memoria hasta 1 min antes de vencer (`Reset()` con mutex).
- Decisión: credenciales en `FCM_CREDENCIALES_B64` (JSON en base64, una línea) con `FCM_PROJECT_ID`; van juntas o ninguna, y sin ellas no hay push. Corrige el hallazgo de la revisión (`fcm-service-account` no era nombre de variable y el JSON tenía varias líneas). `FCM_URL` solo fuera de prod.
- Decisión: el push sale del mismo `Emit` que el centro y solo si el aviso se guardó. Cola de 500 y 4 trabajadores; 3 intentos solo en fallos temporales; `UNREGISTERED` borra el token.
- Decisión: los tipos de grupo siguen `preferencias.notificaciones.grupos`; los de amistad no tienen interruptor en nv-05 y siempre suenan fuera del horario silencioso (hora de México).
- Operación: PowerShell `[Convert]::ToBase64String([IO.File]::ReadAllBytes("llave.json"))`; en AWS, `/wherever/dev/FCM_PROJECT_ID` (String) y `/wherever/dev/FCM_CREDENCIALES_B64` (SecureString); la siguiente instalación de imagen los toma.

## NV-46 — Descarga de cambios (2026-11-12)
- Estado (2026-11-13): fusionada en `main` (PR #74). Local: build, vet, Unit, critic, `check_sdd` y `check_endpoints` (99 rutas). Falta la firma.
- Decisión: contador `sync_estado.seq` por usuario con `$inc` atómico; `planes.Service.WithSync` numera alta, edición y borrado (`planes.sync_seq`, `ix_dueno_sync`). `startVotes` también lo usa: la ganadora de una votación cambia el plan.
- Decisión: los ajustes no se numeran; viajan cuando cambia `usuarios.version` (todas las ediciones del perfil pasan por `apply`, que la sube). `account.SettingsOf` da la misma forma que `GET /usuarios/yo`.
- Decisión: cursor `desde` firmado (nv-17) con `seq:version:emitido_unix` (hora del servidor). Más de 30 días = 410 con `campos[{desde: requiere_completa}]`.
- Límite aceptado: dos ediciones del mismo usuario en el mismo milisegundo pueden confirmarse fuera de orden; quien sincroniza en medio puede saltarse una hasta la siguiente edición.
- Para nv-47: aplicar acciones con los mismos `planes.Service` (ya numeran) y `account` (ya sube la versión).

## NV-47 — Acciones sin conexión (2026-11-12)
- Estado (2026-11-13): fusionada en `main` (PR #75). Local: build, vet, Unit, lint 0, critic, `check_sdd`, `check_endpoints` (100 rutas) e Integration de sincronización en verde. Falta la firma.
- Decisión: cada acción se reclama insertando en `sync_acciones` (`ux_usuario_cliente`, TTL 7 días) antes de aplicarla; repetir responde `duplicada` con `resultado_original`. Se aplica con los mismos `planes.Service` y `account.Service` de la API.
- Decisión: si la base falla a la mitad, lo aplicado queda y la acción que falló se suelta (`Release`): 503 y reintentar es seguro.
- Corrección de drift: 101 acciones = 422 (no 413); conflicto de ajustes por `usuarios.version` (sus PATCH no llevan If-Match).

## NV-48 — Capa de cifrado de campos (2026-11-12)
- Estado (2026-11-13): fusionada en `main` (PR #76). Paquete `internal/platform/cifrado` y `DATA_KEYS` en la configuración; lo usa nv-49 (PR #77, abierto). Falta la firma.
- Decisión: `DATA_KEYS` = `id:base64,…` (32 bytes cada una); la primera cifra y todas descifran. Dato asociado de GCM = colección, campo e id del documento. Formato `enc:v1:<id>:<nonce>:<cifrado>` en base64url.
- Decisión: obligatoria en prod (no arranca); en dev y staging puede faltar y nv-49 guarda en claro con aviso. El servidor de AWS es staging: cargar `/wherever/dev/DATA_KEYS` (SecureString) para que cifre allá.
- Corrección de drift: el ánimo no se guarda (solo el consentimiento `estado_animo`); se cifran alergias y fecha de nacimiento.
- Medido: ~1,4 µs por cifrar y descifrar un valor.
- A evitar: varios heredocs `<<'EOF'` en un mismo comando de Bash con acentos y comillas; fallan con «unexpected EOF». Usar Write para los documentos.

## NV-49 — Cifrar datos sensibles (2026-11-12)
- Estado (2026-11-13): fusionada en `main` (PR #77). Falta la firma, cargar `DATA_KEYS` en `/wherever/dev/` y correr la migración en ese entorno.
- Decisión: la capa vive en `usuarios.Repository` (`WithCipher`): `Create` y `apply` cifran, `findOne` y `apply` descifran. `account`, `recomendaciones`, `asistente` y la exportación no cambian.
- Decisión: `User.BirthDate` (`bson:"-"`) queda en claro para el código y `BirthDateStored` (`any`) va a la base: texto cifrado o la fecha de antes. `mayor_de_edad: true` al registrar.
- Decisión: un valor que no abre = `ErrCipher` (503); nunca se devuelve el texto cifrado como dato. Un valor copiado de otra cuenta no abre (dato asociado con el id).
- Migración: `make migrar-cifrado` (cuenta) y `make migrar-cifrado APLICAR=1`; idempotente, también rota, y no sube `version`.
- A evitar: construir un `usuarios.Repository` de producción sin `WithCipher` (leería el texto cifrado como alergias y el filtro de nv-23 dejaría solo `["ninguno"]`); hoy son `cmd/api` (`users` y `startVotes`) y `authtest`.

## Estado de las olas (2026-11-13)
- Olas 5 (nv-37 a nv-41) y 7 (nv-46 a nv-49) completas en `main` (PRs #69 a #77). Las nueve siguen sin firma.
- Sin ejecutar: ola 6 (nv-42 → 43 y nv-44 → 45; nv-42 primero porque las otras tres dependen de ella; revisar antes el presupuesto de US$50 por Rekognition, S3 y CloudFront).
- Olas nuevas (2026-11-13, Osmar Reséndiz) para las specs que no entraban en las 7, ordenadas por dependencias:
  - Ola 8, sin bloqueos (ya tienen lo que necesitan en `main`): nv-54 (favoritos), nv-58 (La Vaca), nv-64 (errores de la app y código de amigo), nv-57 (chat de grupo). Corre en paralelo con la ola 6.
  - Ola 9, negocios aliados (hoja A5): escribir y ejecutar nv-55 (alta por pasos, INE, datos fiscales, CLABE, revisión del admin) → nv-56 (panel del negocio). Es la que más bloquea: de ella dependen nv-59, 60, 61 y 62. Escribir las specs puede ir en paralelo con las olas 6 y 8.
  - Ola 10, después de las olas 6 y 9: nv-59 → 60 (reservas y Stripe), nv-61 (cupones, necesita nv-56), nv-62 (check-in y reseñas, necesita nv-56 y la ola 6 completa) y nv-63 (postales, necesita nv-42, 43 y 45).
  - Si el tiempo no alcanza el 2 de diciembre, lo que quede es de la ola 10.
- WIP sin fusionar, salidos de una base vieja (nv-37): `origin/nv-54`, `origin/nv-64` y `claude/nv-58-vaca` (solo local). Traerles `main` antes de seguir.
- Pendiente del operador antes de desplegar: `/wherever/dev/DATA_KEYS` (nv-48/49) y después `make migrar-cifrado APLICAR=1` en ese entorno; `/wherever/dev/FCM_PROJECT_ID` y `FCM_CREDENCIALES_B64` (nv-41, van juntas o ninguna). Ninguna es obligatoria en staging: sin ellas la API arranca, guarda en claro y no manda push. `deploy-env.sh` lee todo `/wherever/dev/`, así que basta con instalar la siguiente imagen.
- A evitar: fiarse del `main` local; el 2026-11-13 iba 21 commits atrás de `origin/main`. Hacer `git fetch` antes de revisar el estado.

## NV-42 — Evaluación de imágenes y texto (2026-11-13)
- Estado: rama `nv-42` desde `main`, PR contra `main`. Paquete `internal/moderacion` (`Moderador.Evaluar`, `Rekognition`, `TextoIA`, reglas) y doble `internal/testutil/rekognitiontest`. Local: build, vet, Unit, lint, critic, `check_sdd` y `check_endpoints`. Falta la firma y la prueba en vivo (`MODERACION_LIVE=1`), que espera el permiso del deployer.
- Decisión: `Evaluar` no devuelve error; cualquier falla (adaptador, reglas, contenido vacío, imagen > 5 MB, texto > 4000) deja `cuarentena` con `Falla`. Si otra parte rechaza, gana el rechazo.
- Decisión: sin Comprehend. Los datos personales salen de los patrones de `ia.Anonimizar` (puntaje 60, cuarentena); una palabra bloqueada rechaza sin gastar llamada al modelo.
- Decisión: AWS SDK v2 para Rekognition (firma, IMDSv2, un reintento) en lugar de un cliente propio. Solo cuentan desnudez, violencia, drogas y odio; alcohol y traje de baño no, por bares y balnearios. Se sube cada etiqueta a su raíz por `ParentName`.
- Decisión: sin cableado ni variables nuevas; lo arma nv-45 con la `ia.Guardia` para el texto. Umbrales fijos (`Fijas`) hasta que nv-43 los lea de `moderacion_reglas`. Pruebas en Unit: el paquete no usa bases.
- Costo: US$0.001 por imagen; con unas 300 fotos al mes, US$0.30 sobre unos US$41 de infraestructura. nv-45 modera una vez por original y pone límite de subidas.
- Operación (antes de nv-45 en AWS, no bloquea este PR): un administrador sube versiones nuevas de `wherever-dev-boundary` y `wherever-dev-deployer-policy` (el deployer no puede cambiar el boundary) y luego `terraform apply`. Recomendado: exclusión de servicios de IA de AWS Organizations.
- A evitar: bajar el umbral de cuarentena por debajo de 40 sin bajar `confianzaMinima` (Rekognition no devuelve etiquetas menores). Publicar contenido de usuarios con moderación sin antes agregarla al aviso y subir `AVISO_VERSION` (nv-45).

## NV-44 — Bucket de imágenes y CloudFront (2026-11-13)
- Estado: rama `claude/nv-44-0bfc9a` desde `origin/main`, PR contra `main`. `infra/aws-dev/imagenes.tf` (bucket privado, OAC, caché de un año, segunda distribución), política en línea `imagenes` en el rol, `ImagenesDelProyecto` en el boundary y política nueva `wherever-dev-deployer-imagenes`. Local: `fmt`, `validate`, Trivy en 0, critic, `check_sdd`, `check_endpoints`. Sin Go. Falta la firma y que el operador aplique.
- Decisión: `force_destroy` en el bucket. Porqué: `destruir.sh` fallaba con un bucket con objetos; las imágenes del entorno de desarrollo no se respaldan.
- Decisión: CloudFront solo lee `i/*` (política del bucket con `AWS:SourceArn`); `cuarentena/` se vence a 1 día. Sin `s3:ListBucket` en el rol: una clave inexistente da 403.
- Decisión: SSE-S3 y no KMS, sin versiones ni registros de acceso (costo; excepciones de Trivy con motivo). Política de caché propia con TTL mínimo de un año: los nombres son inmutables.
- Decisión: los permisos de CloudFront y S3 del deployer van en `iam/wherever-dev-deployer-imagenes.json`, porque la del deployer ya no admite más (6144). Drift: los permisos de balanceador y CloudFront que crearon nv-12 no están en el repo; esta política solo agrega lo de imágenes.
- Costo: CloudFront en capa gratuita; S3 unos US$0.05 al mes. Total estimado unos US$41.35 de US$50 (infraestructura US$41 + Rekognition US$0.30).
- Operación, en orden (antes de nv-45 en AWS, no bloquea este PR): 1) administrador: versión nueva de `wherever-dev-boundary` (JSON del repo) y política nueva `wherever-dev-deployer-imagenes` adjunta a `wherever-deployer`; esperar a que IAM propague. 2) `crear.sh --solo-plan`: solo recursos nuevos, sin reemplazar instancias. 3) `crear.sh`. 4) `verificar.sh` (salida `verificar`, ya con `--imagenes`). 5) Al fusionar nv-45: `/wherever/dev/IMAGENES_BUCKET` e `IMAGENES_URL` (salidas de Terraform) e `instalar-actualizador.sh` en las dos instancias.
- A evitar: guardar algo que no se pueda perder en el bucket de desarrollo (`destruir.sh` lo vacía). Publicar en otra ruta que no sea `i/`: CloudFront no la sirve.

## Pendientes de decidir
- Proveedor del catálogo de lugares (Google Places, Foursquare u OSM): licencias.
- Proveedor del modelo de lenguaje (API o modelo local).
- D-1: cuenta nueva por Google sin `fecha_nacimiento` (propuesta en `specs/nv-04/spec.md`; dueño A1).
- Proveedor de correo (SMTP o servicio) y su remitente (`MAIL_FROM`).
- Firma de `nv-00…nv-03`: siguen en Borrador con evidencia. Desde nv-04 el revisor es Osmar Reséndiz (único integrante de backend; el Excel nombra a Jorge Alberto Rodríguez Enriquez para las hojas J y a Alexander Dominguez Zarate para las A como responsables del diseño).
- Zona horaria por ciudad (documento de backend) contra `America/Mexico_City` fija (Excel): la constitución toma la zona de la ciudad con ese valor por defecto.

## Mapa de fases
- `nv-00` = constitución, stack y gobernanza · `nv-01` = foundation/pipeline/salud · `nv-02` = molde DTO→validator→service · `nv-03` = puerta de calidad incremental (advisory). Módulos de negocio desde `nv-04`, empezando por auth (J1) y usuarios (A1); el resto sigue el orden de producto del documento de backend: planes (A4), grupos (J3), social (J2), lugares (A2) y luego lo demás.
