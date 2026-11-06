# plan.md — NV-12 Dos instancias de la API detrás de un balanceador y CloudFront

1. Mapeo
- Crear: `infra/aws-dev/alb.tf`, `infra/aws-dev/cdn.tf`, `docs/auditoria-estado.md`.
- Modificar:
  - Código: el cálculo de la IP del cliente y su configuración (`TRUSTED_PROXY_HOPS`, también en `configs/.env.example`), con pruebas; middleware de compresión en `internal/http`; sonda `/api/v1/probe/lenta`; `docs/rate-limit-matrix.md`.
  - Infra: `infra/aws-dev/main.tf` o `api.tf` (segunda instancia, IP elástica, grupos de seguridad, permiso del rol de la instancia), `variables.tf`, `outputs.tf`, calendarios de apagado y encendido.
  - Servidor: `actualizar.sh` (salida y reingreso del balanceador, instalación de B condicionada a A), unit del temporizador, `instalar-actualizador.sh` por Systems Manager.
  - Pipeline: `.github/workflows/cd-dev.yml` (URL de CloudFront, espera de unos 10 min, lecturas seguidas, 5xx sostenidos como apagado, etiqueta `org.opencontainers.image.revision`), variable `API_URL`.
  - Otros: `iam/wherever-dev-boundary.json`, `iam/wherever-dev-deployer-policy.json`, `verificar.sh`, `README.md`, `.specify/memory/constitution.md` (§3, §4, §8), `AGENTS.md`, `CHECKLIST_PR.md`, `Memoria.md`.
- Sin cambios: contratos. `docs/endpoints.md` solo suma la sonda, si `check_endpoints` la exige.

2. Guardarraíles
- La corrección de la IP del cliente va en su propio PR, probada y fusionada antes de crear el balanceador. La compresión también va en su propio PR, en producción antes de quitar Caddy.
- Antes de escribir Terraform y con los permisos ya ampliados, recurso de prueba de ELB y de CloudFront creado y borrado; si una política de la organización los bloquea, se detiene.
- Sin secretos nuevos en Git ni Terraform.
- Grupo de seguridad de las APIs: 8080 solo desde el balanceador; 22 cerrado (instalación por Systems Manager).
- `terraform plan` sin reemplazos inesperados; `apply` solo con confirmación del revisor y respaldo del estado antes y después.
- Una instancia a la vez en cada actualización; nunca se saca la única instancia sana; siempre se vuelve a registrar.
- Caddy y `sslip.io` siguen hasta pasar todas las pruebas y cambiar la app.
- Primero el boundary y después la política del deployer en la consola.

3. Pruebas
- Locales: `go build`, `go vet`, cinco suites, pruebas de la IP del cliente, `terraform fmt`/`validate`, `trivy config`, `actionlint`, `critic`, `check_sdd`.
- Reales: salud, aislamiento, caída de una instancia, despliegue escalonado, regreso automático, compresión, `cd-dev` con todo apagado, petición larga, estado compartido y calendario (spec §Tests).

4. Secuencia
1. Revisor aprueba (a)–(e); NV-11 terminada.
2. Revisor amplía boundary y política del deployer en la consola.
3. Verificar que las políticas de la organización permiten ELB y CloudFront (recurso de prueba).
4. PR de la corrección de la IP del cliente; fusionar y desplegar.
5. PR de la compresión y la sonda; fusionar y desplegar. Auditoría de estado local; corregir o aceptar por escrito.
6. Terraform: segunda API, balanceador y CloudFront; `plan` mostrado; `apply` con confirmación.
7. IP elástica nueva en Atlas.
8. Comprobar salud, aislamiento y HTTPS por CloudFront.
9. Instalar el temporizador y `actualizar.sh` en la segunda API; probar despliegue escalonado y regreso automático.
10. Actualizar `API_URL`, `cd-dev.yml` y `REQUEST_TIMEOUT`; cambiar la URL de la app; quitar Caddy y poner `TRUSTED_PROXY_HOPS=2`.
11. Pruebas de caída, compresión, `cd-dev` con todo apagado, petición larga, estado compartido y calendario; documentos, presupuesto y evidencia; `Memoria.md`.

5. Verificación
- `curl -i https://<id>.cloudfront.net/health → 200`, certificado válido
- `curl -i http://<id>.cloudfront.net/ → redirige a HTTPS`
- `curl -i http://<dns-del-balanceador>/health` desde internet sin pasar por CloudFront → sin respuesta
- `curl -sI -H 'Accept-Encoding: gzip' https://<id>.cloudfront.net/ready → Content-Encoding: gzip`
- Dos destinos sanos en el grupo de destino
- `infra/aws-dev/scripts/verificar.sh → exit 0`
