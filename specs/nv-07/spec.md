# NV-07 — Despliegue de desarrollo en AWS Free Tier

## Contexto
Primer despliegue del backend. La constitución dejó AWS y Terraform fuera de `nv-00…nv-03`; esta spec los abre solo para un **entorno de desarrollo de costo cero**, sin CD, sin alta disponibilidad y sin datos reales. La app móvil solo acepta HTTPS (constitución móvil §6), así que el servidor necesita un certificado válido desde el primer día. **Depende de**: NV-01 a NV-05 (API, `/health`, `/ready`, bases de datos).

## Requisitos
1. Una sola instancia EC2 Linux de un tipo elegible para el Free Tier, dentro de las horas gratuitas o de los créditos del plan gratuito.
2. Disco raíz gp3 cifrado de máximo 30 GB.
3. Apagado todos los días a las 22:00 y encendido de lunes a viernes a las 08:00, hora de Ciudad de México, con EventBridge Scheduler; fines de semana apagada.
4. Security Group solo con 80 y 443 públicos, 22 únicamente desde la IP de quien desarrolla (o cerrado y acceso por Systems Manager) y el puerto de la API solo desde esa misma IP. Las bases de datos nunca se exponen.
5. HTTPS automático con certificado válido, sin comprar dominio (`IP-con-guiones.sslip.io`) o con dominio propio si existe.
6. Secretos en AWS Systems Manager Parameter Store (SecureString); ninguno en Git ni en Terraform.
7. Alerta de gasto con un presupuesto mensual de US$1.
8. Un script de verificación de solo lectura que falle si algo sale del Free Tier o queda expuesto.
9. Todo como código en `infra/aws-dev`, repetible con `terraform apply` y desechable con `terraform destroy`.

## Diseño
- **Cómputo:** `t3.micro` (cuenta anterior al 15-jul-2025: 750 h/mes durante 12 meses) o `t3.small` (cuenta posterior: créditos durante 6 meses). Créditos de CPU en modo `standard` para evitar cargos por ráfagas. IMDSv2 obligatorio. Amazon Linux 2023, con Docker, Compose, swap de 2 GB y rotación de logs.
- **MongoDB en Atlas** (decisión del revisor, 2026-11-03): la instancia no corre MongoDB. La API usa el Cluster0 de MongoDB Atlas (plan gratuito) con `MONGO_URI` desde Parameter Store, y la IP elástica se agrega al Network Access de Atlas. En la EC2 solo quedan la API, Redis, Neo4j y Caddy.
- **Memoria:** API, Redis y Neo4j caben en 1 GB con límites (Redis 64 MB, Neo4j con heap de 256 MB) y 2 GB de swap; en cuenta nueva se puede usar `t3.small`.
- **IP:** una IP elástica fija, porque cada apagado cambiaría la IP automática y rompería la URL de la app y el certificado.
- **Proxy:** Caddy en Docker (`network_mode: host`) pide y renueva el certificado de Let's Encrypt y reenvía a `127.0.0.1:<puerto de la API>`. Su volumen de datos conserva el certificado entre apagados.
- **Horas:** 14 h × 5 días ≈ 300 h/mes de cómputo; la IP elástica ocupa ≈ 744 h/mes.
- **Permisos:** el rol del Scheduler solo puede encender y apagar esa instancia. El rol de la instancia solo lee los parámetros de `/<proyecto>/<entorno>/` y usa Systems Manager.
- **Secretos:** `deploy-env.sh` arma el `.env` en el servidor desde Parameter Store. Se usa Parameter Store y no Secrets Manager porque este último cuesta US$0.40 por secreto al mes; es una excepción a la constitución (§3 y §6) solo para desarrollo, registrada en `Memoria.md`.
- **Despliegue de la app:** manual. El repositorio es privado, así que la imagen se construye en la máquina de quien despliega (`docker build`) y se copia al servidor por SSH (`scripts/desplegar.sh`: `docker save | ssh … docker load` y `docker compose up -d`). Sin registro de imágenes, sin credenciales de GitHub en el servidor. El CD sigue fuera de alcance.
- **Entorno de la API:** `APP_ENV=staging` (sin OpenAPI, con HSTS, `detalle` visible); sin `SMTP_HOST` los correos solo se registran (nv-06).
- **Estado de Terraform:** local, fuera de Git. No contiene secretos. Un estado remoto (S3) queda para cuando haya más de una persona desplegando.

