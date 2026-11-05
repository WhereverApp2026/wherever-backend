#!/usr/bin/env bash
# Decide si un cambio toca código (spec nv-09). Lee rutas por stdin, una por línea, e imprime
# code=true o code=false. Solo documentación da false; ante la duda, true: sin rutas, o con
# cualquier ruta que no sea claramente documentación, corren todos los jobs.
#   uso: git diff --name-only <base>...<head> | scripts/ci/cambios.sh
set -uo pipefail

seen=false
while IFS= read -r path; do
	path="${path%$'\r'}"
	[[ -n "$path" ]] || continue
	seen=true
	case "$path" in
		docs/* | specs/* | .specify/*) ;;
		.github/* | .semgrep/* | scripts/* | deployments/* | infra/*)
			echo "code=true"
			exit 0
			;;
		*.md) ;;
		*)
			echo "code=true"
			exit 0
			;;
	esac
done

if [[ "$seen" == "true" ]]; then
	echo "code=false"
else
	echo "code=true"
fi
