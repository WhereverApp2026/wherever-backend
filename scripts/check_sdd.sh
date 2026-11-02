#!/usr/bin/env bash
# check_sdd — cada spec en specs/nv-* trae sus artefactos y las 7 secciones + Aprobación
# (constitución §11). Imprime SDD-OK y sale con 0 si todo está.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# Las specs usan dos variantes de nombres (plantilla numerada y la de nv-00/nv-01);
# se exige la estructura: 7 secciones de nivel 2 más la de Aprobación.
MIN_SECTIONS=7
FILES=("spec.md" "plan.md" "task.md" "checklists/gates.md")

failures=0
specs=0
for dir in specs/nv-*/; do
	specs=$((specs + 1))
	for file in "${FILES[@]}"; do
		if [[ ! -f "$dir$file" ]]; then
			echo "FAIL: falta $dir$file"
			failures=$((failures + 1))
		fi
	done
	[[ -f "${dir}spec.md" ]] || continue
	sections=$(grep -c -E '^## ' "${dir}spec.md" || true)
	approval=$(grep -c -E '^## Aprobación' "${dir}spec.md" || true)
	if [[ $approval -ne 1 || $((sections - approval)) -lt $MIN_SECTIONS ]]; then
		echo "FAIL: ${dir}spec.md necesita $MIN_SECTIONS secciones '## ' más una '## Aprobación' (tiene $sections)"
		failures=$((failures + 1))
	fi
	if ! grep -q -E '^- \*\*Estado:\*\*' "${dir}spec.md"; then
		echo "FAIL: ${dir}spec.md sin '**Estado:**' en la aprobación"
		failures=$((failures + 1))
	fi
done

if [[ $specs -eq 0 ]]; then
	echo "FAIL: no hay specs en specs/nv-*"
	exit 1
fi
if [[ $failures -gt 0 ]]; then
	echo "check_sdd: FAIL ($failures)"
	exit 1
fi
echo "SDD-OK ($specs specs)"
