#!/usr/bin/env bash
# critic-guardrails — gate pre-push con 4 bloqueantes (constitución §6, skill critic-guardrails).
#   B1 secretos en Git · B2 token en logs/errores · B3 pipeline alterado · B4 ruta sin fila en docs/endpoints.md
# nv-00 activa B1 y B2; B3 y B4 se activan en nv-01, cuando existen pipeline y rutas.
# Sale con 0 si pasa y con 1 si algún bloqueante falla.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

SELF="scripts/critic-guardrails.sh"
failures=0

fail() { echo "FAIL $1: $2"; failures=$((failures + 1)); }

# Archivos versionados o por versionar (respeta .gitignore), sin este script.
mapfile -t FILES < <(git ls-files -co --exclude-standard | grep -v -x "$SELF" || true)
mapfile -t GO_FILES < <(printf '%s\n' "${FILES[@]}" | grep -E '\.go$' | grep -v -E '_test\.go$' || true)

check_b1() {
	local hits
	# .env real nunca va a Git; solo la plantilla configs/.env.example.
	hits=$(printf '%s\n' "${FILES[@]}" | grep -E '(^|/)\.env(\..+)?$' | grep -v -x 'configs/.env.example' || true)
	[[ -n "$hits" ]] && fail B1 "archivo .env versionado: $hits"

	[[ ${#FILES[@]} -eq 0 ]] && return 0
	hits=$(grep -n -I -E \
		-e '://[A-Za-z0-9._%-]+:[^@/[:space:]]+@' \
		-e 'BEGIN (RSA |EC |DSA |OPENSSH )?PRIVATE KEY' \
		-e '(sk|rk)_live_[A-Za-z0-9]+' \
		-e 'AKIA[0-9A-Z]{16}' \
		-e '([Pp]assword|PASSWORD|[Ss]ecret|SECRET|[Aa]pi_?[Kk]ey|API_KEY|[Jj]wt_?[Kk]ey|JWT_KEY)[[:space:]]*:?=[[:space:]]*"[^"_]{8,}"' \
		-- "${FILES[@]}" 2>/dev/null || true)
	[[ -n "$hits" ]] && fail B1 "posible secreto en Git:"$'\n'"$hits"
	return 0
}

check_b2() {
	[[ ${#GO_FILES[@]} -eq 0 ]] && return 0
	local calls hits
	calls='(slog|log|logger)\.[A-Za-z]+\(|fmt\.Errorf\(|errors\.New\('
	# Valor de un token como argumento: se quitan los literales y se busca el identificador.
	hits=$(grep -H -n -E "$calls" -- "${GO_FILES[@]}" 2>/dev/null \
		| sed -E 's/"([^"\\]|\\.)*"/""/g' \
		| grep -i -E 'token' || true)
	# Token como atributo de slog: "token", "access_token", "refresh_token".
	hits+=$'\n'$(grep -H -n -E "$calls" -- "${GO_FILES[@]}" 2>/dev/null \
		| grep -i -E '"(access_|refresh_)?token"[[:space:]]*,' || true)
	hits=$(sed '/^$/d' <<<"$hits")
	[[ -n "$hits" ]] && fail B2 "token en logs o errores:"$'\n'"$hits"
	return 0
}

check_b3() {
	if [[ ! -f cmd/api/main.go ]]; then
		echo "SKIP B3: sin pipeline todavía (llega en nv-01)"
		return 0
	fi
	echo "FAIL B3: existe cmd/api/main.go y la verificación del pipeline aún no está implementada (nv-01)"
	failures=$((failures + 1))
}

check_b4() {
	if [[ ${#GO_FILES[@]} -eq 0 ]] || ! grep -q -E '\.(Get|Post|Put|Patch|Delete)\("' -- "${GO_FILES[@]}" 2>/dev/null; then
		echo "SKIP B4: sin rutas registradas todavía (llega en nv-01)"
		return 0
	fi
	echo "FAIL B4: hay rutas y la verificación contra docs/endpoints.md aún no está implementada (nv-01)"
	failures=$((failures + 1))
}

check_b1
check_b2
check_b3
check_b4

if [[ $failures -gt 0 ]]; then
	echo "critic-guardrails: FAIL ($failures)"
	exit 1
fi
echo "critic-guardrails: PASS"
