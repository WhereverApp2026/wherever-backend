# NV-09 — CI de seguridad: análisis estático, imagen y DAST nocturno

## Contexto
`ci-pr.yml` compila, corre las suites y mide cobertura (gate 45 %). `nightly.yml` corre pruebas largas. Falta análisis de seguridad automático del código y de la imagen. Esta spec lo agrega sin cuentas ni tokens nuevos y sin alargar el PR más de lo necesario. **Depende de**: NV-08 (la imagen sale del `Dockerfile`; el CD solo corre si `ci-pr` termina en verde, así que lo que se agregue a `ci-pr` también puede frenar un despliegue).

## Requisitos
1. Un filtro por rutas decide qué jobs corren: un PR que solo toca `docs/`, `specs/`, `.specify/` o archivos `.md` no corre los jobs de Go ni de imagen, pero sí `guardrails`, `semgrep` y el gate. Un `push` a `main` corre todo, porque de ese resultado depende `cd-dev`.
2. **Semgrep** (CLI, reglas `p/golang`, `p/owasp-top-ten`, `p/secrets`) en PR. Las del registro fallan con severidad ERROR; las propias de `.semgrep/semgrep.yaml` siguen fallando con cualquier severidad, para no bajar el umbral que ya tenían. Sin Semgrep Cloud, sin token y con `--metrics=off`.
3. **gosec**, **govulncheck** y **golangci-lint** en PR. `govulncheck` falla solo con vulnerabilidades alcanzables desde el código.
4. **Imagen en PR:** `docker build` sin publicar (con `--provenance=false`), **Dockle** (falla con FATAL) y **Trivy de imagen** (falla con HIGH/CRITICAL que ya tengan corrección, `--ignore-unfixed`).
5. **Gate de PR y reporte:** un job final agrega los resultados y escribe una tabla en el resumen del workflow (`$GITHUB_STEP_SUMMARY`) con cada etapa, su estado y el conteo de hallazgos. Termina en rojo si falla cualquiera obligatoria.
6. **DAST nocturno:** en `nightly.yml`, levantar las bases con compose y la API con su imagen, crear los índices y correr **OWASP ZAP en modo pasivo**, sembrado con el OpenAPI. Falla con alertas altas. El reporte se guarda como artefacto.
7. Las excepciones (falsos positivos) viven en archivos versionados (`.trivyignore`, comentarios `#nosec` o `nosemgrep` con motivo; `.semgrepignore` solo si hace falta) y cada una lleva motivo y fecha de revisión.
8. Acciones fijadas por SHA, `permissions` mínimos, sin secretos nuevos.
9. Presupuesto: el PR no pasa de unos 8 minutos en total; los jobs corren en paralelo y usan caché de módulos y de capas.

## Diseño
- Jobs en `ci-pr.yml`: `changes` (filtro), `guardrails` (critic, endpoints y sdd, que salen de `pipeline`), `semgrep`, `go-security` (gosec + govulncheck + golangci-lint), `image-scan` (build + Dockle + Trivy), `pipeline` (suites y cobertura) y `pr-gate` (`needs` de todos, `if: always()`, escribe el reporte).
- **Sin duplicar:** `pipeline` ya corría lint, govulncheck, semgrep y `docker build`. Esos pasos se mueven a los jobs nuevos en lugar de repetirse. `critic`, `endpoints` y `sdd` se mueven a `guardrails` porque también aplican a un cambio de documentos.
- **`.semgrepignore` no se crea:** si existe, Semgrep deja de usar sus exclusiones por defecto, y la línea base no necesita ninguna excepción.
- Un job omitido por el filtro cuenta como bueno en el gate; uno fallido o cancelado no.
- `nightly.yml`: job `dast-zap`. `deployments/docker-compose.yml` solo trae las bases, así que la API corre desde su imagen con `APP_ENV=staging` y llaves de un solo uso; antes se corre `scripts/indexes`, porque sin índices `/ready` da 503. Espera `/ready` 200 y corre ZAP contra `http://localhost:8080`.
- **ZAP con plan propio, no `zap-baseline.py`:** la API no tiene páginas que rastrear, y el baseline solo vería la raíz, que da 404. `.zap/plan.yaml` usa el marco de automatización: importa `internal/http/openapi.json` y corre solo el análisis pasivo. El proxy de ZAP va en el puerto 8090, porque el 8080 es de la API. La entrada `solo` de `workflow_dispatch` evita correr la mutación completa en la prueba manual.
- Los resultados no se suben a la pestaña de seguridad de GitHub (requiere GitHub Advanced Security en repos privados): se guardan como artefactos y como resumen.
- **Interacción con el CD:** como `cd-dev` solo despliega si `ci-pr` termina en éxito, una vulnerabilidad nueva con corrección frenaría el despliegue. Es lo deseado. La salida de emergencia es una excepción fechada en `.trivyignore` o `desplegar.sh`.

