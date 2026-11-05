# NV-08 — Despliegue continuo (CD) al entorno de desarrollo

## Contexto
Hoy el backend tiene CI (`ci-pr.yml`, `nightly.yml`) y despliegue manual (`infra/aws-dev/scripts/desplegar.sh`, nv-07). La constitución (§3 y §8) y nv-07 dejaron el CD fuera de alcance. Esta spec lo abre **solo para el entorno de desarrollo**: cada cambio que llega a `main` con CI en verde se publica solo en el servidor, y se regresa solo a la versión anterior si no queda sano. Producción sigue sin CD. **Depende de**: NV-07 (EC2, Parameter Store, Caddy) y del CI en verde.

**Por qué el servidor jala la versión y GitHub no se la empuja:** el primer diseño usaba OIDC y Systems Manager, pero una SCP de la organización de AWS niega `iam:GetOpenIDConnectProvider` y Organizations no está disponible para este proyecto, así que no se puede levantar. Con este diseño GitHub no necesita ningún acceso a AWS.

## Requisitos
1. El CD arranca solo cuando `ci-pr` termina con éxito por un `push` a `main` de este repositorio. Un PR, una rama, un fork o un CI cancelado nunca publican.
2. La imagen sale del `Dockerfile` del commit que pasó CI, con ese SHA como etiqueta y como `version` de `/health` (`--build-arg VERSION=<sha>`). Se elimina la excepción del binario compilado a mano.
3. GitHub no tiene ninguna credencial de AWS ni puede mandar comandos al servidor.
4. La imagen se publica en GitHub Container Registry (GHCR) con el `GITHUB_TOKEN` temporal del workflow. La EC2 la baja con un token de solo lectura guardado en Parameter Store.
5. La EC2 revisa cada minuto si la etiqueta `dev` cambió de digest, y también al arrancar (`OnBootSec=1min`).
6. Los índices declarados se crean con el script idempotente antes de cambiar la API.
7. Si la versión nueva no da `/ready` 200, el servidor regresa solo a la anterior y registra el digest fallido para no reintentarlo.
8. El servidor no empieza una actualización de 21:50 a 22:00 (hora de Ciudad de México), para que el apagado no la corte a la mitad.
9. Un solo despliegue a la vez, en el workflow y en el servidor.
10. Se puede regresar a mano a una versión anterior con `workflow_dispatch` indicando su SHA, solo desde `main`: GHCR re-etiqueta esa versión como `dev` y el servidor la instala sola. Si la imagen ya no existe, se reconstruye solo si el SHA es ancestro de `main`.
11. Toda entrada `sha` debe cumplir `^[0-9a-f]{40}$` antes de usarse y nunca se interpola en un comando.
12. Volver a ejecutar el workflow del mismo SHA no falla: si la etiqueta ya existe, se salta la construcción.
13. Todo rol que cree Terraform lleva el permissions boundary `wherever-dev-boundary`.
14. GHCR conserva solo las últimas 5 versiones.
15. El workflow informa el resultado consultando `/health` por HTTPS, según la tabla de Contratos.

