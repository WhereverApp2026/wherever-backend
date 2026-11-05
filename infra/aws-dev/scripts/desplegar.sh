#!/usr/bin/env bash
# Despliegue manual al servidor de desarrollo (spec nv-07). Corre en tu máquina:
#   1) construye la imagen de la API (el repo es privado: el servidor no la clona),
#   2) la copia por SSH sin registro de imágenes,
#   3) sube compose.yml, Caddyfile y deploy-env.sh,
#   4) arma .env desde Parameter Store y levanta los contenedores.
#
# uso: infra/aws-dev/scripts/desplegar.sh --ip <ip> --site <sitio> --key ~/.ssh/wherever-dev [--path /wherever/dev/]
set -euo pipefail

ip="" site="" key="" path="/wherever/dev/"
while [[ $# -gt 0 ]]; do
	case "$1" in
		--ip) ip="$2"; shift 2 ;;
		--site) site="$2"; shift 2 ;;
		--key) key="$2"; shift 2 ;;
		--path) path="$2"; shift 2 ;;
		*) echo "opción desconocida: $1" >&2; exit 2 ;;
	esac
done
[[ -n "$ip" && -n "$site" && -n "$key" ]] || { echo "uso: $0 --ip <ip> --site <sitio> --key <llave privada> [--path /wherever/dev/]" >&2; exit 2; }

root="$(cd "$(dirname "$0")/../../.." && pwd)"
here="$root/infra/aws-dev"
version="$(git -C "$root" rev-parse --short HEAD)"
ssh_opts=(-i "$key" -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15)
remote="ec2-user@$ip"

echo "desplegar: construyendo wherever-api:dev ($version)"
docker build --build-arg VERSION="$version" -t wherever-api:dev "$root"

echo "desplegar: copiando la imagen por SSH"
docker save wherever-api:dev | gzip | ssh "${ssh_opts[@]}" "$remote" 'gunzip | docker load'

echo "desplegar: subiendo compose, Caddyfile y deploy-env.sh"
scp "${ssh_opts[@]}" "$here/server/compose.yml" "$here/server/Caddyfile" "$here/scripts/deploy-env.sh" "$remote:/opt/wherever/"

echo "desplegar: armando .env desde Parameter Store y levantando contenedores"
# Salida de emergencia (nv-08): apaga el actualizador para que no pise esta versión.
ssh "${ssh_opts[@]}" "$remote" "sudo systemctl disable --now wherever-actualizar.timer 2>/dev/null || true; rm -f /opt/wherever/imagen.env; chmod +x /opt/wherever/deploy-env.sh && /opt/wherever/deploy-env.sh '$path' '$site' && cd /opt/wherever && docker compose up -d --remove-orphans && docker compose ps"

echo "desplegar: listo. Prueba: curl -i https://$site/ready"
