#!/usr/bin/env bash
# Sube al servidor el actualizador de nv-08 y activa su timer. Corre en tu máquina; repítelo cada
# vez que cambien estos archivos, porque el CD solo actualiza la imagen de la API.
#   uso: infra/aws-dev/scripts/instalar-actualizador.sh --ip <ip> --key ~/.ssh/wherever-dev
set -euo pipefail

ip="" key=""
while [[ $# -gt 0 ]]; do
	case "$1" in
		--ip) ip="$2"; shift 2 ;;
		--key) key="$2"; shift 2 ;;
		*) echo "opción desconocida: $1" >&2; exit 2 ;;
	esac
done
[[ -n "$ip" && -n "$key" ]] || { echo "uso: $0 --ip <ip> --key <llave privada>" >&2; exit 2; }

here="$(cd "$(dirname "$0")/.." && pwd)"
ssh_opts=(-i "$key" -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15)
remote="ec2-user@$ip"

scp "${ssh_opts[@]}" "$here/server/compose.yml" "$here/scripts/deploy-env.sh" "$here/scripts/actualizar.sh" \
	"$here/server/wherever-actualizar.service" "$here/server/wherever-actualizar.timer" "$remote:/opt/wherever/"

ssh "${ssh_opts[@]}" "$remote" 'set -e
cd /opt/wherever
chmod +x deploy-env.sh actualizar.sh
sudo mv wherever-actualizar.service wherever-actualizar.timer /etc/systemd/system/
sudo chown root:root /etc/systemd/system/wherever-actualizar.service /etc/systemd/system/wherever-actualizar.timer
sudo systemctl daemon-reload
sudo systemctl enable --now wherever-actualizar.timer
systemctl list-timers wherever-actualizar.timer --no-pager'
