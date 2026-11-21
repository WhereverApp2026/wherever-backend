#!/usr/bin/env bash
# Rotación de secretos de Parameter Store (plan de seguridad, control 14 de docs/seguridad.md).
# Corre en la máquina del operador con el perfil de AWS. Nunca imprime un valor: los nuevos se
# generan aquí y van a AWS por un archivo temporal (no por la línea de comandos).
#
#   rotar-secretos.sh estado                      antigüedad de cada secreto, sin cambiar nada
#   rotar-secretos.sh propios [--aplicar]         rota JWT_KEY, CURSOR_HMAC_KEY y agrega una llave a DATA_KEYS
#   rotar-secretos.sh cargar NOMBRE [--aplicar]   carga una llave de terceros (Stripe, Gemini, SMTP…) ya rotada
#                                                 en su consola, pedida sin mostrarla
#   opciones: --profile wherever-dev --region us-east-2 --sin-reinicio
#
# Cada secreto queda como parámetro avanzado con NoChangeNotification: si pasa el plazo sin rotarse,
# EventBridge avisa al correo de alarmas (rotacion.tf). Propios: 90 días; de terceros: 180.
# Sin --aplicar solo dice qué haría. Con --aplicar reinicia la API en las dos instancias, una a la
# vez y esperando /ready, para que tomen los valores nuevos.
#
# Qué rompe cada rotación (es lo esperado):
#   JWT_KEY          los access vigentes (15 min) dan 401; la app refresca con su refresh (no es JWT).
#   CURSOR_HMAC_KEY  los cursores en uso dan 422; la app pide la primera página o descarga todo.
#   DATA_KEYS        nada: la llave nueva va primero (cifra lo nuevo) y las anteriores se quedan
#                    para descifrar lo viejo. `make migrar-cifrado` re-cifra las cuentas con la nueva.
# PHONE_HMAC_KEY no se rota: los teléfonos solo se guardan como su HMAC y no se pueden recalcular.
set -euo pipefail
export MSYS_NO_PATHCONV=1
umask 077

profile="wherever-dev" region="us-east-2" apply=false restart=true
prefix="/wherever/dev/"
own_days=90 third_days=180
own=(JWT_KEY CURSOR_HMAC_KEY DATA_KEYS)

cmd="${1:-}"
[[ -n "$cmd" ]] && shift
name=""
if [[ "$cmd" == "cargar" ]]; then
	name="${1:?uso: rotar-secretos.sh cargar NOMBRE [--aplicar]}"
	shift
fi
while [[ $# -gt 0 ]]; do
	case "$1" in
		--aplicar) apply=true; shift ;;
		--sin-reinicio) restart=false; shift ;;
		--profile) profile="$2"; shift 2 ;;
		--region) region="$2"; shift 2 ;;
		*) echo "opción desconocida: $1" >&2; exit 2 ;;
	esac
done

aws_() { aws --profile "$profile" --region "$region" "$@"; }
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# openssl en Git Bash termina con \r\n: quitar solo \n dejó un \r en los secretos y partió
# DATA_KEYS en el .env del servidor (2026-11-20).
random_b64() { openssl rand -base64 "$1" | tr -d '\r\n'; }

# En Git Bash (Windows), python y aws son programas de Windows y no entienden /tmp/…: se les pasa
# la ruta de Windows. En Linux cygpath no existe y la ruta queda igual.
native() { if command -v cygpath >/dev/null; then cygpath -m "$1"; else printf '%s' "$1"; fi; }

# put escribe el valor de un archivo como SecureString avanzado con su aviso de antigüedad.
put() {
	local param="$1" value_file="$2" days="$3"
	python -I - "$param" "$(native "$value_file")" "$days" "$(native "$work/entrada.json")" <<'PY'
import json, sys
name, value_file, days, out = sys.argv[1:]
value = open(value_file, encoding="utf-8", newline="").read()
if not value or "\r" in value or "\n" in value:
    sys.exit("rotar-secretos: valor vacío o con saltos de línea; no se cambió " + name)
policies = [{"Type": "NoChangeNotification", "Version": "1.0",
             "Attributes": {"After": days, "Unit": "Days"}}]
json.dump({"Name": name, "Value": value, "Type": "SecureString", "Overwrite": True,
           "Tier": "Advanced", "Policies": json.dumps(policies)}, open(out, "w"))
PY
	aws_ ssm put-parameter --cli-input-json "file://$(native "$work/entrada.json")" --query Version --output text >/dev/null
	rm -f "$work/entrada.json"
	echo "rotar-secretos: $param actualizado (aviso a los $days días sin cambio)"
}

