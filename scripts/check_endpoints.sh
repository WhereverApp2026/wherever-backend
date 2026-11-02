#!/usr/bin/env bash
# check_endpoints — anti-drift entre las rutas del código y docs/endpoints.md (skill drift-guards).
# Toda ruta registrada necesita su fila y toda fila necesita su ruta. Sale con 0 si coinciden.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

DOC="docs/endpoints.md"

normalize() {
	# Fiber usa :param y la doc {param}: ambos se comparan como {}.
	sed -E 's#:[A-Za-z_]+#{}#g; s#\{[A-Za-z_]+\}#{}#g' | sort -u
}

code_routes=$(go run ./scripts/routes | tr -d '\r' | normalize)

# Filas de la tabla: "| MÉTODO | `ruta` ... |". Una celda puede listar varias rutas.
doc_routes=$(grep -E '^\| (GET|POST|PUT|PATCH|DELETE) \|' "$DOC" | while IFS='|' read -r _ method cell _; do
	method=$(echo "$method" | tr -d ' ')
	grep -o -E '`/[^`]*`' <<<"$cell" | tr -d '`' | sed "s#^#${method} #"
done | normalize)

missing_doc=$(comm -23 <(echo "$code_routes") <(echo "$doc_routes"))
missing_code=$(comm -13 <(echo "$code_routes") <(echo "$doc_routes"))

status=0
if [[ -n "$missing_doc" ]]; then
	echo "FAIL: rutas sin fila en $DOC:"
	echo "$missing_doc"
	status=1
fi
if [[ -n "$missing_code" ]]; then
	echo "FAIL: filas de $DOC sin ruta en el código:"
	echo "$missing_code"
	status=1
fi
if [[ $status -eq 0 ]]; then
	echo "check_endpoints: OK ($(echo "$code_routes" | wc -l | tr -d ' ') rutas)"
fi
exit $status
