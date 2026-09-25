#!/bin/sh
set -eu

NODE_NAME="${1:-$(hostname)}"
case "$NODE_NAME" in *[!A-Za-z0-9._-]*|'') echo "Invalid node name" >&2; exit 2;; esac

if [ "$(id -u)" -ne 0 ]; then
  echo "Run this installer as root." >&2
  exit 1
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends ca-certificates git docker.io
systemctl enable --now docker

if [ -d /opt/nexacloud-agent-src/.git ]; then
  git -C /opt/nexacloud-agent-src fetch --depth 1 origin main
  git -C /opt/nexacloud-agent-src reset --hard origin/main
else
  git clone --depth 1 https://github.com/nx0Whyyy/NexaCloud.git /opt/nexacloud-agent-src
fi

docker build -f /opt/nexacloud-agent-src/docker/Dockerfile.agent -t nexacloud-agent:latest /opt/nexacloud-agent-src
docker volume create nexacloud-agent-state >/dev/null
docker run --rm --network host \
  -e NEXA_NODE_NAME="$NODE_NAME" -e NEXA_STATE_PATH=/state/agent.json \
  -v nexacloud-agent-state:/state nexacloud-agent:latest register
docker rm -f nexacloud-agent >/dev/null 2>&1 || true
docker run -d --name nexacloud-agent --restart unless-stopped --network host \
  -e NEXA_NODE_NAME="$NODE_NAME" -e NEXA_STATE_PATH=/state/agent.json \
  -v nexacloud-agent-state:/state -v /var/run/docker.sock:/var/run/docker.sock \
  nexacloud-agent:latest run

echo "NexaAgent installed and started."
