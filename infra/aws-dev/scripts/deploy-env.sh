#!/usr/bin/env bash
# Corre EN la instancia: arma /opt/wherever/.env desde Parameter Store (spec nv-07).
# Lee todos los parámetros de la ruta (SecureString descifrados) sin mostrar sus valores.
# GHCR_TOKEN no va al .env: es del actualizador (nv-08) y la API no debe verlo.
#   uso: deploy-env.sh /wherever/dev/
set -euo pipefail
umask 077

path="${1:?uso: deploy-env.sh /<proyecto>/<entorno>/}"
region="$(cut -d= -f2 /opt/wherever/region)"
out=/opt/wherever/.env
tmp="$(mktemp /opt/wherever/.env.XXXXXX)"
trap 'rm -f "$tmp"' EXIT

aws ssm get-parameters-by-path --region "$region" --path "$path" --recursive --with-decryption \
  --query 'Parameters[].[Name,Value]' --output text |
  while IFS=$'\t' read -r name value; do
    [ "${name##*/}" = "GHCR_TOKEN" ] && continue
    printf '%s=%s\n' "${name##*/}" "$value"
  done > "$tmp"

# Región y flujo del log de esta instancia (nv-13): compose.yml los pide para el controlador awslogs.
imds_token="$(curl -s -X PUT -H 'X-aws-ec2-metadata-token-ttl-seconds: 60' http://169.254.169.254/latest/api/token)"
instance_id="$(curl -s -H "X-aws-ec2-metadata-token: $imds_token" http://169.254.169.254/latest/meta-data/instance-id)"
[ -n "$instance_id" ] || { echo "deploy-env: no se pudo leer el id de la instancia" >&2; exit 1; }
printf 'LOG_REGION=%s\nLOG_STREAM=api-%s\n' "$region" "$instance_id" >> "$tmp"
for key in APP_ENV MONGO_URI JWT_KEY JWT_ISSUER JWT_AUDIENCE PHONE_HMAC_KEY CURSOR_HMAC_KEY; do
  grep -q "^${key}=" "$tmp" || { echo "deploy-env: falta ${key} en ${path}" >&2; exit 1; }
done
mv "$tmp" "$out"
trap - EXIT
echo "deploy-env: $(wc -l < "$out") variables en $out (valores no mostrados)"
