#!/usr/bin/env bash
# Mide cuántos comandos de Redis gasta cada operación de la API (spec nv-11, requisito 4).
# Corre contra una API local y el Redis de deployments/docker-compose.yml: usa CONFIG RESETSTAT,
# que no se debe lanzar contra el Redis gestionado. Lo que cuenta es lo que Upstash cobra:
# MULTI y EXEC sí; AUTH, HELLO, PING, SELECT, QUIT, INFO, CONFIG y COMMAND no.
#
# Uso: scripts/medir-redis.sh [url de la API]   (por defecto http://127.0.0.1:8080)
set -euo pipefail

api="${1:-http://127.0.0.1:8080}"
compose_file="${COMPOSE_FILE_MEDIR:-deployments/docker-compose.yml}"
correo="medir-$(date +%s)@example.com"
password='Medicion-Redis-2026!'
device='{"modelo":"medicion","sistema":"linux","app_version":"1.0.0"}'
json='Content-Type: application/json'

rcli() { docker compose -f "$compose_file" exec -T redis redis-cli "$@" | tr -d '\r'; }

# Suma las llamadas de cada comando que se cobra y deja el detalle en una línea.
cobrados() {
	rcli INFO commandstats | awk -F'[_:=,]' '
		/^cmdstat_/ {
			name = $2
			# cmdstat_client|setinfo:calls=2,...
			for (i = 3; i <= NF; i++) if ($i == "calls") { calls = $(i + 1); break }
			base = name
			sub(/\|.*/, "", base)
			if (base ~ /^(auth|hello|reset|ping|select|quit|info|config|command)$/) next
			total += calls
			detail = detail sprintf("%s=%s ", name, calls)
		}
		END { printf "%d|%s\n", total, detail }'
}

# medir <nombre> <comando...>: corre la operación y anota sus comandos.
medir() {
	local name=$1
	shift
	rcli CONFIG RESETSTAT >/dev/null
	"$@" >/dev/null
	# La liberación del contador de escrituras corre al terminar la respuesta.
	sleep 0.2
	local result
	result="$(cobrados)"
	printf '%-34s %4s   %s\n' "$name" "${result%%|*}" "${result#*|}"
}

http() { curl -s --max-time 10 -H "$json" "$@"; }
campo() { sed -n "s/.*\"$1\":\"\([^\"]*\)\".*/\1/p"; }

registro='{"nombre":"Medicion","correo":"'"$correo"'","password":"'"$password"'","fecha_nacimiento":"1990-01-01","acepta_terminos":true,"version_terminos":"2026-10","promociones_correo":false,"dispositivo":'"$device"'}'
login='{"correo":"'"$correo"'","password":"'"$password"'","dispositivo":'"$device"'}'
login_malo='{"correo":"'"$correo"'","password":"Otra-Clave-2026!","dispositivo":'"$device"'}'

rcli FLUSHALL >/dev/null
printf '%-34s %4s   %s\n' 'operación' 'cmds' 'detalle'

access="$(http -X POST "$api/api/v1/usuarios" -d "$registro" | campo access_token)"
if [[ -z "$access" ]]; then
	echo "medir-redis: no se pudo registrar la cuenta de medición en $api" >&2
	exit 1
fi

medir 'GET /health' http "$api/health"
medir 'GET /ready' http "$api/ready"
medir 'GET autenticado (perfil)' http -H "Authorization: Bearer $access" "$api/api/v1/usuarios/yo"
medir 'PATCH autenticado (privacidad)' http -X PATCH -H "Authorization: Bearer $access" \
	"$api/api/v1/usuarios/yo/privacidad" -d '{"datos_sensibles":false}'
medir 'POST registro' http -X POST "$api/api/v1/usuarios" \
	-d "${registro//$correo/otra-$correo}"
medir 'POST login correcto' http -X POST "$api/api/v1/auth/login" -d "$login"
medir 'POST login fallido' http -X POST "$api/api/v1/auth/login" -d "$login_malo"
refresh="$(http -X POST "$api/api/v1/auth/login" -d "$login" | campo refresh_token)"
medir 'POST refresh' http -X POST "$api/api/v1/auth/refresh" -d '{"refresh_token":"'"$refresh"'"}'
medir 'POST logout' http -X POST -H "Authorization: Bearer $access" "$api/api/v1/auth/logout"
