# task.md — NV-09 CI de seguridad

## Fase 0 — Preparación
- [x] T01 Decisiones (a)–(d) dadas al ordenar ejecutarla; se confirman al firmar.
- [x] T02 Rama `nv-09-ci-seguridad`, desde `nv-08-cd` porque depende de ese PR aún sin fusionar.

## Fase 1 — Línea base (Req. 2, 3, 4, 7)
- [x] T03 Correr Semgrep, gosec, govulncheck, golangci-lint, Dockle y Trivy de imagen en local; guardar la lista de hallazgos.
- [x] T04 Sin hallazgos que corregir ni exceptuar (ver `gates.md`).

## Fase 2 — Workflows (Req. 1–5, 8, 9)
- [x] T05 Job `changes` con filtro por rutas.
- [x] T06 Job `semgrep`.
- [x] T07 Job `go-security` (gosec, govulncheck, golangci-lint).
- [x] T08 Job `image-scan` (build con `--provenance=false`, Dockle, Trivy).
- [x] T09 Job `pr-gate` y `scripts/ci/resumen-gate.sh`.
- [x] T10 Acciones fijadas por SHA y `permissions` mínimos en todo lo nuevo.

## Fase 3 — Nocturno (Req. 6)
- [x] T11 Job `dast-zap` en `nightly.yml` (compose + `/ready` + ZAP baseline + artefacto).
- [ ] T12 Corrida manual (`workflow_dispatch`) y revisión del reporte.

## Fase 4 — Documentos
- [x] T13 Enmienda de la constitución §3, §6 y §8.
- [x] T14 `AGENTS.md`, `CHECKLIST_PR.md`, `Memoria.md`.

## Fase 5 — Pruebas y PR
- [ ] T15 `actionlint`, `yaml.safe_load`, `go build`, `go vet`, cinco suites, `check_sdd`, `critic`.
- [ ] T16 PR de referencia: medir duración (≤ 8 min).
- [ ] T17 PR de prueba con hallazgo plantado → rojo; cerrar sin fusionar.
- [ ] T18 PR de prueba solo `docs/` → verde sin Go ni imagen; cerrar sin fusionar.
- [ ] T19 Evidencia en `gates.md`; el revisor firma. `critic B3` no aplica: revisa el orden HTTP, no CI.
