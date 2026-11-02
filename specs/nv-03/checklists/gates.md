# Checklist — nv-03 Puerta incremental

> Trazabilidad: `spec §6 ↔ plan §2-§5 ↔ task T1-T2`. Estado: Borrador con evidencia (2026-11-02), sin firma.

- [x] Las 6 capas documentadas y en el orden canónico (`ci-pr.yml`: … sdd → coverage → delta-check → mutation → semgrep; Database y Contract llegan con su primer módulo)
- [x] `check_coverage.sh --min 45` bloqueante (paso `coverage` sin `continue-on-error`)
- [x] `check_delta_coverage.py --min 80 --advisory` informa y no bloquea (exit 0 + `continue-on-error`)
- [x] Mutación enfocada con `continue-on-error` (solo archivos cambiados, `timeout-minutes: 15`)
- [x] Higiene (main.go, mocks, DTOs, testutil, scripts de índices) fuera del delta y de la mutación — `HygieneTest`
- [x] Piloto medido y registrado en `Memoria.md`
- [x] `go build ./...` después de la mutación (en `mutation.sh` y en el paso `build tras mutation`, que sí bloquea)
- [ ] Aprobación firmada: (pendiente de revisor)

## Evidencia (2026-11-02, Go 1.27.0 local, Python 3.11, Windows + Git Bash)
- `python -m unittest discover -s scripts -p "*_test.py"` → 9 OK (delta vacío, delta solo con higiene, bajo el mínimo advisory exit 0, sin advisory exit 1, unión por línea).
- `scripts/check_coverage.sh --min 45` (solo Unit) → OK 56,1 %. Con los perfiles de Unit + Integration + Security → OK 79,6 %. `--min 90` → FAIL exit 1; sin `--min` → exit 2.
- Piloto sobre el código de nv-02 (`--base nv-01-foundation`): delta 93,8 % (60/64 líneas; `cursor.go` 90,5 %, `registry.go` 94,3 %). Contra `origin/main` (nv-02 ya fusionado) → delta vacío, exit 0.
- Piloto de mutación sobre los 6 archivos de nv-02: score 0,56 (23 mutantes muertos, 18 vivos, 2 duplicados, 41 en total) en 107 s; después `go build ./...` OK y el árbol queda limpio.
- `critic-guardrails` PASS, `check_endpoints` OK (10 rutas), `check_sdd` SDD-OK, `semgrep 1.95.0` → 0, `yaml.safe_load` OK en `ci-pr.yml` y `.go-mutesting.yml`.
