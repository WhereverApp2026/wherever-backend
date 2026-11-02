#!/usr/bin/env bash
# Mutación enfocada en lo cambiado (nv-03, advisory): go-mutesting sobre los .go de
# internal/ cambiados desde `git merge-base`, sin tests ni higiene. Informa el score y
# nunca bloquea; en CI el paso además lleva continue-on-error.
# Uso: scripts/mutation.sh [--base origin/main]
# Tras la mutación siempre corre `go build ./...` (AGENTS.md §4); ese sí falla si algo quedó roto.
set -euo pipefail
export LC_NUMERIC=C

# Versión fijada: el fork de avito-tech no publica tags.
MUTESTING="github.com/avito-tech/go-mutesting/cmd/go-mutesting@v0.0.0-20261002081621-9ce5f731e321"
# Misma higiene que scripts/check_delta_coverage.py.
HYGIENE_RE='^(cmd/[^/]+/main\.go|internal/testutil/.*|.*/mocks/.*|.*mock.*\.go|.*/dto\.go|scripts/indexes/.*)$'

BASE=""
while [[ $# -gt 0 ]]; do
	case "$1" in
		--base) BASE="${2:-}"; shift 2 ;;
		*) echo "uso: $0 [--base <rama>]" >&2; exit 2 ;;
	esac
done
cd "$(dirname "$0")/.."
if [[ -z "$BASE" ]]; then
	BASE=main
	git rev-parse --verify --quiet origin/main >/dev/null && BASE=origin/main
fi

merge_base="$(git merge-base "$BASE" HEAD)"
mapfile -t files < <(git diff --name-only --diff-filter=AM "$merge_base" -- 'internal/*.go' \
	| grep -v '_test\.go$' | grep -Ev "$HYGIENE_RE" || true)

if [[ ${#files[@]} -eq 0 ]]; then
	echo "mutation: sin archivos cambiados para mutar (base ${merge_base:0:12})"
	exit 0
fi

echo "mutation: base ${merge_base:0:12}, ${#files[@]} archivo(s): ${files[*]}"
start=$SECONDS
set +e
go run "$MUTESTING" --config=.go-mutesting.yml "${files[@]}" > bin/mutation.log 2>&1
status=$?
set -e
grep -E '^The mutation score' bin/mutation.log || echo "mutation: sin score (go-mutesting salió con $status); ver bin/mutation.log"
echo "mutation: $((SECONDS - start)) s"
rm -f report.json

go build ./...
echo "mutation: go build ./... OK"
