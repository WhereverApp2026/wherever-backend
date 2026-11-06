#!/usr/bin/env bash
# Crea o actualiza la infraestructura de desarrollo y deja la API corriendo (spec nv-14). Corre en la
# máquina del operador, con terraform.tfvars y el perfil de AWS. Es idempotente.
#   uso: infra/aws-dev/scripts/crear.sh [--solo-plan] [--profile wherever-dev] [--region us-east-2]
set -euo pipefail
export MSYS_NO_PATHCONV=1

only_plan=false profile="wherever-dev" region="us-east-2"
while [[ $# -gt 0 ]]; do
	case "$1" in
		--solo-plan) only_plan=true; shift ;;
		--profile) profile="$2"; shift 2 ;;
		--region) region="$2"; shift 2 ;;
		*) echo "opción desconocida: $1" >&2; exit 2 ;;
	esac
done

scripts="$(cd "$(dirname "$0")" && pwd)"
# pwd -W: terraform.exe no entiende rutas de Git Bash, y MSYS_NO_PATHCONV evita que se conviertan.
here="$(cd "$scripts/.." && { pwd -W 2>/dev/null || pwd; })"
backups="$HOME/wherever-dev-terraform-respaldo"
ready_wait=1200
tf() { terraform -chdir="$here" "$@"; }
aws_() { aws --profile "$profile" --region "$region" --output text "$@" | tr -d '\r'; }
backup() {
	mkdir -p "$backups"
	if [[ -f "$here/terraform.tfstate" ]]; then
		cp "$here/terraform.tfstate" "$backups/terraform.tfstate.$(date +%Y%m%d-%H%M%S).$1"
	fi
}

plan="$(mktemp -u "$here/crear.XXXXXX.tfplan")"
trap 'rm -f "$plan"' EXIT

tf init -input=false -no-color >/dev/null
backup antes-de-crear
echo "crear: calculando el plan"
tf plan -input=false -no-color -out="$(basename "$plan")" | grep -E '^  # |^Plan:|^No changes' || true
if [[ "$only_plan" == "true" ]]; then
	echo "crear: solo plan; no se cambió nada"
	exit 0
fi

if tf show -no-color "$(basename "$plan")" | grep -q '^Plan:'; then
	read -r -p "Escribe CREAR para aplicar este plan: " answer
	[[ "$answer" == "CREAR" ]] || { echo "crear: cancelado; no se cambió nada"; exit 1; }
	tf apply -input=false -no-color "$(basename "$plan")" | grep -E 'Apply complete|Error' || true
	backup despues-de-crear
fi

url="$(tf output -raw cloudfront_url)"
target_group="$(tf output -raw target_group_arn)"
ips="$(tf output -json public_ips | python -I -c 'import json,sys; print(" y ".join(sorted(v + "/32" for v in json.load(sys.stdin).values())))')"
nodes="$(tf output -json instance_ids | python -I -c 'import json,sys; [print(k, v) for k, v in sorted(json.load(sys.stdin).items())]' | tr -d '\r')"

while read -r node instance; do
	state="$(aws_ ec2 describe-instances --instance-ids "$instance" --query 'Reservations[0].Instances[0].State.Name')"
	if [[ "$state" == "stopped" || "$state" == "stopping" ]]; then
		echo "crear: la instancia $node ($instance) está apagada por el horario; enciéndela y vuelve a correr este script."
		exit 1
	fi
	echo "crear: esperando a Systems Manager en $node ($instance)"
	ping=""
	for _ in $(seq 1 60); do
		ping="$(aws_ ssm describe-instance-information --filters "Key=InstanceIds,Values=$instance" \
			--query 'InstanceInformationList[0].PingStatus' 2>/dev/null || true)"
		[[ "$ping" == "Online" ]] && break
		sleep 10
	done
	[[ "$ping" == "Online" ]] || { echo "crear: $instance no aparece en Systems Manager" >&2; exit 1; }
	"$scripts/instalar-actualizador.sh" --instance "$instance" --node "$node" --target-group "$target_group" \
		--public-url "$url" --profile "$profile" --region "$region" </dev/null
done <<<"$nodes"

cat <<EOF

crear: la infraestructura está lista. Falta lo que no se puede automatizar:
  1. Atlas → Network Access: permite $ips. La API no se instala hasta que lo hagas.
  2. Confirma el correo de AWS "Subscription Confirmation", si llegó uno nuevo.
  3. Si la URL cambió, actualiza el repositorio y avisa al equipo móvil:
       gh variable set API_URL --body "$url"

crear: esperando hasta $((ready_wait / 60)) minutos a que $url/ready dé 200
EOF
deadline=$((SECONDS + ready_wait))
code=000
while ((SECONDS < deadline)); do
	code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$url/ready" || true)"
	[[ "$code" == "200" ]] && break
	sleep 15
done
if [[ "$code" != "200" ]]; then
	echo "crear: $url/ready sigue en $code; revisa el paso 1 y los logs del actualizador" >&2
	exit 1
fi
echo "crear: listo. $(curl -s --max-time 10 "$url/health" || true)"
