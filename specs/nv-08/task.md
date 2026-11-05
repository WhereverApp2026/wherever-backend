# NV-08 — Despliegue continuo al entorno de desarrollo

## T1 — Permisos
- **Hacer**: con el usuario administrador, actualizar `wherever-dev-boundary` y `wherever-dev-deployer-policy` con las versiones recortadas de `infra/aws-dev/iam/` (`wherever-deployer` no puede, a propósito).
- **Verificar**: `iam:ListRoleTags` responde; `ecr:DescribeRepositories` ya no.
- **No hacer**: usar el usuario raíz o `dev-user`; dar permisos de administrador.

## T2 — Imagen con índices
- **Modificar**: `Dockerfile` (binario `/indexes`), `.dockerignore` (permitir `scripts/indexes`).
- **Verificar**: `docker build .`; `/indexes` contra el compose local dos veces seguidas → `indexes: OK`; sin `.env` ni secretos en la imagen.

## T3 — Boundary
- **Modificar**: `permissions_boundary` en los dos roles de `infra/aws-dev/main.tf`.
- **Hacer**: copiar el estado fuera del repositorio, mostrar el `plan` y aplicar solo con confirmación.
- **Verificar**: `fmt`, `validate`; el `plan` no reemplaza nada; Systems Manager sigue `Online`; `/ready` 200; ciclo real de apagado y encendido.

## T4 — Servidor
- **Crear**: `scripts/actualizar.sh`, `scripts/instalar-actualizador.sh`, `server/wherever-actualizar.service`, `server/wherever-actualizar.timer`.
- **Modificar**: `server/compose.yml` (`API_IMAGE`), `scripts/deploy-env.sh` (excluir `GHCR_TOKEN`), `scripts/desplegar.sh` (apagar el timer).
- **Verificar**: `bash -n`; en la instancia, una imagen buena se instala, una rota regresa y queda en `failed_digest`, y una versión actual sin `/ready` no registra fallos.

## T5 — Workflow
- **Crear**: `.github/workflows/cd-dev.yml` y la variable del repositorio `API_URL`.
- **Incluir**: filtros de `workflow_run`, validación de `sha`, subida idempotente, re-etiquetado a `dev`, 5 versiones, regreso manual y tabla de `/health`.
- **Verificar**: `actionlint`, `yaml.safe_load`, acciones fijadas por SHA, un solo despliegue a la vez, ningún `${{ inputs.sha }}` dentro de un `run`.
- **No hacer**: tocar el orden de `ci-pr.yml`; guardar claves de AWS en GitHub.

## T6 — Token
- **Hacer**: el dueño crea el PAT clásico con solo `read:packages` y lo carga en `/wherever/dev/GHCR_TOKEN` como SecureString.
- **Verificar**: `deploy-env.sh` no lo escribe en `.env`; no aparece en `docker inspect` de la API.
- **No hacer**: pegarlo en el chat ni guardarlo en el repositorio.

## T7 — Pruebas reales
- **Hacer**: camino feliz, regreso automático, arranque, sin acceso a AWS, token aislado, filtros, regreso manual, entrada inválida, idempotencia y ventana.
- **Verificar**: `/health` muestra el SHA esperado en cada caso; `verificar.sh → 0`.

## T8 — Constitución y documentos
- **Modificar**: `.specify/memory/constitution.md` §3 y §8 (incluida la frase "`main` protegida"), `AGENTS.md`, `CHECKLIST_PR.md`, `infra/aws-dev/README.md`.
- **Verificar**: `critic`, `check_sdd`, `check_endpoints`.

## T9 — Gates y memoria
- **Hacer**: retirar `wherever-sin-limites`; evidencia en `gates.md`.
- **Actualizar**: `Memoria.md` (cierre de la excepción del binario local, riesgo aceptado de `main`, mantenimiento manual de los scripts del servidor).
- **Reportar**: evidencia de Sin acceso a AWS y de Regreso automático en el reporte final.
- **Al final**: `terraform destroy` y `apply` de nv-07, actualizando Atlas y `API_URL`.
