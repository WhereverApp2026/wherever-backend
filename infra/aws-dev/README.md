# infra/aws-dev — Entorno de desarrollo en AWS (spec nv-07)

Una EC2 del Free Tier en `us-east-2` con la API, Redis, Neo4j y Caddy (HTTPS con Let's Encrypt).
**MongoDB vive en MongoDB Atlas**, no en la instancia. Apagado diario a las 22:00 y encendido de
lunes a viernes a las 08:00 (`America/Mexico_City`). Sin datos reales, sin CD, desechable.

## Antes de empezar (una sola vez)
1. **Raíz:** desde la consola, activa MFA en el usuario raíz y borra sus claves de acceso si existen. No se usa para nada de esto.
2. **Política** `wherever-dev-deployer-policy` en IAM de la cuenta del proyecto **`816101201001`** ("Ctrl Shift Create"): pega `iam/wherever-dev-deployer-policy.json` (solo `us-east-2`, recursos `wherever-dev-*` y `/wherever/dev/*`, con `ec2:ImportKeyPair` y `ec2:DeleteKeyPair` para el par de llaves; sin permisos de administrador).
3. **Usuario IAM** `wherever-deployer` con MFA y esa política; una clave de acceso solo para él.
4. **Perfil local:** `aws configure --profile wherever-dev` con esa clave y región `us-east-2` (sin `login_session` en `~/.aws/config`). Comprueba que `aws sts get-caller-identity --profile wherever-dev` dé `arn:aws:iam::816101201001:user/wherever-deployer`: ni `:root` ni un usuario con `AdministratorAccess`.
5. **Atlas:** después del `apply`, agrega la IP elástica (`terraform output public_ip`) en *Network Access* de Atlas. Mejor aún, usa un usuario de base de datos con `readWrite` solo en `wherever` (no Atlas Admin).

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
| `JWT_KEY`, `PHONE_HMAC_KEY` | 32 bytes aleatorios o más, distintos a los de tu `.env` local |
| `JWT_ISSUER`, `JWT_AUDIENCE` | `wherever-api`, `wherever-app` |
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

## Verificar
```bash
$(terraform -chdir=infra/aws-dev output -raw verificar)   # solo lectura; falla si algo sale del Free Tier o queda expuesto
curl -i "$(terraform -chdir=infra/aws-dev output -raw url_https)/ready"
```
`/ready` da 503 si Atlas no acepta la IP elástica o falta un índice (`make indexes` contra Atlas).

## Pendiente: correo
La salida solo abre 443, 80 y 27017. Cuando se elija proveedor SMTP, agrega su puerto (587 o 465) a `egress_ports` en `main.tf` y vuelve a aplicar.

## Memoria de la instancia
`t3.micro` (1 GB): API 192 MB, Redis 96 MB (`maxmemory 64mb`), Neo4j 512 MB (heap 256 MB), Caddy 64 MB y 2 GB de swap. Sin MongoDB local, cabe con margen.

## Destruir
```bash
terraform -chdir=infra/aws-dev destroy
```
La IP elástica se libera con el destroy; una suelta cobra. Los parámetros de `/wherever/dev/` no los crea Terraform: bórralos con `aws ssm delete-parameter` si ya no sirven.