## Contratos
Variables de `infra/aws-dev/variables.tf` (obligatorias: `ssh_cidr`, `budget_email`): `region`, `instance_type`, `volume_gb` (8–30), `swap_gb`, `ssh_enabled`, `key_name`, `app_port`, `app_port_cidr`, `domain_name`, `timezone`, `stop_expression`, `start_expression`, `start_enabled`, `budget_usd`.

Salidas: `instance_id`, `public_ip`, `url_https`, `ssm_session`, `verificar`.

Rutas que el despliegue debe responder por HTTPS: `GET /health` (200), `GET /ready` (200; 503 si falta MongoDB o Neo4j; 200 `degraded` si falta Redis) y `GET /api/v1/ping` (200). Sin endpoints nuevos.

## Tests
- `terraform fmt -check` y `terraform validate` sin errores.
- `trivy config infra/` (o `tfsec`) sin hallazgos altos.
- `scripts/verificar.sh` termina en 0: tipo elegible, créditos `standard`, IMDSv2, disco ≤ 30 GB, solo 22/80/443/puerto de la app, ningún puerto de base de datos, SSH sin `0.0.0.0/0`, una IP elástica asociada y ninguna suelta, sin NAT, reglas de apagado activas y presupuesto creado.
- Prueba manual: apagar y encender la instancia; la IP no cambia, los contenedores vuelven solos y `/ready` responde sin intervención (MongoDB en Atlas sigue aceptando la IP elástica).
- Prueba manual: la app móvil (o `curl`) llega por HTTPS con certificado válido.

## Criterios
- `terraform plan` crea solo: 1 instancia, 1 IP elástica, 1 Security Group con sus reglas, 2 roles, 1 perfil, 2 reglas del Scheduler y 1 presupuesto.
- `scripts/verificar.sh` en verde.
- Sin secretos en el repositorio (`critic B1 → PASS`).
- Con la instancia apagada, el costo de cómputo es cero.

## Límites
- Solo desarrollo: sin datos reales, sin alta disponibilidad, sin copias de seguridad automáticas (las instantáneas de disco cuestan). Los datos del servidor (Redis y Neo4j) se consideran desechables; MongoDB vive en Atlas.
- El Network Access de Atlas y el usuario de base de datos se administran a mano en Atlas, fuera de Terraform.
- Sin CD ni despliegue desde CI. Sin balanceador, NAT, RDS, ElastiCache ni Secrets Manager (no son gratuitos).
- Sin WAF ni monitoreo avanzado; solo el presupuesto y el script.
- El plan gratuito dura 12 meses (cuenta anterior a jul-2025) o 6 meses (cuenta nueva); al terminar, o se cierra el entorno o se paga. Hay que anotar la fecha en `Memoria.md`.
- `sslip.io` depende de un servicio de terceros y de los límites de Let's Encrypt; con dominio propio desaparece esa dependencia.
- Los precios y los límites del Free Tier cambian; se confirman en la página oficial de AWS antes de aplicar.
- Producción real (multi-AZ, TLS administrado, estado remoto, CD) será una spec posterior.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador sin ejecutar
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Ajustes del revisor antes de ejecutar (2026-11-03):** MongoDB en Atlas en lugar de Docker en la EC2; región `us-east-2`; perfil `wherever-dev` sin usuario raíz (usuario IAM `wherever-deployer` con la política `wherever-dev-deployer`). Cuenta creada el 2026-11-03 (plan gratuito nuevo: US$100 de crédito, vence el 2027-05-02): `t3.micro`, y el presupuesto no descuenta créditos.
- **Detalle:** especificada; tareas aún sin ejecutar. La evidencia irá en `specs/nv-07/checklists/gates.md` + `Memoria.md`.
