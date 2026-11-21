# infra/aws-dev — Entorno de desarrollo en AWS (spec nv-07)

Dos EC2 en zonas distintas de `us-east-2`, detrás de un balanceador y de CloudFront, que da el HTTPS (nv-12). MongoDB vive en Atlas, Redis en Upstash y Neo4j en Aura (nv-11).
**MongoDB vive en MongoDB Atlas**, no en la instancia. Apagado diario a las 22:00 y encendido de
lunes a viernes a las 08:00 (`America/Mexico_City`). Sin datos reales, sin CD, desechable.

## Antes de empezar (una sola vez)
1. **Raíz:** desde la consola, activa MFA en el usuario raíz y borra sus claves de acceso si existen. No se usa para nada de esto.
2. **Política** `wherever-dev-deployer-policy` en IAM de la cuenta del proyecto **`816101201001`** ("Ctrl Shift Create"): pega `iam/wherever-dev-deployer-policy.json` (solo `us-east-2`, recursos `wherever-dev-*` y `/wherever/dev/*`, con `ec2:ImportKeyPair` y `ec2:DeleteKeyPair` para el par de llaves; sin permisos de administrador).
3. **Usuario IAM** `wherever-deployer` con MFA y esa política; una clave de acceso solo para él.
4. **Perfil local:** `aws configure --profile wherever-dev` con esa clave y región `us-east-2` (sin `login_session` en `~/.aws/config`). Comprueba que `aws sts get-caller-identity --profile wherever-dev` dé `arn:aws:iam::816101201001:user/wherever-deployer`: ni `:root` ni un usuario con `AdministratorAccess`.
5. **Atlas:** después del `apply`, agrega la IP elástica (`terraform output public_ip`) en *Network Access* de Atlas. El servidor usa **su propio usuario de base de datos** (`wherever-dev-ec2`, `readWrite` solo en `wherever`), distinto del de tu `.env` local: si se filtra uno, el otro sigue en pie.
6. **Respaldo del estado:** `terraform.tfstate` es local y no va a Git; sin él no hay `destroy` limpio. Copia el estado (y `terraform.tfvars`) fuera del repo después de cada `apply`.

## Desplegar
```bash
cd infra/aws-dev
ssh-keygen -t ed25519 -f ~/.ssh/wherever-dev -C wherever-dev   # la privada nunca sale de tu máquina
cp terraform.tfvars.example terraform.tfvars                  # ssh_cidr, ssh_public_key, budget_email
terraform init
terraform plan -out plan.tfplan                               # revisar: 1 instancia, 1 EIP, 1 SG, 2 roles, 1 perfil, 2 schedules, 1 presupuesto, 1 par de llaves
terraform apply plan.tfplan
```

Secretos en Parameter Store (SecureString; nunca en Git ni en Terraform). Ruta `/wherever/dev/`:

