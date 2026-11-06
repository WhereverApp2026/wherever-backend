# infra/aws-dev — Entorno de desarrollo en AWS (spec nv-07)

Una EC2 del Free Tier en `us-east-2` con la API y Caddy (HTTPS con Let's Encrypt). MongoDB vive en Atlas, Redis en Upstash y Neo4j en Aura (nv-11).
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
| `JWT_KEY`, `PHONE_HMAC_KEY` | 32 bytes aleatorios o más, distintos a los de tu `.env` local (los nombres exactos: la API no lee `JWT_SECRET` ni `HMAC_SECRET`) |
| `JWT_ISSUER`, `JWT_AUDIENCE` | `wherever-api`, `wherever-app` |
| `TRUSTED_PROXIES` | `172.16.0.0/12` (Caddy llega por la red de Docker; solo de ahí se cree `X-Forwarded-For`) |
| `JOB_HOUR` | `21`: la instancia se apaga a las 22:00; a las 03:00 el trabajo de privacidad nunca correría (nv-15) |
| `HIDE_INTERNALS` | `true`: servidor público, sin `detalle` ni sondas (nv-15) |
| `CORS_ALLOWED_ORIGIN`, `SMTP_*`, `MAIL_FROM`, `APP_LINK_BASE` | opcionales |

En Git Bash (Windows) antepón `MSYS_NO_PATHCONV=1`; si no, la ruta `/wherever/dev/...` se convierte en una ruta de Windows.

```bash
MSYS_NO_PATHCONV=1 aws ssm put-parameter --profile wherever-dev --region us-east-2 --name /wherever/dev/MONGO_URI --type SecureString --value '<cadena>'
```

La app (la imagen se construye aquí porque el repo es privado y se copia por SSH):
```bash
infra/aws-dev/scripts/desplegar.sh --ip "$(terraform -chdir=infra/aws-dev output -raw public_ip)" \
  --site "$(terraform -chdir=infra/aws-dev output -raw site_address)" --key ~/.ssh/wherever-dev
```

Si `docker build` se cuelga en `go mod download` (red inestable hacia `proxy.golang.org`), excepción de desarrollo: compila en local y empaqueta sobre la misma base, luego corre el resto de `desplegar.sh` a mano (copiar imagen, `deploy-env.sh`, `compose up`). No es reproducible: anota el commit en `Memoria.md`.
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$(git rev-parse --short HEAD)" -o <dir>/api ./cmd/api
# <dir>/Dockerfile: FROM gcr.io/distroless/static-debian12:nonroot / COPY api /api / USER nonroot:nonroot / ENTRYPOINT ["/api"]
docker build -t wherever-api:dev <dir>
```

## Actualización automática (spec nv-08)
Cada `push` a `main` con CI en verde publica la imagen en GHCR (`cd-dev.yml`); el servidor la detecta y se instala sola, con regreso automático.

Una sola vez:
1. Crea un PAT clásico con solo `read:packages` y cárgalo tú, sin pegarlo en ningún chat:
```bash
MSYS_NO_PATHCONV=1 aws ssm put-parameter --profile wherever-dev --region us-east-2 --name /wherever/dev/GHCR_TOKEN --type SecureString --value '<token>'
```
2. Define la variable del repositorio `API_URL` con la URL del servidor.
3. Activa el timer (repítelo si cambian estos archivos; requiere que `ssh_cidr` tenga tu IP vigente):
```bash
infra/aws-dev/scripts/instalar-actualizador.sh --ip "$(terraform -chdir=infra/aws-dev output -raw public_ip)" --key ~/.ssh/wherever-dev
```

- **Ver qué pasó:** `journalctl -u wherever-actualizar -n 50`.
- **Regresar:** en Actions, `cd-dev` → Run workflow con el SHA completo; el servidor la instala solo.
- **Reintentar una versión fallida:** borra su línea de `/var/log/wherever/failed_digest`.
- **Si el workflow termina en rojo con un SHA antiguo:** el servidor regresó, o venció `GHCR_TOKEN`.
- **Emergencia:** `desplegar.sh` apaga el timer; se reactiva con `instalar-actualizador.sh`.

## Verificar
```bash
$(terraform -chdir=infra/aws-dev output -raw verificar)   # solo lectura; falla si algo sale del Free Tier o queda expuesto
curl -i "$(terraform -chdir=infra/aws-dev output -raw url_https)/ready"
```
`/ready` da 503 si Atlas no acepta la IP elástica o falta un índice (`make indexes` contra Atlas).

## CI
`ci-pr.yml` corre en todo PR (sin filtro de rutas) y construye la imagen con el `Dockerfile`, pero no revisa Terraform: `fmt`, `validate` y Trivy de `infra/` se corren a mano.

## Servicios externos (spec nv-11)
- **Neo4j en pausa.** Aura Free se pausa tras 72 horas sin uso y **borra la instancia tras 30 días en pausa**. En pausa, `/ready` da 503 y el actualizador no instala imágenes. Para reanudar: consola de Aura → instancia → Resume. Compruébalo antes de la entrega y de la evaluación.
- **Cuota de Redis.** Upstash Free da 500 000 comandos al mes. Mira el consumo en su consola. Costos por operación: `scripts/medir-redis.sh`. Sin cuota, `/ready` da 200 `degraded` y lo autenticado da 503.
- **Plan B de Redis.** Regresa el servicio `redis` a `server/compose.yml` (historial de Git), apunta `REDIS_ADDR` a `redis:6379` y corre `instalar-actualizador.sh`.
- **Rotar credenciales.** Cambia la clave en la consola del servicio y carga el parámetro con `--overwrite`. La API la toma al instalar la siguiente imagen; para aplicarla antes, entra por SSH y corre en `/opt/wherever` `sudo ./deploy-env.sh /wherever/dev/ "$(grep -m1 '^SITE_ADDRESS=' .env | cut -d= -f2-)"`, y luego `docker compose --env-file .env --env-file imagen.env up -d --no-deps api`.
- **Diagnóstico.** Con una clave de Redis mala, la API registra `i/o timeout`, no `WRONGPASS`. Compruébala con `redis-cli --tls -u` desde un contenedor. Copia la cadena de Upstash en Connect → TCP; no la armes a mano.
- **Esquema y datos de demostración.** `make siembra`, con `NEO4J_URI` en el entorno. Es idempotente e imprime el tamaño del grafo.

## Observabilidad (spec nv-13)
- **Logs:** CloudWatch, grupo `/wherever/dev/api`, 14 días. Un flujo `api-<instancia>` y otro `despliegues-<instancia>`. También siguen en `docker logs`. Detalle en `docs/observabilidad.md`.
- **Alarmas:** llegan al correo de `budget_email`; AWS manda un correo que hay que confirmar una vez.
- **Instalar o actualizar archivos del servidor:** `infra/aws-dev/scripts/instalar-actualizador.sh --instance <id>`, por Systems Manager.

## Pendiente: correo
La salida solo abre 443, 80 y 27017. Cuando se elija proveedor SMTP, agrega su puerto (587 o 465) a `egress_ports` en `main.tf` y vuelve a aplicar.

## Memoria de la instancia
`t3.micro` (1 GB): API 192 MB, Caddy 64 MB y 2 GB de swap. Cada contenedor tiene tope propio: si Neo4j se pasa, el OOM mata solo ese contenedor y `unless-stopped` lo levanta. Medido en reposo (2026-11-03): Neo4j 360 MB, API 22 MB, Caddy 21 MB, Redis 3 MB; ~210 MB disponibles y ~200 MB de swap en uso. Si falta, `instance_type = "t3.small"` (elegible en esta cuenta, plan gratuito nuevo).

## Destruir
```bash
terraform -chdir=infra/aws-dev destroy
```
Un `destroy` + `apply` **cambia la IP elástica**: cambia la URL de sslip.io, hay que actualizar *Network Access* de Atlas (y quitar la IP vieja) y cualquier cliente que apunte a la URL anterior. Hazlo cuando nada dependa de esa URL.
La IP elástica se libera con el destroy; una suelta cobra. Los parámetros de `/wherever/dev/` no los crea Terraform: bórralos con `aws ssm delete-parameter` si ya no sirven.
