# plan.md — NV-09 CI de seguridad

1. Mapeo
- Crear: `.trivyignore` (vacío al inicio, solo con excepciones fechadas), `.zap/plan.yaml`, `scripts/ci/cambios.sh` (filtro), `scripts/ci/resumen-gate.sh` (escribe la tabla en `$GITHUB_STEP_SUMMARY`). `.golangci.yml` ya existe; `.semgrepignore` no se crea.
- Modificar:
  - `.github/workflows/ci-pr.yml` (jobs `changes`, `guardrails`, `semgrep`, `go-security`, `image-scan`, `pr-gate`; `pipeline` pierde lo que se movió).
  - `.github/workflows/nightly.yml` (job `dast-zap`).
  - `.specify/memory/constitution.md` (§3, §6 y §8), `AGENTS.md`, `CHECKLIST_PR.md`, `Memoria.md`, `.gitignore`.
- Sin cambios: código de la API, contratos, `docs/endpoints.md`, `cd-dev.yml`, `infra/`.
- Rama: `nv-09-ci-seguridad` sale de `nv-08-cd`, porque depende de ese `Dockerfile` y esa enmienda; el PR apunta a `main`, porque `ci-pr` solo corre ahí.

2. Guardarraíles
- Acciones fijadas por SHA, `permissions` mínimos (`contents: read`).
- Sin secretos, cuentas ni tokens nuevos; Semgrep sin login.
- Ningún paso se silencia con `|| true` ni `continue-on-error`.
- Cada excepción lleva motivo y fecha de revisión; no se baja ningún umbral existente (gate de cobertura 45 %).
- Los jobs omitidos por el filtro cuentan como buenos en el gate; los fallidos o cancelados, no.
- Este PR cambia CI y su orden; `critic B3` no lo detecta porque revisa el orden HTTP. Queda bajo la aprobación del revisor al firmar.
- El PR de referencia no pasa de unos 8 minutos; si lo supera, lo lento se mueve a `nightly.yml`.

3. Pruebas
- Locales: `actionlint`, `yaml.safe_load`, cada herramienta contra el repositorio actual (línea base de hallazgos), `go build`, `go vet`, cinco suites, `check_sdd`, `critic`.
- Reales: PR con hallazgo plantado (rojo), corrida manual de ZAP; PR solo de `docs/` (verde sin Go ni imagen) después de fusionar nv-08 y nv-09.

4. Secuencia
1. Revisor aprueba las decisiones (a)–(d).
2. Línea base: correr las herramientas en local y listar hallazgos existentes.
3. Corregir o exceptuar (con motivo y fecha) lo encontrado.
4. Job `changes` y filtro; luego `semgrep` y `go-security`.
5. `image-scan` (build, Dockle, Trivy).
6. `pr-gate` y reporte.
7. `dast-zap` en `nightly.yml`; corrida manual.
8. Enmienda de la constitución y documentos.
9. PR de referencia con medición de tiempo; PR de prueba con hallazgo plantado. Evidencia en `gates.md`.
9b. Tras fusionar nv-08 y nv-09: PR de solo `docs/`.
10. `Memoria.md`.

5. Verificación
- `actionlint .github/workflows/*.yml → sin errores`
- Corrida del PR de referencia: todos los jobs en verde, resumen del gate visible, duración ≤ 8 min.
- PR de prueba con hallazgo plantado: `pr-gate` en rojo.
- Artefacto del reporte de ZAP en la corrida manual de `nightly.yml`.
