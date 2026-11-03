#!/usr/bin/env bash
# Corre EN la instancia: arma /opt/wherever/.env desde Parameter Store (spec nv-07).
# Lee todos los parámetros de la ruta (SecureString descifrados) sin mostrar sus valores.
#   uso: deploy-env.sh /wherever/dev/ [sitio]
set -euo pipefail
umask 077

path="${1:?uso: deploy-env.sh /<proyecto>/<entorno>/ [sitio]}"
site="${2:-}"
region="$(cut -d= -f2 /opt/wherever/region)"
out=/opt/wherever/.env
tmp="$(mktemp /opt/wherever/.env.XXXXXX)"
trap 'rm -f "$tmp"' EXIT

aws ssm get-parameters-by-path --region "$region" --path "$path" --recursive --with-decryption \
  --query 'Parameters[].[Name,Value]' --output text |
  while IFS=$'\t' read -r name value; do
    printf '%s=%s\n' "${name##*/}" "$value"
  done > "$tmp"

if [ -n "$site" ]; then
  printf 'SITE_ADDRESS=%s\n' "$site" >> "$tmp"
fi
for key in APP_ENV MONGO_URI JWT_KEY JWT_ISSUER JWT_AUDIENCE PHONE_HMAC_KEY; do
  grep -q "^${key}=" "$tmp" || { echo "deploy-env: falta ${key} en ${path}" >&2; exit 1; }
done
mv "$tmp" "$out"
trap - EXIT
echo "deploy-env: $(wc -l < "$out") variables en $out (valores no mostrados)"
