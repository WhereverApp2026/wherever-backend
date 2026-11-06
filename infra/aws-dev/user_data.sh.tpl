#!/bin/bash
# Arranque de la instancia de desarrollo (spec nv-07). Solo prepara el sistema: Docker,
# Compose, swap y rotación de logs. La app la sube scripts/instalar-actualizador.sh y los secretos los
# arma scripts/deploy-env.sh desde Parameter Store (${param_path}). Sin secretos aquí.
set -euo pipefail
exec > >(tee -a /var/log/wherever-bootstrap.log) 2>&1
echo "[$(date -u +%FT%TZ)] inicio del arranque"

dnf -y update --security
dnf -y install docker

# Rotación de logs de los contenedores para no llenar el disco.
mkdir -p /etc/docker
cat > /etc/docker/daemon.json <<'JSON'
{ "log-driver": "json-file", "log-opts": { "max-size": "10m", "max-file": "3" } }
JSON
systemctl enable --now docker
usermod -aG docker ec2-user

# Docker Compose v2 como plugin, versión fijada y con su suma de verificación.
COMPOSE_VERSION="v2.39.4"
ARCH="$(uname -m)"
DEST=/usr/local/lib/docker/cli-plugins
mkdir -p "$DEST"
BASE="https://github.com/docker/compose/releases/download/$${COMPOSE_VERSION}/docker-compose-linux-$${ARCH}"
curl -fsSL "$BASE" -o "$DEST/docker-compose"
curl -fsSL "$BASE.sha256" -o /tmp/docker-compose.sha256
(cd "$DEST" && sed "s#\S*docker-compose-linux-$${ARCH}#docker-compose#" /tmp/docker-compose.sha256 | sha256sum -c -)
chmod +x "$DEST/docker-compose"

# Swap: API, Redis y Neo4j caben en 1 GB con límites y este respaldo.
if [ ! -f /swapfile ]; then
  dd if=/dev/zero of=/swapfile bs=1M count=$(( ${swap_gb} * 1024 ))
  chmod 600 /swapfile
  mkswap /swapfile
  echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi
swapon -a

mkdir -p /opt/wherever
chown ec2-user:ec2-user /opt/wherever
echo "AWS_REGION=${region}" > /opt/wherever/region
echo "[$(date -u +%FT%TZ)] arranque terminado"
