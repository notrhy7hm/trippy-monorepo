#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root, or with sudo." >&2
  exit 1
fi

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y \
  ca-certificates \
  curl \
  docker.io \
  docker-compose-v2 \
  golang-go \
  nodejs \
  npm \
  postgresql-client

systemctl enable --now docker

go version
node --version
npm --version
docker --version
docker compose version
psql --version
