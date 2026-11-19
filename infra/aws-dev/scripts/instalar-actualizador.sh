#!/usr/bin/env bash
# Instala cosign, sube al servidor el actualizador de nv-08, la sonda de nv-13 y el respaldo semanal de
# MongoDB, y activa sus timers (el del respaldo solo en el nodo a). Corre en tu
# máquina y entra por Systems Manager, sin SSH; repítelo cada vez que cambien estos archivos, porque
# el CD solo actualiza la imagen de la API. No reinicia la API: eso lo hace la siguiente imagen.
# Si la instancia es nueva y no tiene .env, también hace el primer arranque (nv-14). Con --node, anota
# su lugar en el balanceador (nv-12).
#   uso: instalar-actualizador.sh --instance <id> [--node a|b --target-group <arn> --public-url <url>]
#        [--profile wherever-dev] [--region us-east-2]
set -euo pipefail
export MSYS_NO_PATHCONV=1

instance="" node="" target_group="" public_url="" profile="wherever-dev" region="us-east-2"
while [[ $# -gt 0 ]]; do
	case "$1" in
		--instance) instance="$2"; shift 2 ;;
		--node) node="$2"; shift 2 ;;
		--target-group) target_group="$2"; shift 2 ;;
		--public-url) public_url="$2"; shift 2 ;;
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

# cosign para que actualizar.sh verifique la firma de cada imagen. Binario oficial de la versión
# fijada, comprobado contra el checksum de aquí (de cosign_checksums.txt de esa versión); sin
# coincidencia no se instala. Antes de subir actualizar.sh: el nuevo no instala nada sin cosign.
# Para subir de versión, cambia las tres variables juntas (y COSIGN_RELEASE de cd-dev.yml).
cosign_version=v3.1.4
cosign_sha_amd64=424e2c97d0b45fd7a6b608a87f090f35633f4c23ece5f092d48bba11aaf4bbfa
cosign_sha_arm64=ade83b7aac62e131cacfe96d958948b2150cc9efec46a570a2649b8e27dedad2
# shellcheck disable=SC2016 # se expande en la instancia, no aquí
install_cosign='set -e
case "$(uname -m)" in
x86_64) arch=amd64 sum="$SUM_AMD64" ;;
aarch64) arch=arm64 sum="$SUM_ARM64" ;;
*) echo "instalar-actualizador: arquitectura sin cosign: $(uname -m)" >&2; exit 1 ;;
esac
bin=/usr/local/bin/cosign
if [ -x "$bin" ] && echo "$sum  $bin" | sha256sum -c --quiet >/dev/null 2>&1; then
  echo "instalar-actualizador: cosign $VERSION ya instalado"; exit 0
fi
tmp="$(mktemp)"
trap "rm -f \"$tmp\"" EXIT
curl -fsSL --retry 3 -o "$tmp" "https://github.com/sigstore/cosign/releases/download/$VERSION/cosign-linux-$arch"
echo "$sum  $tmp" | sha256sum -c --quiet
install -m 0755 -o root -g root "$tmp" "$bin"
mkdir -p /var/lib/wherever/sigstore
"$bin" version | grep -i gitversion'
echo "instalar-actualizador: instalando cosign $cosign_version"
remote "VERSION=$cosign_version SUM_AMD64=$cosign_sha_amd64 SUM_ARM64=$cosign_sha_arm64
$install_cosign"

for file in server/compose.yml scripts/deploy-env.sh scripts/actualizar.sh scripts/respaldo-mongo.sh; do
	upload "$here/$file" "/opt/wherever/$(basename "$file")"
done
for file in wherever-actualizar.service wherever-actualizar.timer wherever-sonda.service wherever-sonda.timer \
	wherever-respaldo.service wherever-respaldo.timer; do
	upload "$here/server/$file" "/etc/systemd/system/$file"
done

# Antes de los timers: el actualizador necesita .env.
first_boot='set -e
cd /opt/wherever
chmod +x deploy-env.sh actualizar.sh
if [ -f .env ]; then echo "instalar-actualizador: ya tiene .env; sin primer arranque"; exit 0; fi
./deploy-env.sh /wherever/dev/
chown ec2-user:ec2-user .env'

# shellcheck disable=SC2016 # se expande en la instancia, no aquí
timers='set -e
cd /opt/wherever
chown ec2-user:ec2-user compose.yml deploy-env.sh actualizar.sh
chmod +x deploy-env.sh actualizar.sh
# Corre como root y lee .env: solo root lo cambia.
chown root:root respaldo-mongo.sh
chmod 700 respaldo-mongo.sh
node=""
[ -f nodo.env ] && node="$(sed -n "s/^NODE=//p" nodo.env)"
cd /etc/systemd/system
units="wherever-actualizar.service wherever-actualizar.timer wherever-sonda.service wherever-sonda.timer wherever-respaldo.service wherever-respaldo.timer"
chown root:root $units
chmod 644 $units
systemctl daemon-reload
systemctl enable --now wherever-actualizar.timer wherever-sonda.timer
# Respaldo de MongoDB solo en el nodo a: las dos instancias ven la misma base.
if [ "$node" = a ]; then
  systemctl enable --now wherever-respaldo.timer
else
  systemctl disable --now wherever-respaldo.timer
fi
systemctl list-timers "wherever-*" --no-pager'

if [[ -n "$node" ]]; then
	remote "printf 'NODE=%s\\nTARGET_GROUP_ARN=%s\\nPUBLIC_URL=%s\\n' '$node' '$target_group' '$public_url' > /opt/wherever/nodo.env"
fi
remote "$first_boot"
remote "$timers"
