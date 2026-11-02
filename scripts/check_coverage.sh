#!/usr/bin/env bash
# Cobertura global (constitución §7): único bloqueo de cobertura hoy.
# Uso: scripts/check_coverage.sh --min 45 [perfil.out ...]
#   Con perfiles (los que dejan unit, integration y security en CI) los une y mide.
#   Sin perfiles corre la suite Unit con -coverpkg para medirla sola.
# Deja el perfil unido en bin/coverage.out para scripts/check_delta_coverage.py.
set -euo pipefail
export LC_NUMERIC=C

MIN=""
PROFILES=()
while [[ $# -gt 0 ]]; do
	case "$1" in
		--min) MIN="${2:-}"; shift 2 ;;
		*) PROFILES+=("$1"); shift ;;
	esac
done
[[ "$MIN" =~ ^[0-9]+([.][0-9]+)?$ ]] || { echo "uso: $0 --min <porcentaje> [perfil.out ...]" >&2; exit 2; }

cd "$(dirname "$0")/.."
mkdir -p bin
OUT=bin/coverage.out
COVERPKG="./internal/...,./cmd/..."

if [[ ${#PROFILES[@]} -eq 0 ]]; then
	go test ./internal/... ./cmd/... -count=1 -covermode=atomic -coverpkg="$COVERPKG" -coverprofile=bin/coverage-unit.out >/dev/null
	PROFILES=(bin/coverage-unit.out)
fi

tmp="$(mktemp bin/coverage.XXXXXX)"
trap 'rm -f "$tmp"' EXIT
mode=""
for p in "${PROFILES[@]}"; do
	[[ -s "$p" ]] || { echo "check_coverage: falta el perfil $p" >&2; exit 2; }
	m="$(head -n 1 "$p")"
	[[ -z "$mode" ]] && { mode="$m"; echo "$mode" > "$tmp"; }
	[[ "$m" == "$mode" ]] || { echo "check_coverage: modos distintos ($mode / $m)" >&2; exit 2; }
	tail -n +2 "$p" >> "$tmp"
done
mv "$tmp" "$OUT"
trap - EXIT

# go tool cover suma los bloques repetidos de varios perfiles.
total="$(go tool cover -func="$OUT" | awk '/^total:/ { sub("%", "", $NF); print $NF }')"
if awk -v t="$total" -v m="$MIN" 'BEGIN { exit !(t + 0 >= m + 0) }'; then
	echo "check_coverage: OK (${total}% >= ${MIN}%)"
else
	echo "check_coverage: FAIL (${total}% < ${MIN}%)" >&2
	exit 1
fi