## Diseño
- **Workflow:** `.github/workflows/cd-dev.yml`, aparte de `ci-pr.yml`, que sigue "sin deploy". Se dispara con `workflow_run` sobre `ci-pr` y con `workflow_dispatch` (entrada `sha`). Con `workflow_run` exige `conclusion == success`, `event == push`, `head_branch == main` y que `head_repository` sea este repositorio. Hace checkout del `head_sha`, no de la punta de la rama. Acciones fijadas por SHA, y `permissions` mínimos (`contents: read`, `packages: write`).
- **Imagen:** `ghcr.io/whereverapp2026/wherever-backend`, privada, ligada al repositorio con la etiqueta `org.opencontainers.image.source`. Se construye con `--provenance=false`: las atestaciones crean manifiestos extra que `delete-package-versions` contaría como versiones, y borrarlos rompería la imagen. Una etiqueta inmutable por SHA y la etiqueta móvil `dev`.
- **Versión:** `cmd/api/main.go` ya lee `main.version` y el `Dockerfile` ya lo fija con `ARG VERSION`; **sin cambio de código** ni de contrato.
- **Entrada `sha`:** llega al paso por variable de entorno y se valida antes de cualquier otro paso, sin `${{ inputs.sha }}` dentro de un `run`.
- **Token de lectura:** PAT clásico con solo `read:packages`, en `/wherever/dev/GHCR_TOKEN` (SecureString), creado y cargado por el dueño. **`deploy-env.sh` lo excluye del `.env`**: ese script vuelca toda la ruta, y sin el filtro el token llegaría a las variables de entorno de la API. `actualizar.sh` lo lee en cada corrida, entra a GHCR con `--password-stdin` en un `DOCKER_CONFIG` temporal y lo borra al terminar.
- **Índices:** el `Dockerfile` agrega el binario `/indexes` (`scripts/indexes`), que corre en un contenedor de un solo uso. La API sigue sin crear índices. Sin esto, un módulo con índices nuevos daría `/ready` 503 y regresaría siempre.
- **Pasos en el servidor** (`infra/aws-dev/scripts/actualizar.sh`, como root, con `flock` para no solaparse):
  1. Sale sin hacer nada de 21:50 a 22:00.
  2. Baja `dev` y obtiene su digest. Sale si es el activo o si está en `failed_digest`.
  3. **Comprueba que la versión actual dé `/ready` 200; si no, sale y reintenta al minuto siguiente, sin registrar fallo.** Esto evita culpar a una imagen buena por el arranque de la instancia (Neo4j tarda más que `OnBootSec=1min`) o por una caída de Atlas.
  4. Corre `deploy-env.sh` y `/indexes`. Si el binario no existe en la imagen (SHA anterior a nv-08), registra el digest como fallido; otro error de índices se reintenta.
  5. Fija la imagen por digest en `imagen.env`, cambia solo el servicio `api` y espera `/ready` 200 hasta 120 s.
  6. Si no llega: regresa a la imagen anterior. **Registra el digest como fallido solo si la anterior sí queda sana**; si tampoco queda sana, el problema es del entorno y no de la imagen.
  7. Si llega: borra las imágenes sin usar.
- **Registro de fallos:** `/var/log/wherever/failed_digest`, un digest por línea. Para reintentar uno a mano se borra su línea.
- **Timer:** `wherever-actualizar.timer` con `OnBootSec=1min` y `OnUnitActiveSec=1min`; sus registros van al journal (`journalctl -u wherever-actualizar`).
- **Comprobación desde GitHub:** consulta `/health` hasta 5 minutos con la tabla de Contratos. La URL es la variable del repositorio `API_URL`, porque un `destroy` y `apply` cambia la IP elástica.
- **Salida de emergencia:** `desplegar.sh` sigue sirviendo si GitHub o GHCR no responden; apaga el timer y borra `imagen.env` para que no se pisen. Se reactiva con `instalar-actualizador.sh`.

## Contratos
Sin endpoints nuevos ni cambios de contrato: `docs/endpoints.md` no cambia. `GET /health` sigue siendo `{estado, version}`; `version` traerá el SHA completo porque así se compila.

Resultado del workflow según `/health`:

| Lo que ve | Resultado |
|---|---|
| Responde con el SHA nuevo | Verde |
| No hay conexión durante los 5 minutos | Verde con aviso: se actualizará al encender |
| Empezó de 21:50 a 22:00 | Verde con aviso: se actualizará a las 08:00 |
| Responde con otro SHA tras 5 minutos | Rojo: alerta de regreso en el servidor |
| Responde con un código distinto de 200 tras 5 minutos | Rojo |

Terraform: solo `permissions_boundary` en los roles de la instancia y del programador. Sin recursos nuevos.

Parameter Store: nuevo `/wherever/dev/GHCR_TOKEN`. Variable del repositorio: `API_URL`.

Constitución (enmienda): §3 fila CI y §8 pasan a decir "`ci-pr.yml` sin deploy; `cd-dev.yml` publica la imagen y el entorno de desarrollo se actualiza solo (nv-08); producción sin CD". Se corrige también la frase "`main` protegida" (ver Límites).

## Tests
- `go build ./...`, `go vet ./...` y las cinco suites sin cambios (el código de la API no cambia; el `Dockerfile` sí, por `/indexes`).
- `docker build .` produce `/api` y `/indexes`; `/indexes` corre contra el compose local y es idempotente.
- `actionlint` sobre `cd-dev.yml`, `yaml.safe_load` y `bash -n` de los scripts.
- `terraform fmt -check`, `validate` y `trivy config infra/` sin hallazgos altos.
- Pruebas reales:
  - **Camino feliz:** un commit a `main` termina con `/health` en ese SHA y el workflow en verde.
  - **Regreso automático:** una imagen que falla en `/ready` deja la versión anterior sirviendo, el digest en `failed_digest`, sin reintentos, y el workflow en rojo.
  - **Arranque:** tras el encendido de las 08:00 la versión nueva se instala sola, sin registrar fallos por el arranque de Neo4j.
  - **Sin acceso a AWS:** el repositorio no tiene secretos de AWS y el workflow no pide `id-token`.
  - **Token aislado:** `GHCR_TOKEN` no aparece en `/opt/wherever/.env` ni en `docker inspect` de la API.
  - **Filtros:** una corrida de PR y una de otra rama no publican.
  - **Regreso manual:** un SHA con imagen se instala sin reconstruir; uno sin imagen se reconstruye; uno que no es ancestro de `main` se rechaza.
  - **Entrada inválida:** un `sha` con otra forma se rechaza antes de tocar GHCR.
  - **Idempotencia:** volver a ejecutar el mismo SHA termina en verde sin construir.
  - **Ventana:** de 21:50 a 22:00 el servidor no actualiza.
