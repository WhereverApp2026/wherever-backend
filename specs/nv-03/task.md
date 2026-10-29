# NV-03 — Puerta de Calidad Incremental (advisory)

## T1 — Borradores + diseños, sin firmar
- **Crear**: `specs/nv-03/spec.md` (puerta de 6 capas, advisory, higiene), `plan.md`, `task.md`, `checklists/gates.md`; estado 🚧 Borrador.

## T2 — Implementar y medir (se ejecuta después)
- **Implementar**: `scripts/check_delta_coverage.py --min 80 --advisory`, `scripts/check_coverage.sh`, config de mutación enfocada, jobs `delta-check + mutation` en `ci-pr.yml` (`continue-on-error`, `fetch-depth: 0`).
- **Medir**: piloto en el código de nv-02; registrar delta, score y tiempo reales en `Memoria.md`.
- **Conciliar**: mapa de fases en `Memoria.md` (`nv-02` = molde, `nv-03` = puerta).
- **Verificar**: `check_sdd SDD-OK + critic PASS + endpoints OK + build sin errores`.
- **No hacer**: cambiar el 45% o el CI canónico, bloquear por mutación (exige enmienda §7), firmar (solo el revisor humano).
