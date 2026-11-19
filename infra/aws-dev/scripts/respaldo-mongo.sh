#!/usr/bin/env bash
# Corre EN la instancia a, como root, con wherever-respaldo.timer (viernes 21:00, hora de México).
# Atlas M0 no tiene respaldos: este script hace un mongodump de la base de MONGO_URI (.env), lo
# sube a S3 como respaldos/AAAA-MM-DD.archive.gz y registra en CloudWatch el evento `respaldo`
# con `estado` ok o error. El rol de la instancia solo puede escribir en respaldos/; restaurar se
# hace desde la máquina del operador (README, «Respaldos de MongoDB»).
#
# La cadena de Atlas nunca va en la línea de comandos ni en el log: viaja por la entrada estándar
# al contenedor, que la deja en un archivo de configuración de mongodump dentro de él.
#
#   uso: respaldo-mongo.sh            respaldo real (solo en la instancia a)
#        respaldo-mongo.sh --seco DIR deja el archivo en DIR; sin S3 ni CloudWatch (probar-respaldo.sh)
# Variables para pruebas: RESPALDO_ENV_FILE (archivo con MONGO_URI) y RESPALDO_RED (red de Docker).
set -euo pipefail

app=/opt/wherever
env_file="${RESPALDO_ENV_FILE:-$app/.env}"
network="${RESPALDO_RED:-bridge}"
log_group=/wherever/dev/api
prefix=respaldos/
# Mismo mayor que Atlas (8.0); trae mongodump 100.20.1. Para subirla, cambia la etiqueta y el
# digest del índice juntos (`docker buildx imagetools inspect mongo:8.0`).
mongo_image=mongo:8.0.38@sha256:bdc216e9d5a6ec2cfdac789b5832fdab64cd32055f59885cae918a92de63c361
container=wherever-respaldo
dump_timeout=40m

dry_dir=""
if [[ "${1:-}" == "--seco" ]]; then
	dry_dir="${2:?uso: respaldo-mongo.sh --seco <directorio>}"
	[[ -d "$dry_dir" ]] || { echo "respaldo: no existe el directorio $dry_dir" >&2; exit 2; }
elif [[ $# -gt 0 ]]; then
	echo "uso: respaldo-mongo.sh [--seco <directorio>]" >&2
	exit 2
fi

# Solo la instancia a respalda: las dos ven la misma base y dos copias por semana no suman nada.
if [[ -z "$dry_dir" ]]; then
	NODE=""
	# shellcheck source=/dev/null
	[[ -f "$app/nodo.env" ]] && . "$app/nodo.env"
	if [[ "$NODE" != "a" ]]; then
		echo "respaldo: esta instancia es el nodo '${NODE:-sin nodo}'; solo la a respalda"
		exit 0
	fi
	exec 9>/run/wherever-respaldo.lock
	flock -n 9 || { echo "respaldo: ya hay un respaldo en curso" >&2; exit 1; }
	region="$(cut -d= -f2 "$app/region")"
fi

umask 077
work="$(mktemp -d)"
day="$(TZ=America/Mexico_City date +%F)"
name="$day.archive.gz"
archive="$work/$name"
step=inicio

instance_id() {
	local token
	token="$(curl -s -X PUT -H 'X-aws-ec2-metadata-token-ttl-seconds: 60' http://169.254.169.254/latest/api/token)" || return 0
	curl -s -H "X-aws-ec2-metadata-token: $token" http://169.254.169.254/latest/meta-data/instance-id
}

# Evento en CloudWatch, flujo respaldos-<instancia>. Sin datos de la base: estado, tamaño y paso.
# Siempre queda también en el journal. Devuelve 1 si CloudWatch no lo recibió.
log_event() {
	local message="$1" instance stream events
	echo "respaldo: evento $message"
	[[ -z "$dry_dir" ]] || return 0
	instance="$(instance_id)"
	[[ -n "$instance" ]] || { echo "respaldo: sin id de instancia; el evento no llegó a CloudWatch" >&2; return 1; }
	stream="respaldos-$instance"
	aws logs create-log-stream --region "$region" --log-group-name "$log_group" --log-stream-name "$stream" 2>/dev/null || true
	events="[{\"timestamp\":$(($(date +%s) * 1000)),\"message\":\"${message//\"/\\\"}\"}]"
	aws logs put-log-events --region "$region" --log-group-name "$log_group" --log-stream-name "$stream" \
		--log-events "$events" >/dev/null || { echo "respaldo: CloudWatch no recibió el evento" >&2; return 1; }
}

finish() {
	local code=$?
	docker rm -f "$container" >/dev/null 2>&1 || true
	rm -rf "$work"
	if ((code != 0)) && [[ "$step" != registrar ]]; then
		echo "respaldo: FALLÓ en el paso '$step' (código $code); no hay respaldo nuevo de $day" >&2
		log_event "{\"evento\":\"respaldo\",\"estado\":\"error\",\"paso\":\"$step\"}" || true
	fi
	exit "$code"
}
trap finish EXIT

step=leer-uri
uri="$(grep -m1 '^MONGO_URI=' "$env_file" | cut -d= -f2- || true)"
[[ -n "$uri" ]] || { echo "respaldo: falta MONGO_URI en $env_file" >&2; exit 1; }

step=imagen
docker pull -q "$mongo_image" >/dev/null

# mongodump lee la cadena de --config (YAML); la comilla simple se escapa duplicándola. stderr se
# guarda y se imprime sin credenciales, por si el driver repite la cadena en un error.
step=mongodump
config="uri: '${uri//\'/\'\'}'"
unset uri
if ! printf '%s\n' "$config" | timeout "$dump_timeout" docker run -i --rm --name "$container" \
	--network "$network" --memory 256m --entrypoint sh "$mongo_image" \
	-c 'umask 077; cat > /tmp/mongodump.yaml; exec mongodump --config=/tmp/mongodump.yaml --archive --gzip --quiet' \
	>"$archive" 2>"$work/stderr"; then
	sed -E 's#://[^@/ ]*@#://***@#g' "$work/stderr" >&2
	exit 1
fi
unset config

step=verificar
size="$(stat -c %s "$archive")"
((size > 0)) || { echo "respaldo: el archivo salió vacío" >&2; exit 1; }
gzip -t "$archive" || { echo "respaldo: el archivo no es un gzip válido" >&2; exit 1; }

step=subir
if [[ -n "$dry_dir" ]]; then
	cp "$archive" "$dry_dir/$name"
	echo "respaldo: en seco, $name ($size bytes) en $dry_dir; no se subió"
else
	account="$(aws sts get-caller-identity --region "$region" --query Account --output text)"
	bucket="wherever-dev-respaldos-$account"
	# --expected-bucket-owner: si el bucket no es de esta cuenta, S3 rechaza la subida. `aws s3 cp`
	# no la acepta (AWS CLI 2.38 de Amazon Linux 2023): por eso `s3api put-object`, que sube hasta
	# 5 GB en una sola petición, de sobra para el piloto.
	aws s3api put-object --bucket "$bucket" --key "$prefix$name" --body "$archive" --region "$region" \
		--expected-bucket-owner "$account" >/dev/null
	echo "respaldo: subido s3://$bucket/$prefix$name ($size bytes)"
fi
rm -f "$archive"

step=registrar
log_event "{\"evento\":\"respaldo\",\"estado\":\"ok\",\"bytes\":$size,\"archivo\":\"$prefix$name\"}" ||
	{ echo "respaldo: el respaldo SÍ se subió, pero sin el evento la alarma de 7 días no lo verá" >&2; exit 1; }