- El reporte final incluye la evidencia de **Sin acceso a AWS** y de **Regreso automático**, que reemplazan a la prueba de Confianza del diseño con OIDC.

## Criterios
- Sin claves de AWS en GitHub; el único token del servidor es de solo lectura y no llega a la API (`critic B1 → PASS`).
- `terraform plan` solo cambia los dos roles; no reemplaza la EC2 ni la IP elástica.
- Del merge a `/health` con el SHA nuevo, sin intervención humana, con la instancia encendida.
- Una versión rota nunca deja el servidor caído ni se reintenta en bucle.
- GHCR dentro del plan gratuito (imagen de unos 45 MB, máximo 5 versiones).

## Límites
- **Riesgo aceptado (revisor, 2026-11-04):** `main` no se puede proteger en este plan de GitHub (la API pide Pro o repositorio público). Quien pueda hacer push a `main` publica en el servidor con solo pasar CI. Aceptable sin datos reales; **no lo sería en producción**.
- **El token caduca:** al vencer el PAT, el servidor deja de actualizarse y el workflow termina en rojo a los 5 minutos con el SHA antiguo. Se renueva a mano. Un PAT clásico `read:packages` alcanza todos los paquetes que su dueño puede leer, no solo este.
- **Falso verde:** sin acceso a AWS, GitHub no distingue una instancia apagada de una caída: ambas dan "sin conexión". Un servidor caído en horario laboral también daría verde con aviso.
- **Sin autocorrección con la versión actual rota:** si la versión activa no da `/ready` 200, el servidor no actualiza (paso 3 de Diseño). Esa situación se resuelve con `desplegar.sh`.
- **Mantenimiento manual:** `actualizar.sh`, las unidades de systemd y `deploy-env.sh` viven en el servidor; cambiarlos en `infra/` no los actualiza. Se suben con `instalar-actualizador.sh`, y requiere SSH desde la IP vigente.
- Los cambios de `infra/` (Terraform, `compose.yml`, `Caddyfile`) siguen siendo manuales; el CD solo actualiza la imagen de la API.
- El regreso automático no deshace índices ya creados ni cambios en datos: los cambios de esquema deben ser compatibles con la versión anterior.
- Solo se regresa a SHA desde la fusión de nv-08: los anteriores no traen `/indexes`.
- Una versión con más de 5 versiones de antigüedad ya no está en GHCR y hay que reconstruirla.
- Solo desarrollo: sin aprobación humana, sin canary ni cero tiempo de caída (la API se reinicia unos segundos).
- El estado de Terraform sigue siendo local y es el único registro de la infraestructura: se copia fuera del repositorio antes de cada `apply`.
- El contenedor `wherever-sin-limites` (excepción temporal de pruebas) se retira antes de firmar.
- `terraform destroy` de nv-07 se prueba al final: cambia la IP elástica, la URL de sslip.io, la lista de Atlas y la variable `API_URL`.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con implementación local, sin desplegar
- **Revisores:** Osmar Reséndiz (pendiente de firmar)
- **Fecha:** —
- **Control de cambios (2026-11-04):** se cancela el diseño con OIDC, ECR y Systems Manager porque una SCP lo impide. El revisor define el diseño por sondeo: GHCR, PAT en Parameter Store, timer de systemd, `failed_digest`, re-etiquetado para el regreso manual, 5 versiones y evaluación de `/health`. Agregados al revisarlo: exclusión del token en `deploy-env.sh`, comprobación previa de `/ready`, regla para registrar fallos solo si la anterior queda sana, `--provenance=false` y aviso en la ventana de apagado.
- **Pendiente para ejecutar:** ver el ciclo real de apagado y encendido con el boundary; el dueño crea el PAT y carga `/wherever/dev/GHCR_TOKEN`.
