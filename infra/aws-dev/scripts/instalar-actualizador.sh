#!/usr/bin/env bash
# Sube al servidor el actualizador de nv-08 y la sonda de nv-13, y activa sus timers. Corre en tu
# máquina y entra por Systems Manager, sin SSH; repítelo cada vez que cambien estos archivos, porque
# el CD solo actualiza la imagen de la API. No reinicia la API: eso lo hace la siguiente imagen.
# Con --site, si la instancia es nueva y no tiene .env, también hace el primer arranque (nv-14).
#   uso: instalar-actualizador.sh --instance <id> [--site <sitio>] [--profile wherever-dev] [--region us-east-2]
set -euo pipefail
export MSYS_NO_PATHCONV=1

instance="" site="" profile="wherever-dev" region="us-east-2"
while [[ $# -gt 0 ]]; do
	case "$1" in
		--instance) instance="$2"; shift 2 ;;
		--site) site="$2"; shift 2 ;;
		--profile) profile="$2"; shift 2 ;;
		--region) region="$2"; shift 2 ;;
		*) echo "opción desconocida: $1" >&2; exit 2 ;;
	esac
done
[[ -n "$instance" ]] || { echo "uso: $0 --instance <id> [--profile <perfil>] [--region <región>]" >&2; exit 2; }

here="$(cd "$(dirname "$0")/.." && pwd)"
aws_() { aws --profile "$profile" --region "$region" --output text "$@" | tr -d '\r'; }

# Corre un comando en la instancia y espera su resultado; falla si el comando falla.
remote() {
	local params id status=""
	params="$(printf '%s' "$1" | python -I -c 'import json,sys; print(json.dumps({"commands":[sys.stdin.read()]}))')"
	id="$(aws_ ssm send-command --instance-ids "$instance" --document-name AWS-RunShellScript \
		--parameters "$params" --query Command.CommandId)"
	# Hasta 10 minutos: la primera orden espera a que termine el arranque de la instancia.
	for _ in $(seq 1 300); do
		status="$(aws_ ssm get-command-invocation --command-id "$id" --instance-id "$instance" --query Status 2>/dev/null || true)"
		case "$status" in Success | Failed | Cancelled | TimedOut) break ;; esac
		sleep 2
	done
	aws_ ssm get-command-invocation --command-id "$id" --instance-id "$instance" --query StandardOutputContent
	if [[ "$status" != "Success" ]]; then
		aws_ ssm get-command-invocation --command-id "$id" --instance-id "$instance" --query StandardErrorContent >&2
		echo "instalar-actualizador: el comando terminó en $status" >&2
		exit 1
	fi
}

# Un comando por archivo: cada uno cabe holgado en el límite de tamaño de Systems Manager.
upload() {
	echo "instalar-actualizador: subiendo $(basename "$2")"
	remote "printf '%s' '$(base64 -w0 "$1")' | base64 -d > '$2'"
}

# Una instancia nueva crea /opt/wherever al final de su arranque (user_data).
echo "instalar-actualizador: esperando el arranque de la instancia"
remote 'cloud-init status --wait >/dev/null || true; test -d /opt/wherever'

for file in server/compose.yml server/Caddyfile scripts/deploy-env.sh scripts/actualizar.sh; do
	upload "$here/$file" "/opt/wherever/$(basename "$file")"
done
for file in wherever-actualizar.service wherever-actualizar.timer wherever-sonda.service wherever-sonda.timer; do
	upload "$here/server/$file" "/etc/systemd/system/$file"
done

# Antes de los timers: el actualizador necesita .env, y Caddy no lo levanta nadie más.
first_boot() {
	remote "set -e
cd /opt/wherever
if [ -f .env ]; then echo 'instalar-actualizador: ya tiene .env; sin primer arranque'; exit 0; fi
chmod +x deploy-env.sh
./deploy-env.sh /wherever/dev/ '$site'
chown ec2-user:ec2-user .env
docker compose --env-file .env up -d --no-deps caddy"
}

timers='set -e
cd /opt/wherever
chown ec2-user:ec2-user compose.yml Caddyfile deploy-env.sh actualizar.sh
chmod +x deploy-env.sh actualizar.sh
cd /etc/systemd/system
chown root:root wherever-actualizar.service wherever-actualizar.timer wherever-sonda.service wherever-sonda.timer
chmod 644 wherever-actualizar.service wherever-actualizar.timer wherever-sonda.service wherever-sonda.timer
systemctl daemon-reload
systemctl enable --now wherever-actualizar.timer wherever-sonda.timer
systemctl list-timers "wherever-*" --no-pager'

remote 'cd /opt/wherever; chmod +x deploy-env.sh actualizar.sh'
if [[ -n "$site" ]]; then
	first_boot
fi
remote "$timers"