## Contratos
Sin endpoints ni cambios de código de la API. Enmienda de la constitución: §3 (fila CI), `ci-pr.yml` incluye análisis estático, de imagen y gate, y `nightly.yml` incluye DAST; §6 ya no difiere Trivy ni ZAP; §8, el orden de CI pasa de una sola lista a jobs en paralelo.

## Tests
- `actionlint` y `yaml.safe_load` sobre los workflows tocados.
- Cada herramienta corre en local o en una rama de prueba con el repositorio actual y su resultado de base queda documentado (hallazgos existentes, a corregir o a exceptuar con motivo).
- **Prueba de que frena:** un PR de prueba con un hallazgo plantado (por ejemplo, un secreto falso y una función con `exec.Command` sin validar) debe salir rojo; después se cierra sin fusionar.
- **Prueba del filtro:** un PR que solo toca `docs/` termina en verde sin correr Go ni imagen. Solo se puede hacer con nv-08 y nv-09 ya en `main`; antes, cualquier rama difiere de `main` en código. Mientras, `scripts/ci/cambios.sh` se prueba en local.
- ZAP nocturno: una corrida manual (`workflow_dispatch`) con el reporte como artefacto.
- `go build ./...`, `go vet ./...` y las suites sin cambios.

## Criterios
- Sin secretos ni cuentas nuevas (`critic B1 → PASS`).
- El PR de referencia no pasa de 8 minutos.
- Los hallazgos existentes están corregidos o exceptuados con motivo y fecha.
- El reporte del gate se ve en el resumen de cada corrida.

## Límites
- Sin SonarCloud (no aplica a repos privados en plan gratuito).
- ZAP es pasivo, no escaneo activo; solo ve las rutas sin sesión o sus respuestas 401: sin token no analiza las respuestas autenticadas. El fuzzing de la API va en NV-10.
- Las reglas `p/` de Semgrep vienen del registro público sin versión fija: un cambio allí puede poner un PR en rojo sin cambio de código. Se reemplaza `--config=auto`, que exigía métricas.
- `delta-check` y `mutation` siguen con `continue-on-error`: son advisory por nv-03. La regla de no silenciar pasos aplica a lo nuevo de esta spec.
- La imagen se fija por digest (Dockle, Trivy) o por versión (ZAP); hay que actualizarlas a mano.
- El gate no es obligatorio para fusionar: `main` no se puede proteger en este plan de GitHub (riesgo aceptado, ver NV-08).
- Consume minutos del plan gratuito de Actions; lo lento va a la corrida nocturna.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Implementada en la rama `nv-09-ci-seguridad`; faltan pruebas reales y firma
- **Revisores:** Osmar Reséndiz (pendiente de firmar)
- **Fecha:** —
- **Correcciones al ejecutar (2026-11-05):** `critic B3` revisa el orden de los middlewares HTTP, no CI, así que no marca este cambio; `.golangci.yml` ya existía; compose no levanta la API; `zap-baseline` no vería rutas; `.semgrepignore` quitaría exclusiones; `ci-pr` solo corre en PR hacia `main`, por lo que nv-09 sale de nv-08 y el PR apunta a `main`.
- **Decisiones del revisor, dadas al ordenar ejecutarla el 2026-11-05 (confirmar al firmar):** (a) Semgrep con reglas del registro público, sin cuenta ni token; (b) el gate del PR informa pero no bloquea la fusión (`main` no se puede proteger); (c) umbrales de fallo: Semgrep ERROR, Trivy HIGH/CRITICAL con corrección, Dockle FATAL, ZAP alto; (d) enmienda de la constitución §3.
- **Detalle:** la evidencia irá en `specs/nv-09/checklists/gates.md` + `Memoria.md`.
