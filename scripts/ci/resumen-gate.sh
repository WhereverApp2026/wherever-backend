#!/usr/bin/env bash
# Gate del PR (spec nv-09): recibe líneas "etapa|resultado|hallazgos" por stdin, escribe la tabla
# en $GITHUB_STEP_SUMMARY (o en stdout, en local) y sale con 1 si una etapa falló o se canceló.
# Una etapa omitida por el filtro de rutas cuenta como buena.
set -euo pipefail

summary="${GITHUB_STEP_SUMMARY:-/dev/stdout}"
failed=0
seen=0

{
	echo "## Gate de seguridad del PR"
	echo
	echo "| Etapa | Estado | Hallazgos |"
	echo "|---|---|---|"
} >>"$summary"

while IFS='|' read -r stage result findings; do
	[[ -n "$stage" ]] || continue
	seen=$((seen + 1))
	case "$result" in
		success) icon="✅ pasó" ;;
		skipped) icon="⏭️ omitida (solo documentación)" ;;
		*)
			icon="❌ ${result:-sin resultado}"
			failed=$((failed + 1))
			;;
	esac
	echo "| $stage | $icon | ${findings:--} |" >>"$summary"
done

# Sin etapas no hay nada que afirmar: el gate no puede pasar por no haber visto nada.
if ((seen == 0)); then
	echo "resumen-gate: no llegó ninguna etapa" >&2
	exit 1
fi

{
	echo
	if ((failed > 0)); then
		echo "**Resultado: ❌ $failed etapa(s) obligatoria(s) fallaron.**"
	else
		echo "**Resultado: ✅ todas las etapas obligatorias pasaron.**"
	fi
} >>"$summary"

((failed == 0))
