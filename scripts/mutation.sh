#!/usr/bin/env bash
# Mutación (nv-03, advisory) con go-mutesting. Nunca bloquea; en CI el paso además lleva
# continue-on-error. Corre en una copia temporal del árbol (archivos versionados y nuevos no
# ignorados): go-mutesting reemplaza el archivo original mientras prueba cada mutante, y en el
# árbol de trabajo eso puede terminar en un commit con un mutante.
#
# Uso:
#   scripts/mutation.sh [--base origin/main]   PR: archivos cambiados dentro de MUTATION_SCOPE
#   scripts/mutation.sh --full                 nightly: todo internal/ sin tests ni higiene
#
# MUTATION_SCOPE (regex sobre la ruta) acota la corrida de PR; por defecto el service y los
# tokens de auth, para que quepa en los 15 min del paso de CI. Tras la mutación siempre corre
# `go build ./...` (AGENTS.md §4) en el árbol real; ese sí falla si algo quedó roto.
set -euo pipefail
export LC_NUMERIC=C

# Versión fijada: el fork de avito-tech no publica tags.
MUTESTING="github.com/avito-tech/go-mutesting/cmd/go-mutesting@v0.0.0-20261002081621-9ce5f731e321"
# Misma higiene que scripts/check_delta_coverage.py.
HYGIENE_RE='^(cmd/[^/]+/main\.go|internal/testutil/.*|.*/mocks/.*|.*mock.*\.go|.*/dto\.go|scripts/indexes/.*)$'
SCOPE_RE="${MUTATION_SCOPE:-^internal/auth/(service|tokens)\.go$}"

BASE=""
FULL=0
while [[ $# -gt 0 ]]; do
	case "$1" in
		--base) BASE="${2:-}"; shift 2 ;;
		--full) FULL=1; shift ;;
		*) echo "uso: $0 [--base <rama>] [--full]" >&2; exit 2 ;;
	esac
done
cd "$(dirname "$0")/.."
ROOT="$(pwd)"
mkdir -p bin

if [[ $FULL -eq 1 ]]; then
	label="completa"
	mapfile -t files < <(git ls-files -- 'internal/*.go' | grep -v '_test\.go$' | grep -Ev "$HYGIENE_RE" || true)
else
	if [[ -z "$BASE" ]]; then
		BASE=main
		git rev-parse --verify --quiet origin/main >/dev/null && BASE=origin/main
	fi
	merge_base="$(git merge-base "$BASE" HEAD)"
	label="base ${merge_base:0:12}, alcance $SCOPE_RE"
	mapfile -t files < <({ git diff --name-only --diff-filter=AM "$merge_base" -- 'internal/*.go'; git ls-files -o --exclude-standard -- 'internal/*.go'; } \
		| sort -u | grep -v '_test\.go$' | grep -Ev "$HYGIENE_RE" | grep -E "$SCOPE_RE" || true)
fi

if [[ ${#files[@]} -eq 0 ]]; then
	echo "mutation: sin archivos para mutar ($label)"
	exit 0
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
git ls-files -z -c -o --exclude-standard | tar --null -T - -cf - | tar -xf - -C "$work"

echo "mutation: $label, ${#files[@]} archivo(s): ${files[*]}"
start=$SECONDS
set +e
(cd "$work" && go run "$MUTESTING" --config=.go-mutesting.yml "${files[@]}") > bin/mutation.log 2>&1
status=$?
set -e
grep -E '^The mutation score' bin/mutation.log || echo "mutation: sin score (go-mutesting salió con $status); ver bin/mutation.log"
echo "mutation: $((SECONDS - start)) s (copia temporal; el árbol de trabajo no se toca)"

cd "$ROOT"
go build ./...
echo "mutation: go build ./... OK"
