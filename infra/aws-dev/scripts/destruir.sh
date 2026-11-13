#!/usr/bin/env bash
# Elimina la infraestructura de desarrollo (spec nv-14). Corre en la máquina del operador. Muestra lo
# que va a borrar y exige escribir DESTRUIR. No toca lo que vive fuera de Terraform.
#   uso: infra/aws-dev/scripts/destruir.sh [--solo-plan]
set -euo pipefail
export MSYS_NO_PATHCONV=1

only_plan=false
while [[ $# -gt 0 ]]; do
	case "$1" in
		--solo-plan) only_plan=true; shift ;;
		*) echo "opción desconocida: $1" >&2; exit 2 ;;
	esac
done

# pwd -W: terraform.exe no entiende rutas de Git Bash, y MSYS_NO_PATHCONV evita que se conviertan.
here="$(cd "$(dirname "$0")/.." && { pwd -W 2>/dev/null || pwd; })"
backups="$HOME/wherever-dev-terraform-respaldo"
tf() { terraform -chdir="$here" "$@"; }
backup() {
	mkdir -p "$backups"
	cp "$here/terraform.tfstate" "$backups/terraform.tfstate.$(date +%Y%m%d-%H%M%S).$1"
}

plan="$(mktemp -u "$here/destruir.XXXXXX.tfplan")"
trap 'rm -f "$plan"' EXIT

tf init -input=false -no-color >/dev/null
backup antes-de-destruir
echo "destruir: calculando lo que se va a borrar"
tf plan -destroy -input=false -no-color -out="$(basename "$plan")" | grep -E '^  # |^Plan:|^No changes' || true
if [[ "$only_plan" == "true" ]]; then
	echo "destruir: solo plan; no se borró nada"
	exit 0
fi

cat <<'EOF'

Se pierden los logs de CloudWatch, las IP elásticas y las imágenes del bucket (Terraform lo vacía):
al recrear, las URL de CloudFront de la API y de las imágenes cambian.
No se tocan: Parameter Store, Atlas, Aura, Upstash, GHCR, el boundary ni las políticas del deployer.
EOF
read -r -p "Escribe DESTRUIR para continuar: " answer
[[ "$answer" == "DESTRUIR" ]] || { echo "destruir: cancelado; no se borró nada"; exit 1; }

tf apply -input=false -no-color "$(basename "$plan")" | grep -E 'Apply complete|Destroy complete|Error' || true
backup despues-de-destruir
remaining="$(tf state list | wc -l)"
if [[ "$remaining" != "0" ]]; then
	echo "destruir: quedan $remaining recursos en el estado; revisa la salida" >&2
	tf state list >&2
	exit 1
fi
echo "destruir: listo; no queda infraestructura de Terraform."
