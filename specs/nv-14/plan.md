# plan.md — NV-14 Infraestructura como paso configurado

1. Mapeo
- Crear: `infra/aws-dev/scripts/crear.sh`, `infra/aws-dev/scripts/destruir.sh`.
- Modificar: `infra/aws-dev/scripts/actualizar.sh` (primer arranque), `infra/aws-dev/scripts/instalar-actualizador.sh` (`--site`, espera de `cloud-init`, `Caddyfile`), `infra/aws-dev/README.md`, `.specify/memory/constitution.md` (§3), `AGENTS.md`, `Memoria.md`.
- Sin cambios: código de la API, contratos, Terraform, `ci-pr.yml`, `cd-dev.yml`, política del deployer.

2. Guardarraíles
- Sin claves de AWS en GitHub.
- Los dos scripts muestran el plan y exigen la confirmación escrita; solo aplican el plan de esa corrida.
- Respaldo del estado antes y después de cada corrida.
- `instalar-actualizador.sh --site` no repite el primer arranque si ya hay `.env`.
- El ciclo completo solo en una ventana acordada con el equipo.

3. Pruebas
- Locales: `go build`, `go vet`, cinco suites, `bash -n`, `terraform fmt`/`validate`, `trivy config`, `critic`, `check_sdd`.
- Reales: `--solo-plan` de ambos, instalador en el servidor vivo y ciclo completo.

4. Secuencia
1. Respuesta del docente.
2. Scripts y primer arranque.
3. `--solo-plan` y el instalador en el servidor vivo.
4. PR.
5. Terminar NV-12 y adaptar los scripts.
6. Ciclo completo en la ventana acordada; evidencia; firma.

5. Verificación
- `crear.sh --solo-plan` → sin cambios.
- `destruir.sh --solo-plan` → lista los recursos y no borra nada.
- Tras el ciclo: `curl -i <url>/ready → 200` y `/health` con el SHA de `main`.
