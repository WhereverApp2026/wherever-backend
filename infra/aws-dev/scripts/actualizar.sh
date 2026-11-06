#!/usr/bin/env bash
# Corre EN la instancia, como root, con wherever-actualizar.timer (spec nv-08). Si la etiqueta
# `dev` de GHCR cambió de digest, instala esa versión; si no queda sana, regresa a la anterior.
set -euo pipefail

image="ghcr.io/whereverapp2026/wherever-backend"
app=/opt/wherever
failed=/var/log/wherever/failed_digest
active="$app/imagen.env"
param=/wherever/dev/GHCR_TOKEN
ready_url=http://127.0.0.1:8080/ready
log_group=/wherever/dev/api
ready_wait=120

mkdir -p /var/log/wherever
touch "$failed"

exec 9>/run/wherever-actualizar.lock
flock -n 9 || exit 0

# El apagado de las 22:00 no debe cortar una actualización a la mitad.
now="$(TZ=America/Mexico_City date +%H%M)"
if ((10#$now >= 2150 && 10#$now < 2200)); then
	exit 0
fi

region="$(cut -d= -f2 "$app/region")"
docker_config="$(mktemp -d)"
export DOCKER_CONFIG="$docker_config"
trap 'rm -rf "$docker_config"' EXIT

aws ssm get-parameter --region "$region" --name "$param" --with-decryption \
	--query Parameter.Value --output text |
	docker login ghcr.io -u wherever-dev --password-stdin >/dev/null

docker pull -q "$image:dev" >/dev/null
digest="$(docker image inspect "$image:dev" --format '{{range .RepoDigests}}{{println .}}{{end}}' | grep -m1 "^$image@sha256:" || true)"
[[ -n "$digest" ]] || { echo "actualizar: no se pudo leer el digest de $image:dev" >&2; exit 1; }

current=""
[[ -f "$active" ]] && current="$(cut -d= -f2- "$active")"
[[ "$digest" == "$current" ]] && exit 0
grep -qxF "$digest" "$failed" && exit 0

# Evento de despliegue en CloudWatch (nv-13). Nunca detiene una actualización: es solo el aviso.
log_event() {
	local token instance stream
	token="$(curl -s -X PUT -H 'X-aws-ec2-metadata-token-ttl-seconds: 60' http://169.254.169.254/latest/api/token)" || return 0
	instance="$(curl -s -H "X-aws-ec2-metadata-token: $token" http://169.254.169.254/latest/meta-data/instance-id)" || return 0
	stream="despliegues-$instance"
	aws logs create-log-stream --region "$region" --log-group-name "$log_group" --log-stream-name "$stream" 2>/dev/null || true
	# En JSON: la forma abreviada de la CLI no admite llaves en el mensaje.
	local events="[{\"timestamp\":$(($(date +%s) * 1000)),\"message\":\"{\\\"evento\\\":\\\"$1\\\",\\\"digest\\\":\\\"$2\\\"}\"}]"
	aws logs put-log-events --region "$region" --log-group-name "$log_group" --log-stream-name "$stream" \
		--log-events "$events" >/dev/null ||
		echo "actualizar: no se pudo registrar el evento $1 en CloudWatch" >&2
}

ready_now() { [[ "$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "$ready_url" || true)" == "200" ]]; }

wait_ready() {
	local deadline=$((SECONDS + ready_wait))
	while ((SECONDS < deadline)); do
		ready_now && return 0
		sleep 5
	done
	return 1
}

# Sin esto, una caída de Atlas o una pausa de Aura parecerían un fallo de la imagen nueva. Una
# instancia recién creada no tiene versión actual: ahí no hay nada que comprobar (nv-14).
if [[ -n "$current" ]] && ! ready_now; then
	echo "actualizar: la versión actual no da /ready 200; se reintenta en la siguiente revisión"
	exit 0
fi

echo "actualizar: instalando $digest"
site="$(grep -m1 '^SITE_ADDRESS=' "$app/.env" | cut -d= -f2-)"
"$app/deploy-env.sh" /wherever/dev/ "$site"
# Corre como root; sin esto, `docker compose` manual como ec2-user ya no puede leer .env.
chown ec2-user:ec2-user "$app/.env"

set +e
docker run --rm --env-file "$app/.env" --entrypoint /indexes "$digest"
indexes_status=$?
set -e
case "$indexes_status" in
	0) ;;
	126 | 127)
		# La imagen no trae /indexes: es anterior a nv-08 y no sirve para este flujo.
		echo "actualizar: $digest no trae /indexes; se registra como fallida" >&2
		echo "$digest" >>"$failed"
		exit 1
		;;
	*)
		echo "actualizar: fallaron los índices ($indexes_status); se reintenta en la siguiente revisión" >&2
		exit 1
		;;
esac

switch_api() {
	cd "$app"
	if [[ -n "$1" ]]; then
		printf 'API_IMAGE=%s\n' "$1" >"$active"
		docker compose --env-file .env --env-file imagen.env up -d --no-deps api
	else
		rm -f "$active"
		docker compose --env-file .env up -d --no-deps api
	fi
}

switch_api "$digest"
if wait_ready; then
	echo "actualizar: $digest instalada"
	log_event instalada "$digest"
	docker image prune -af >/dev/null
	exit 0
fi

echo "actualizar: $digest no dio /ready 200; regresando a la anterior" >&2
switch_api "$current"
if wait_ready; then
	# La anterior quedó sana, así que el problema es de la imagen nueva.
	echo "$digest" >>"$failed"
	echo "actualizar: regreso completo; $digest registrada como fallida" >&2
	log_event regreso "$digest"
else
	echo "actualizar: la versión anterior tampoco da /ready 200; no se registra fallo" >&2
	log_event regreso "$digest"
fi
exit 1
