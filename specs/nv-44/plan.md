# plan.md — NV-44 Infraestructura de imágenes: S3 y CloudFront

1. Mapeo
- Crear: `infra/aws-dev/imagenes.tf`, `infra/aws-dev/iam/wherever-dev-deployer-imagenes.json`.
- Modificar: `infra/aws-dev/iam/wherever-dev-boundary.json`, `outputs.tf`, `scripts/crear.sh`, `scripts/destruir.sh`, `scripts/verificar.sh`, `README.md`, `Memoria.md`.
- Sin cambios: código Go (lo cablea nv-45), `internal/moderacion` (nv-43).

2. Guardarraíles
- Sin secretos en Git; bucket sin acceso público y sin ACL; permisos acotados al bucket y a sus dos prefijos.
- Sin `terraform apply`/`plan` contra la cuenta ni escrituras en Parameter Store desde la sesión: los hace el operador.

3. Secuencia
1. Revisar drift y corregir la spec.
2. Terraform, IAM y scripts.
3. Validar (`fmt`, `validate`, Trivy, guardarraíles) y documentar.

4. Verificación
- `terraform fmt -check`, `terraform validate`, `trivy config`, `critic`, `check_sdd`, `check_endpoints`; tras el `apply` del operador, `verificar.sh`.