estado() {
	aws_ ssm describe-parameters --parameter-filters "Key=Path,Option=Recursive,Values=${prefix%/}" \
		--query 'Parameters[].[Name,LastModifiedDate,Tier]' --output text | tr -d '\r' |
		python -I -c '
import sys, datetime
now = datetime.datetime.now(datetime.timezone.utc)
print("%-42s %5s  %s" % ("parametro", "dias", "nivel"))
for line in sorted(sys.stdin):
    name, when, tier = line.rstrip("\n").split("\t")
    try:
        ts = datetime.datetime.fromisoformat(when)
    except ValueError:
        ts = datetime.datetime.fromtimestamp(float(when), datetime.timezone.utc)
    print("%-42s %5d  %s" % (name, (now - ts).days, tier))'
}

restart_api() {
	[[ "$restart" == "true" ]] || { echo "rotar-secretos: sin reinicio; la API tomará los valores en el siguiente despliegue"; return; }
	local ids
	ids="$(aws_ ec2 describe-instances --filters "Name=tag:Name,Values=wherever-dev-api-*" "Name=instance-state-name,Values=running" \
		--query 'Reservations[].Instances[].InstanceId' --output text | tr -d '\r')"
	[[ -n "$ids" ]] || { echo "rotar-secretos: no hay instancias encendidas; la API tomará los valores al encender" >&2; return; }
	for id in $ids; do
		echo "rotar-secretos: reiniciando la API en $id"
		local command
		# shellcheck disable=SC2016 # el script se expande en la instancia, no aquí
		command="$(aws_ ssm send-command --instance-ids "$id" --document-name AWS-RunShellScript \
			--parameters 'commands=["set -e","cd /opt/wherever","./deploy-env.sh /wherever/dev/","chown ec2-user:ec2-user .env","docker compose --env-file .env --env-file imagen.env up -d --no-deps api","for n in $(seq 1 40); do c=$(curl -s -o /dev/null -w %{http_code} http://localhost:8080/ready || true); [ \"$c\" = 200 ] && exit 0; sleep 3; done; echo ready=$c; exit 1"]' \
			--query Command.CommandId --output text | tr -d '\r')"
		aws_ ssm wait command-executed --command-id "$command" --instance-id "$id" 2>/dev/null || {
			echo "rotar-secretos: la API de $id no quedó lista; revisa con Systems Manager antes de seguir" >&2
			exit 1
		}
	done
	echo "rotar-secretos: API reiniciada en todas las instancias"
}

case "$cmd" in
	estado)
		estado
		;;
	propios)
		if [[ "$apply" != "true" ]]; then
			echo "rotar-secretos: rotaría ${own[*]} y reiniciaría la API. Agrega --aplicar."
			estado
			exit 0
		fi
		random_b64 48 >"$work/v" && put "${prefix}JWT_KEY" "$work/v" "$own_days"
		random_b64 48 >"$work/v" && put "${prefix}CURSOR_HMAC_KEY" "$work/v" "$own_days"
		aws_ ssm get-parameter --name "${prefix}DATA_KEYS" --with-decryption --query Parameter.Value --output text | tr -d '\r\n' >"$work/actual"
		printf 'k%s:%s,' "$(date -u +%Y%m%d%H%M)" "$(random_b64 32)" >"$work/v"
		cat "$work/actual" >>"$work/v"
		put "${prefix}DATA_KEYS" "$work/v" "$own_days"
		rm -f "$work/v" "$work/actual"
		restart_api
		echo "rotar-secretos: listo. Para re-cifrar las cuentas con la llave nueva: make migrar-cifrado (con -aplicar)."
		;;
	cargar)
		[[ "$name" =~ ^[A-Z0-9_]+$ ]] || { echo "nombre inválido: $name" >&2; exit 2; }
		if [[ "$apply" != "true" ]]; then
			echo "rotar-secretos: pediría el valor nuevo de $name, lo cargaría y reiniciaría la API. Agrega --aplicar."
			exit 0
		fi
		read -rsp "Valor nuevo de $name (no se muestra): " value
		echo
		[[ -n "$value" ]] || { echo "valor vacío; no se cambió nada" >&2; exit 1; }
		printf '%s' "$value" >"$work/v"
		unset value
		put "${prefix}${name}" "$work/v" "$third_days"
		rm -f "$work/v"
		restart_api
		;;
	*)
		sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'
		exit 2
		;;
esac
