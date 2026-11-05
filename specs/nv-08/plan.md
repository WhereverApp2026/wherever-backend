# plan.md — NV-08 Despliegue continuo al entorno de desarrollo

1. Mapeo
- Crear: `.github/workflows/cd-dev.yml`, `infra/aws-dev/scripts/actualizar.sh`, `infra/aws-dev/scripts/instalar-actualizador.sh`, `infra/aws-dev/server/wherever-actualizar.{service,timer}`, `infra/aws-dev/iam/wherever-dev-boundary.json`.
- Modificar:
  - `Dockerfile` y `.dockerignore` (binario `/indexes`).
  - `infra/aws-dev/main.tf` (`permissions_boundary` en los dos roles), `server/compose.yml` (`${API_IMAGE:-wherever-api:dev}`), `scripts/deploy-env.sh` (excluir `GHCR_TOKEN`), `scripts/desplegar.sh` (apagar el timer), `iam/wherever-dev-deployer-policy.json`, `README.md`.
  - `.specify/memory/constitution.md` (§3 y §8; el espejo de la raíz solo enlaza), `AGENTS.md`, `CHECKLIST_PR.md`, `Memoria.md`.
- Sin cambios: código de la API, contratos, `docs/endpoints.md`, `ci-pr.yml`.

2. Guardarraíles
- Sin credenciales de AWS en GitHub; el workflow no pide `id-token`.
- `GHCR_TOKEN` nunca llega al `.env` ni a la API; se usa con `--password-stdin` en un `DOCKER_CONFIG` temporal.
- El workflow valida `event`, `conclusion`, `head_branch` y `head_repository`, y construye el `head_sha` exacto.
- `sha` validado con `^[0-9a-f]{40}$`; nunca `${{ inputs.sha }}` dentro de un `run`.
- Todo rol lleva `wherever-dev-boundary`.
- Antes de cada `apply`, copia del estado fuera del repositorio.
- `terraform plan` no debe reemplazar la instancia ni la IP elástica. Si lo propone, se detiene y se revisa.

3. Pruebas
- Locales: `go build`, `go vet`, cinco suites, `docker build`, `/indexes` dos veces seguidas, `actionlint`, `bash -n`, `terraform fmt`/`validate`, `critic`, `check_sdd`.
- Reales: los caminos de la spec §Tests sobre el entorno de desarrollo; Sin acceso a AWS y Regreso automático van con evidencia en el reporte final.

4. Secuencia
1. Revisor define el diseño (hecho el 2026-11-04).
2. El administrador actualiza `wherever-dev-boundary` y `wherever-dev-deployer-policy` en la consola.
3. `apply` del boundary sobre los dos roles: respaldar el estado, mostrar el `plan`, confirmar y comprobar Systems Manager y `/ready`.
4. Ciclo real de apagado y encendido con el boundary puesto.
5. El dueño crea el PAT y carga `/wherever/dev/GHCR_TOKEN`; se define la variable `API_URL`.
6. PR; al fusionar, `cd-dev.yml` publica la primera imagen.
7. `ssh_cidr` con la IP vigente; `instalar-actualizador.sh` sube los scripts y activa el timer.
8. Pruebas reales; evidencia en `gates.md`.
9. `Memoria.md`: se cierra la excepción del binario compilado a mano.
10. Al final, `terraform destroy` y `apply` de nv-07, actualizando Atlas y `API_URL`.

5. Verificación
- `curl -s "$API_URL/health" → version = SHA de main`
- `curl -i "$API_URL/ready" → 200`
- `journalctl -u wherever-actualizar` sin errores
- `infra/aws-dev/scripts/verificar.sh → exit 0`
