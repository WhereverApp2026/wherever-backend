# plan.md — NV-13 Observabilidad en la nube

1. Mapeo
- Crear: `infra/aws-dev/observabilidad.tf`, `docs/observabilidad.md` (consultas guardadas y cómo leer las alarmas).
- Crear también: `server/wherever-sonda.service` y `.timer`, `internal/platform/logger/logger_test.go`.
- Modificar: `server/compose.yml` (controlador `awslogs`), `internal/platform/logger/logger.go` (correos y credenciales en texto libre), `deploy-env.sh`, `actualizar.sh` (eventos de despliegue), `instalar-actualizador.sh` (por Systems Manager), `infra/aws-dev/main.tf` (permisos del rol de la instancia), `outputs.tf`, `verificar.sh`, `iam/wherever-dev-boundary.json`, `iam/wherever-dev-deployer-observabilidad.json` (política nueva), `README.md`, `.specify/memory/constitution.md` (§3), `AGENTS.md`, `CHECKLIST_PR.md`, `Memoria.md`.
- Sin cambios: contratos, `docs/endpoints.md`, `ci-pr.yml`, `cd-dev.yml`.

2. Guardarraíles
- Ningún log con cuerpos de petición, contraseñas, tokens ni correos completos.
- Permisos de logs acotados al grupo `/wherever/dev/api`; sin `Resource: "*"` salvo donde AWS lo exige.
- `terraform plan` no reemplaza la EC2 ni la IP elástica; `apply` solo con confirmación y respaldo del estado.
- La suscripción de correo se confirma por el operador; el correo no se guarda en Git.
- Se mide el volumen de logs antes de dejarlo corriendo.

3. Pruebas
- Locales: `go build`, `go vet`, cinco suites, `terraform fmt`/`validate`, `trivy config`, `critic`, `check_sdd`.
- Reales: log en CloudWatch, error provocado con alarma y correo, consulta de Insights.

4. Secuencia
1. Revisor aprueba (a)–(e).
2. Revisor amplía el boundary y la política del deployer en la consola (primero el boundary).
3. Revisar los campos de log de la API y quitar cualquier dato sensible.
4. `observabilidad.tf`; `plan` mostrado; `apply` con confirmación.
5. Operador confirma la suscripción de SNS.
6. Revisar antes el trabajo nocturno pendiente de NV-15, porque el paso siguiente borra esos logs. `compose.yml` con `awslogs` y la sonda por `instalar-actualizador.sh`; reiniciar la API; comprobar el flujo en CloudWatch.
7. Provocar un error y comprobar métrica, alarma y correo.
8. Medir el volumen diario y ajustar.
9. Documentos y evidencia; `Memoria.md`.

5. Verificación
- `aws logs tail /wherever/dev/api --since 5m` muestra la petición de prueba.
- Alarma en estado `ALARM` tras el error provocado y correo recibido.
- `infra/aws-dev/scripts/verificar.sh → exit 0`
