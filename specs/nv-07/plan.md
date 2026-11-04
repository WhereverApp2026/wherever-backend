# plan.md — NV-07 Despliegue de desarrollo en AWS

1. Mapeo
- Crear: `infra/aws-dev/{main.tf,variables.tf,outputs.tf,user_data.sh.tpl,terraform.tfvars.example,.gitignore,README.md}`, `infra/aws-dev/server/{compose.yml,Caddyfile}`, `infra/aws-dev/scripts/{verificar.sh,deploy-env.sh,desplegar.sh}`.
- Modificar: `.gitignore` raíz (`*.tfstate`, `terraform.tfvars`, `.terraform/`), `README.md`, `AGENTS.md` (comandos y estructura), `Memoria.md`. El compose del servidor es aparte (`infra/aws-dev/server/compose.yml`: API, Redis y Neo4j sin `ports` de bases, `restart: unless-stopped`, límites de memoria); el de `deployments/` sigue siendo solo de desarrollo local. MongoDB en Atlas.

2. Guardarraíles
- Nada de `0.0.0.0/0` en SSH; `ssh_cidr` lo rechaza.
- Ningún puerto de base de datos en el Security Group.
- Créditos de CPU `standard`; IMDSv2 obligatorio; disco cifrado.
- Sin secretos en Git ni en el estado de Terraform; solo Parameter Store.
- `critic B1` sigue vigente y debe pasar sobre `infra/`.

3. Pruebas
- `terraform fmt -check`, `terraform validate`, `trivy config infra/`.
- `scripts/verificar.sh` con la instancia encendida y apagada.
- Ciclo manual de apagado y encendido; prueba por HTTPS.

4. Secuencia
1. Cuenta: confirmar fecha de creación, usuario IAM sin raíz, tipo elegible.
2. Variables y `terraform plan`.
3. `terraform apply` y espera del arranque.
4. Secretos en Parameter Store.
5. Despliegue de la app con `deploy-env.sh` y Compose.
6. Verificación y ciclo de apagado/encendido.
7. `Memoria.md`: fecha de fin del plan gratuito, excepción de Parameter Store, deuda de producción.

5. Verificación
- `terraform fmt -check && terraform validate`
- `./scripts/verificar.sh --url $(terraform output -raw url_https) → exit 0`
- `curl -i $(terraform output -raw url_https)/ready → 200`
