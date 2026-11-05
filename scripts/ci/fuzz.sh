#!/usr/bin/env bash
# Corre cada objetivo de fuzz con tiempo acotado (spec nv-10). Los busca en el código, así un
# objetivo nuevo entra solo. Sigue aunque uno falle y al final sale con 1 si alguno falló.
#   uso: scripts/ci/fuzz.sh [tiempo por objetivo, 60s por defecto] [carpeta de logs]
set -uo pipefail

fuzztime="${1:-60s}"
logs="${2:-bin/fuzz}"
summary="${GITHUB_STEP_SUMMARY:-/dev/stdout}"
mkdir -p "$logs"

mapfile -t packages < <(go list ./internal/... ./cmd/...)
failed=0
total=0

{
	echo "## Fuzz ($fuzztime por objetivo)"
	echo
	echo "| Objetivo | Resultado | Ejecuciones |"
	echo "|---|---|---|"
} >>"$summary"

for pkg in "${packages[@]}"; do
	mapfile -t targets < <(go test -list '^Fuzz' "$pkg" 2>/dev/null | grep '^Fuzz' || true)
	for target in "${targets[@]}"; do
		[[ -n "$target" ]] || continue
		total=$((total + 1))
		log="$logs/$target.log"
		if go test "$pkg" -run '^$' -fuzz "^$target\$" -fuzztime "$fuzztime" >"$log" 2>&1; then
			result="✅"
		else
			result="❌"
			failed=$((failed + 1))
		fi
		execs="$(grep -oE 'execs: [0-9]+' "$log" | tail -1 | cut -d' ' -f2)"
		echo "| ${pkg##*/}.$target | $result | ${execs:--} |" >>"$summary"
		echo "$target $result ${execs:--}"
	done
done

echo >>"$summary"
echo "**$total objetivos, $failed con fallos.** Un fallo deja su caso en \`testdata/fuzz/\` y se corrige con esa semilla." >>"$summary"

if ((total == 0)); then
	echo "fuzz: no se encontró ningún objetivo" >&2
	exit 1
fi
((failed == 0))