| Parámetro | Valor |
|---|---|
| `APP_ENV` | `staging` |
| `MONGO_URI` | cadena de Atlas con la base `wherever` |
| `REDIS_ADDR` | SecureString: URL `rediss://` de Upstash, tal como la da su consola |
| `NEO4J_URI` | SecureString: `neo4j+s://` de Aura con usuario y contraseña antes de la arroba |
| `JWT_KEY`, `PHONE_HMAC_KEY`, `CURSOR_HMAC_KEY` | 32 bytes aleatorios o más, distintos a los de tu `.env` local (los nombres exactos: la API no lee `JWT_SECRET` ni `HMAC_SECRET`) |
| `JWT_ISSUER`, `JWT_AUDIENCE` | `wherever-api`, `wherever-app` |
| `TRUSTED_PROXIES` | `172.16.0.0/12` (el balanceador llega desde la VPC; solo de ahí se cree `X-Forwarded-For`) |
| `TRUSTED_PROXY_HOPS` | `2`: CloudFront y el balanceador; sin este valor, un cliente comparte el límite con los demás (nv-12) |
| `REQUEST_TIMEOUT` | `55s`: menos que los 60 s de CloudFront, para que el cliente reciba el error de la API y no un 504 (nv-12) |
| `JOB_HOUR` | `21`: la instancia se apaga a las 22:00; a las 03:00 el trabajo de privacidad nunca correría (nv-15) |
| `HIDE_INTERNALS` | `true`: servidor público, sin `detalle` ni sondas (nv-15) |
| `CORS_ALLOWED_ORIGIN`, `SMTP_*`, `MAIL_FROM`, `APP_LINK_BASE` | opcionales |
| `GOOGLE_CLIENT_ID` | String, opcional: client ID del cliente OAuth «Aplicación web» del proyecto de Firebase. Sin él la API arranca y `POST /auth/google` responde 503; la API lo lee al reiniciar (nv-72) |
| `APP_VERSION_MINIMA`, `APP_VERSION_ACTUAL` | String, semver `X.Y.Z`; la mínima en `0.0.0` no bloquea a nadie y nunca es mayor que la actual (nv-16) |
| `TERMINOS_VERSION`, `AVISO_VERSION` | String; `2026-10` y `2026-11` hoy (nv-16) |
| `MODO_MANTENIMIENTO` | String, `false`; ver [Mantenimiento](#mantenimiento-spec-nv-16) |

En Git Bash (Windows) antepón `MSYS_NO_PATHCONV=1`; si no, la ruta `/wherever/dev/...` se convierte en una ruta de Windows.

```bash
MSYS_NO_PATHCONV=1 aws ssm put-parameter --profile wherever-dev --region us-east-2 --name /wherever/dev/MONGO_URI --type SecureString --value '<cadena>'
```

La app la instalan `crear.sh` y el actualizador; ver [Crear y destruir](#crear-y-destruir-spec-nv-14).

## Actualización automática (spec nv-08)
Cada `push` a `main` con CI en verde publica la imagen en GHCR (`cd-dev.yml`); el servidor la detecta, verifica su firma y se instala sola, con regreso automático.

Una sola vez:
1. Crea un PAT clásico con solo `read:packages` y cárgalo tú, sin pegarlo en ningún chat:
```bash
MSYS_NO_PATHCONV=1 aws ssm put-parameter --profile wherever-dev --region us-east-2 --name /wherever/dev/GHCR_TOKEN --type SecureString --value '<token>'
```
2. Define la variable del repositorio `API_URL` con la URL del servidor.
3. Activa el timer con `crear.sh`, que corre `instalar-actualizador.sh` en cada instancia. Repítelo si cambian esos archivos.

- **Ver qué pasó:** `journalctl -u wherever-actualizar -n 50`.
- **Firma rechazada:** el digest queda en `failed_digest` y CloudWatch registra el evento `firma_rechazada`. Si el log dice «no se pudo comprobar la firma» (red, Rekor o `GHCR_TOKEN`), no se marca: reintenta cada minuto sin instalar.
- **Regresar:** en Actions, `cd-dev` → Run workflow con el SHA completo; el servidor la instala solo.
- **Reintentar una versión fallida:** borra su línea de `/var/log/wherever/failed_digest`.
- **Si el workflow termina en rojo con un SHA antiguo:** el servidor regresó, rechazó la firma, o venció `GHCR_TOKEN`.

### Firma, SBOM y provenance (PR D, 2026-11-19)
- **Qué se publica:** `docker buildx build --provenance=mode=max --sbom=true`. La etiqueta del SHA apunta a un **índice OCI** con la imagen `linux/amd64` y su manifiesto de atestaciones (SBOM SPDX y provenance SLSA). `dev` apunta al **mismo** digest. Docker en la instancia elige la plataforma del índice; `RepoDigests` guarda el digest del índice, que es el que se firma y se verifica.
- **Firma:** job `firmar`, con cosign `v3.1.4` sin llaves: el OIDC de GitHub (`id-token: write`, solo ahí) da un certificado de Fulcio con la identidad del workflow y la firma queda en Rekor. Siempre por digest. `dev` se mueve después de firmar, así un servidor nunca ve un `dev` sin firma.
- **Verificación en el servidor:** antes de los índices y de salir del balanceador, `actualizar.sh` corre `cosign verify` con `--certificate-identity-regexp '^https://github\.com/WhereverApp2026/wherever-backend/\.github/workflows/cd-dev\.yml@refs/heads/main$'` y `--certificate-oidc-issuer https://token.actions.githubusercontent.com`. Usa las mismas credenciales de GHCR. cosign lo instala `instalar-actualizador.sh` (binario oficial, checksum fijado en el script, `/usr/local/bin/cosign`, caché de TUF en `/var/lib/wherever/sigstore`).
- **Imágenes viejas sin firma (regreso):** solo se aceptan digests firmados por `cd-dev` desde `main`, sin excepción por fecha. Nada se firma a posteriori: un `Run workflow` con el SHA de una imagen anterior al cambio, o de una cuya firma quedó a medias, la **reconstruye** desde ese SHA (si es ancestro de `main`) con SBOM, provenance y firma nuevas; el digest cambia. Una imagen ya firmada no se reconstruye. El regreso automático de `actualizar.sh` a la versión que ya corría no se vuelve a verificar: ya estaba instalada.
- **Retención:** cada despliegue deja de 4 a 5 versiones en GHCR (índice, imagen, atestación, firma y, si GHCR no tiene la API de referrers, su índice). Se guardan 40 (al menos 8 despliegues). Al más viejo puede faltarle una pieza; un regreso a ese SHA lo detecta y lo reconstruye.
- **Comprobar a mano** (con un PAT de `read:packages`, en Docker; en Git Bash antepón `MSYS_NO_PATHCONV=1`):
```bash
digest="$(docker buildx imagetools inspect ghcr.io/whereverapp2026/wherever-backend:dev --format '{{json .Manifest}}' | jq -r .digest)"
cosign verify --certificate-identity-regexp '^https://github\.com/WhereverApp2026/wherever-backend/\.github/workflows/cd-dev\.yml@refs/heads/main$' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com "ghcr.io/whereverapp2026/wherever-backend@$digest"
docker buildx imagetools inspect "ghcr.io/whereverapp2026/wherever-backend@$digest" --format '{{json .SBOM}}' | head -c 300
```
- **Subir cosign:** cambia juntas `cosign_version` y los dos checksums de `instalar-actualizador.sh` (de `cosign_checksums.txt` de esa versión) y `COSIGN_RELEASE` de `cd-dev.yml`; corre `instalar-actualizador.sh` en las dos instancias antes de fusionar.

## Verificar
```bash
$(terraform -chdir=infra/aws-dev output -raw verificar)   # solo lectura; falla si algo sale del Free Tier o queda expuesto
curl -i "$(terraform -chdir=infra/aws-dev output -raw cloudfront_url)/ready"
```
`/ready` da 503 si Atlas no acepta la IP elástica o falta un índice (`make indexes` contra Atlas).

## CI
`ci-pr.yml` construye la imagen con el `Dockerfile`; su job `iac-scan` corre `trivy config` sobre `infra/aws-dev` y `scripts/probar-respaldo.sh`. `fmt` y `validate` de Terraform se corren a mano.

## Servicios externos (spec nv-11)
- **Neo4j en pausa.** Aura Free se pausa tras 72 horas sin uso y **borra la instancia tras 30 días en pausa**. En pausa, `/ready` da 503 y el actualizador no instala imágenes. Para reanudar: consola de Aura → instancia → Resume. Compruébalo antes de la entrega y de la evaluación.
- **Cuota de Redis.** Upstash Free da 500 000 comandos al mes. Mira el consumo en su consola. Costos por operación: `scripts/medir-redis.sh`. Sin cuota, `/ready` da 200 `degraded` y lo autenticado da 503.
- **Plan B de Redis.** Regresa el servicio `redis` a `server/compose.yml` (historial de Git), apunta `REDIS_ADDR` a `redis:6379` y corre `instalar-actualizador.sh`.
- **Rotar credenciales** (control 14): `scripts/rotar-secretos.sh`. Nunca muestra un valor y reinicia la API en las dos instancias, una a la vez, esperando `/ready`.
  - `estado`: días desde el último cambio de cada parámetro.
  - `propios --aplicar`: rota `JWT_KEY` y `CURSOR_HMAC_KEY` y pone una llave nueva al frente de `DATA_KEYS` (las viejas se quedan para descifrar; luego `make migrar-cifrado` con `-aplicar`). Los access vigentes dan 401 y los cursores en uso 422: la app refresca y vuelve a pedir. Cada 90 días.
  - `cargar NOMBRE --aplicar`: después de rotar en su consola una llave de terceros (Stripe, Gemini, SMTP, Atlas, Firebase…), la pide sin mostrarla y la carga. Cada 180 días.
  - Cada parámetro queda **avanzado** (US$0,05 al mes) con `NoChangeNotification`: al vencer, EventBridge avisa al correo de alarmas (`rotacion.tf`).
  - `PHONE_HMAC_KEY` no se rota: los teléfonos solo existen como su HMAC.
  - **Aplicar:** un administrador crea `wherever-dev-deployer-rotacion` (`iam/wherever-dev-deployer-rotacion.json`) adjunta a `wherever-deployer`; luego `crear.sh` (3 recursos nuevos).
- **Diagnóstico.** Con una clave de Redis mala, la API registra `i/o timeout`, no `WRONGPASS`. Compruébala con `redis-cli --tls -u` desde un contenedor. Copia la cadena de Upstash en Connect → TCP; no la armes a mano.
- **Esquema y datos de demostración.** `make siembra`, con `NEO4J_URI` en el entorno. Es idempotente e imprime el tamaño del grafo.

## Mantenimiento (spec nv-16)
Con `MODO_MANTENIMIENTO=true`, `/api/v1/configuracion` anuncia `mantenimiento: true` y toda ruta bajo `/api/v1/` que no sea ping, configuración ni auth responde 503 con `Retry-After: 60`. La API lo lee al arrancar y el actualizador no reinicia por un cambio de parámetro, así que se aplica a mano:

1. `MSYS_NO_PATHCONV=1 aws ssm put-parameter --profile wherever-dev --region us-east-2 --name /wherever/dev/MODO_MANTENIMIENTO --type String --value true --overwrite`.
2. Por Systems Manager, en una instancia y luego en la otra, en `/opt/wherever`: `./deploy-env.sh /wherever/dev/` y `docker compose --env-file .env --env-file imagen.env up -d --no-deps api`. Mientras tanto conviven las dos respuestas.
3. Comprobar por CloudFront: una ruta de negocio da 503 y `/api/v1/configuracion` da 200 con `mantenimiento: true`.
4. Para apagarlo, los mismos pasos con `false`. Igual para subir `APP_VERSION_MINIMA`.

## Observabilidad (spec nv-13)
- **Logs:** CloudWatch, grupo `/wherever/dev/api`, 14 días. Un flujo `api-<instancia>` y otro `despliegues-<instancia>`. También siguen en `docker logs`. Detalle en `docs/observabilidad.md`.
- **Alarmas:** llegan al correo de `budget_email`; AWS manda un correo que hay que confirmar una vez.
- **Instalar o actualizar archivos del servidor:** `infra/aws-dev/scripts/instalar-actualizador.sh --instance <id>`, por Systems Manager.

## Moderación de imágenes (spec nv-42)
- **Permiso:** `rekognition:DetectModerationLabels` solo en `us-east-2`, en tres lugares: el boundary (`iam/wherever-dev-boundary.json`), el rol de la instancia (`aws_iam_role_policy.moderacion` en `main.tf`) y el deployer (`MedirModeracionEnVivo`, solo para la prueba en vivo).
- **Aplicar:** con un usuario administrador, en IAM → *Policies*, crea una versión nueva de `wherever-dev-boundary` y de `wherever-dev-deployer-policy` con los JSON del repo (el deployer no puede cambiar el boundary). Después `terraform apply` con el perfil `wherever-dev`; agrega una política en línea al rol y no recrea las instancias.
- **Sin variables nuevas:** la región sale del entorno y las credenciales del rol de la instancia.
- **Costo:** US$0.001 por imagen; ver `specs/nv-42/spec.md`.

## Imágenes: S3 y CloudFront (spec nv-44)
- **Recursos** (`imagenes.tf`): bucket `wherever-dev-imagenes-<cuenta>` con el acceso público bloqueado, sin ACL, cifrado SSE-S3 y sin listar; segunda distribución de CloudFront con control de acceso de origen y caché de un año. CloudFront solo lee `i/`; `cuarentena/` nunca sale y S3 la vacía a las 24 h (hasta unas 48 h por su ciclo diario).
- **Rol de la instancia** (`aws_iam_role_policy.imagenes`): `s3:PutObject`, `GetObject` y `DeleteObject` solo en `cuarentena/*` e `i/*`; el boundary repite ese techo (`ImagenesDelProyecto`).
- **Aplicar** (antes de nv-45 en AWS): un administrador crea una versión nueva de `wherever-dev-boundary` con el JSON del repo y crea la política `wherever-dev-deployer-imagenes` (`iam/wherever-dev-deployer-imagenes.json`) adjunta a `wherever-deployer` (la del deployer ya no admite más caracteres). Espera a que IAM propague y corre `crear.sh`; agrega recursos y no recrea las instancias.
- **Variables para nv-45** en `/wherever/dev/`: `IMAGENES_BUCKET` (salida `imagenes_bucket`) e `IMAGENES_URL` (salida `imagenes_url`). Las define nv-45 como opcionales fuera de prod; esta spec no cambia la API.
- **`destruir.sh`** vacía y borra el bucket (`force_destroy`): las imágenes se pierden y la URL de imágenes cambia al recrear.
- **Costo:** dentro de la capa gratuita de CloudFront (1 TB y 10 millones de peticiones al mes); S3 unos centavos al mes para el piloto.

## Respaldos de MongoDB
Atlas M0 no tiene respaldos. Cada viernes a las 21:00 (`America/Mexico_City`) la instancia **a** corre `scripts/respaldo-mongo.sh` con `wherever-respaldo.timer`: `mongodump --archive --gzip` de la base de `MONGO_URI` en un contenedor `mongo` fijado por digest, sube `respaldos/AAAA-MM-DD.archive.gz` a S3, borra el archivo local y registra en CloudWatch el evento `respaldo` (`estado` ok o error, tamaño y paso; sin datos). Si ese viernes estuvo apagada, corre al encender (`Persistent=true`).

- **Bucket** (`respaldos.tf`): `wherever-dev-respaldos-<cuenta>`, privado, con el acceso público bloqueado, sin ACL, SSE-S3, versionado y política que niega todo lo que no vaya por TLS. Los respaldos y sus versiones reemplazadas se borran a los 28 días: quedan cuatro.
- **Permisos:** la instancia solo tiene `s3:PutObject` en `respaldos/*` (rol y boundary, `RespaldarMongo`): no puede leer, listar ni borrar. Si alguien sube basura con el nombre de un respaldo, el versionado guarda el anterior. Leer y restaurar se hace con el perfil `wherever-dev` (`BucketDeRespaldos` y `VaciarRespaldosAlDestruir` en `iam/wherever-dev-deployer-imagenes.json`).
- **Secreto:** la cadena de Atlas sale de `.env` y entra al contenedor por la entrada estándar; nunca va en la línea de comandos ni en el log. Los errores de mongodump se imprimen sin credenciales.
- **Alarma** `wherever-dev-respaldos`: sin un respaldo `ok` en 7 días (el máximo de CloudWatch para una alarma) llega el mismo correo que las demás. Aquí la falta de datos cuenta como falla. Recién creada queda en ALARM hasta el primer respaldo.
- **Ver qué pasó:** `journalctl -u wherever-respaldo -n 50` en la instancia a, o el flujo `respaldos-<instancia>` del grupo de logs. Un fallo dice `respaldo: FALLÓ en el paso '<paso>'`.
- **Respaldo a mano:** por Systems Manager en la instancia a, `systemctl start wherever-respaldo.service` (espera a que termine). En la b no hace nada.
- **Prueba:** `infra/aws-dev/scripts/probar-respaldo.sh` (job `iac-scan`): shellcheck, respaldo en seco contra un MongoDB en Docker, restauración y conteo, y una cadena inalcanzable que debe terminar en error. En Windows córrelo dentro de un contenedor Linux con el socket de Docker (ver `Memoria.md`).
- **Costo:** centavos. Un respaldo del piloto pesa pocos MB; cuatro en S3 Standard cuestan menos de un centavo de dólar al mes. Una métrica y una alarma más, dentro de las 10 gratuitas de cada una. Bajar la imagen `mongo` cada semana (el actualizador borra las imágenes sin usar) no cobra: el tráfico de entrada es gratis.
- **`destruir.sh`** borra el bucket y sus respaldos (`force_destroy`): baja el último antes.
- **Subir la versión de `mongo`:** cambia etiqueta y digest juntos en `respaldo-mongo.sh` (`docker buildx imagetools inspect mongo:8.0`) y corre `instalar-actualizador.sh` en la instancia a.

### Restaurar
Primero a un MongoDB local de prueba; a Atlas solo después de revisar que el respaldo está completo. En Git Bash antepón `MSYS_NO_PATHCONV=1` a los comandos de `docker` y `aws`.

```bash
cd infra/aws-dev
bucket="$(terraform output -raw respaldos_bucket)"
aws s3 ls "s3://$bucket/respaldos/" --profile wherever-dev                       # elige la fecha
aws s3 cp "s3://$bucket/respaldos/2026-11-20.archive.gz" respaldo.archive.gz --profile wherever-dev
# ¿Lo sobrescribió alguien? Versiones anteriores del mismo día:
aws s3api list-object-versions --bucket "$bucket" --prefix respaldos/2026-11-20 --profile wherever-dev
#   aws s3api get-object --bucket "$bucket" --key respaldos/2026-11-20.archive.gz --version-id <id> respaldo.archive.gz --profile wherever-dev

# 1. Verificar en un MongoDB local desechable (misma imagen que el respaldo):
imagen=mongo:8.0.38@sha256:bdc216e9d5a6ec2cfdac789b5832fdab64cd32055f59885cae918a92de63c361
docker network create restauracion
docker run -d --name mongo-restauracion --network restauracion "$imagen"
sleep 5
docker run -i --rm --network restauracion "$imagen" mongorestore --uri mongodb://mongo-restauracion:27017 --archive --gzip < respaldo.archive.gz
docker exec mongo-restauracion mongosh wherever --quiet --eval 'db.getCollectionNames().sort().forEach(c => print(c, db[c].countDocuments()))'
docker rm -f mongo-restauracion && docker network rm restauracion
```

2. **A Atlas**, solo si las colecciones y los conteos tienen sentido. Pon la API en mantenimiento ([Mantenimiento](#mantenimiento-spec-nv-16)) y usa tu usuario de Atlas (no el del servidor). La cadena va en un archivo fuera del repo, no en la línea de comandos: crea `~/restaurar.yaml` con una sola línea `uri: '<cadena con la base wherever>'`, y luego:

```bash
docker run -i --rm -v "$HOME/restaurar.yaml:/c.yaml:ro" "$imagen" mongorestore --config=/c.yaml   --archive --gzip --drop --nsInclude 'wherever.*' < respaldo.archive.gz
rm ~/restaurar.yaml respaldo.archive.gz
```

`--drop` reemplaza cada colección del respaldo; lo escrito después del respaldo se pierde. Después corre `make indexes` contra Atlas y quita el mantenimiento. Los datos de Neo4j y Redis no están en este respaldo: el grafo se rehace con `scripts/reconstruir-grafo`.

## Correo saliente (SMTP)
- **Puerto:** `egress_ports.smtp` abre el 587 (STARTTLS). Si el proveedor cambia a 465, cambia el puerto y `SMTP_TLS=tls`.
- **Proveedor:** Brevo (`SMTP_HOST=smtp-relay.brevo.com`, `SMTP_PORT=587`, `SMTP_TLS=starttls`); el plan gratuito da unos 300 correos al día.
- **Credenciales:** `SMTP_USER` es el login SMTP que muestra Brevo (termina en `@smtp-brevo.com`, no es el correo de la cuenta) y `SMTP_PASSWORD` es la clave SMTP de Brevo (SecureString), no la contraseña de la cuenta.
- **Remitente:** `MAIL_FROM` es el correo de un remitente verificado en Brevo, como correo simple (`remitente@dominio`): con `Nombre <correo>` la API no arranca.
- **Por qué no Gmail:** al principio se usó el SMTP de Gmail con contraseña de aplicación, pero Google bloqueaba los envíos desde AWS (cuenta nueva sin verificar). Con un remitente de correo gratuito (Gmail, Outlook) Brevo lo marca como no conforme y los correos pueden llegar a spam; se resuelve al tener dominio propio con SPF y DKIM.
- **Variables** en `/wherever/dev/`: `SMTP_HOST`, `SMTP_PORT`, `SMTP_TLS`, `SMTP_USER`, `SMTP_PASSWORD` (SecureString), `MAIL_FROM` y `APP_LINK_BASE`.
- **Aplicar:** `crear.sh` (agrega una regla de salida al Security Group, sin recrear nada) y luego reiniciar la API para que relea Parameter Store.

## Memoria de la instancia
`t3.micro` (1 GB): API 192 MB y 2 GB de swap.

## Dos instancias (spec nv-12)
- **Costo:** unos US$41 al mes; el balanceador cobra aunque las instancias estén apagadas. Se destruye después de la evaluación con `destruir.sh`: sin créditos se pierde el acceso a la cuenta.
- **Acceso:** el 8080 solo admite al balanceador, y el balanceador solo a CloudFront. El 22 está cerrado; se entra por Systems Manager.
- **Actualización:** cada instancia sale del balanceador, instala y vuelve a entrar; nunca sale si es la única sana. B espera al menos 5 minutos y a que A sirva el SHA nuevo. Si A regresó, B no instala.
- **Caída abrupta:** hasta unos 25 s de errores 502 intermitentes, hasta que el balanceador marca la instancia como no sana.

## WAF y origen (plan de seguridad, PR E)
- **Qué hace** (`waf.tf`): web ACL `wherever-dev-api` en `us-east-1` (CloudFront lo exige ahí) sobre la distribución de la API, con cuatro reglas en orden: reputación de IP de AWS, límite por IP (`waf_rate_limit`, 5000 en 5 minutos, responde 429 con el cuerpo uniforme), entradas maliciosas conocidas y OWASP general. OWASP no mira subidas `multipart/form-data` (falsos positivos con fotos y PDF) y `SizeRestrictions_BODY` solo cuenta (la API acepta hasta 10 MB). La distribución de imágenes no lleva WAF: solo sirve archivos.
- **Origen:** CloudFront agrega la cabecera `X-Origen-Wherever` con un secreto (`random_password.origen`, en el estado, fuera de Git) y el balanceador solo reenvía si viene; sin ella responde 403. Así nadie se salta el WAF con otra distribución que apunte al balanceador. Rotar: `terraform apply -replace=random_password.origen` (unos segundos de 403).
- **Costo:** ≈US$9–10 al mes (US$5 el web ACL, US$1 por regla, US$0.60 por millón de peticiones), también con las instancias apagadas.
- **Aplicar:** un administrador crea la política `wherever-dev-deployer-waf` con `iam/wherever-dev-deployer-waf.json` y la adjunta a `wherever-deployer`; luego `crear.sh` (3 recursos nuevos y 2 cambios; nada se recrea). Al cambiar el balanceador quedan unos segundos de 403 mientras se crea la regla.
- **Revisar:** consola de WAF (`us-east-1`) → *Web ACLs* → `wherever-dev-api` → *Traffic overview* y *Sampled requests* (últimas 3 h). Un 403 legítimo se ve ahí con el nombre de la regla; se corrige con `rule_action_override` a `count` en esa regla, no apagando el grupo. Sin registro completo de peticiones (cuesta por GB).

## Crear y destruir (spec nv-14)
Son el paso configurado de la infraestructura. Corren en tu máquina, con `terraform.tfvars` y el perfil; GitHub no tiene claves de AWS. Muestran el plan y piden confirmar por escrito. El estado de Terraform es local y cada script lo respalda en `~/wherever-dev-terraform-respaldo`.

```bash
infra/aws-dev/scripts/crear.sh
```

```bash
infra/aws-dev/scripts/destruir.sh
```

Con `--solo-plan`, no cambian nada.

**`crear.sh`** aplica Terraform y, en cada instancia, espera a Systems Manager, sube los archivos, arma `.env` desde Parameter Store y activa los timers. El actualizador instala la imagen de `dev` por su cuenta.

Pasos manuales tras recrear; `crear.sh` también los imprime:
1. **Atlas → Network Access:** permite las IP elásticas nuevas. Sin esto la API no se instala, porque `/indexes` falla y el actualizador reintenta cada minuto.
2. **SNS:** confirma el correo *Subscription Confirmation*; se pierde al destruir.
3. **URL:** cambia al recrear, porque la distribución de CloudFront es otra. Actualiza `API_URL` con `gh variable set` y avisa al equipo móvil.

**`destruir.sh`** no toca Parameter Store, Atlas, Aura, Upstash, GHCR, el boundary ni las políticas. Se pierden los logs de CloudWatch, las imágenes del bucket y los respaldos de MongoDB.

Antes de las 21:50 o después de las 08:00: fuera de horario, la instancia nueva se apaga o el actualizador no instala.
La IP elástica se libera con el destroy; una suelta cobra. Los parámetros de `/wherever/dev/` no los crea Terraform: bórralos con `aws ssm delete-parameter` si ya no sirven.
