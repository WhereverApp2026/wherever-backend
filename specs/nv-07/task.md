# NV-07 — Despliegue de desarrollo en AWS

## T1 — Cuenta y presupuesto
- **Hacer**: confirmar la fecha de creación de la cuenta y el tipo de instancia elegible; usuario IAM o SSO distinto del raíz.
- **Verificar**: `aws ec2 describe-instance-types --filters Name=free-tier-eligible,Values=true`.
- **No hacer**: usar el usuario raíz, crear recursos fuera de Terraform.

## T2 — Infraestructura como código
- **Crear**: archivos de `infra/aws-dev`, variables, salidas, `.gitignore`.
- **Verificar**: `terraform fmt -check`, `terraform validate`, `trivy config`.

## T3 — Aplicar
- **Hacer**: `terraform plan` y `apply`; revisar que el plan coincida con los Criterios.
- **Verificar**: `terraform output`, `/var/log/wherever-bootstrap.log` sin errores.

## T4 — Secretos
- **Hacer**: cargar `JWT_KEY`, `PHONE_HMAC_KEY`, URIs y demás en Parameter Store como SecureString.
- **Verificar**: `deploy-env.sh` escribe el `.env` sin mostrar valores; ningún secreto en Git (`critic B1`).

## T5 — Desplegar la API
- **Crear**: `infra/aws-dev/server/compose.yml` (reinicio automático, memoria, bases sin `ports`) y `scripts/desplegar.sh` (imagen por SSH, sin registro). MongoDB en Atlas con la IP elástica en su Network Access.
- **Verificar**: `/health`, `/ready` y `/api/v1/ping` por HTTPS.

## T6 — Apagado y encendido
- **Hacer**: parar y arrancar la instancia a mano; confirmar IP fija y contenedores que vuelven solos.
- **Verificar**: `verificar.sh` encendida y apagada.

## T7 — Gates y memoria
- **Verificar**: `verificar.sh → 0`, `critic`, `check_sdd`.
- **Actualizar**: `Memoria.md` (fin del plan gratuito, excepción de Parameter Store, deuda de producción) y `AGENTS.md`.
