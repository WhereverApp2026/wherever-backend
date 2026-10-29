# plan.md — NV-03 Puerta incremental

1. Mapeo
- Crear: `scripts/check_delta_coverage.py`, `.go-mutesting.yml` (o `scripts/mutation.sh`), `scripts/check_coverage.sh`, jobs `delta-check` y `mutation` en `ci-pr.yml` (`continue-on-error`, `fetch-depth: 0`).
- Modificar: `Makefile` (`make cover`, `make delta`, `make mutate`).

2. Guardarraíles
- El único bloqueo de cobertura sigue siendo 45% global.
- El delta se calcula con `git merge-base main` y unión por línea.
- La mutación nunca corre sobre todo el repo en un PR.
- Tras la mutación, `go build ./...` antes de seguir con `-count=1`.

3. Pruebas
- Piloto sobre el código de nv-02: medir delta y score reales antes de fijar cualquier umbral (§2.5).
- `scripts/check_delta_coverage_test.py`: delta vacío, delta con higiene, delta bajo el mínimo (advisory, exit 0).

4. Secuencia
1. `check_coverage.sh` (global, 45%).
2. `check_delta_coverage.py` en modo advisory.
3. Config de mutación enfocada.
4. Jobs de CI con `continue-on-error`.
5. Piloto medido en un slice y registro en `Memoria.md`.

5. Verificación
- `scripts/check_coverage.sh --min 45 → OK`
- `python scripts/check_delta_coverage.py --min 80 --advisory → exit 0 e informa`
- `go build ./... → sin errores` tras la mutación
