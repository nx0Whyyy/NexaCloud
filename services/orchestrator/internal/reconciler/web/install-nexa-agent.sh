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
apt-get install -y --no-install-recommends ca-certificates curl docker.io
systemctl enable --now docker

case "$(uname -m)" in
  x86_64|amd64) AGENT_ARCH=amd64 ;;
  *) echo "Unsupported architecture: $(uname -m)" >&2; exit 3 ;;
esac
curl -fsSL "https://cloud.nexastudio.dev/downloads/nexa-agent-linux-$AGENT_ARCH" -o /usr/local/bin/nexa-agent
echo "8721472b2faf248405ca83dcd4e0955884328f51e8e289d010eb86942d33b7f2  /usr/local/bin/nexa-agent" | sha256sum -c -
chmod 0755 /usr/local/bin/nexa-agent
install -d -m 0700 /var/lib/nexacloud

printf '\nNexaAgent est installe. Generation du code d activation...\n\n'
if ! NEXA_NODE_NAME="$NODE_NAME" NEXA_STATE_PATH=/var/lib/nexacloud/agent.json nexa-agent register; then
  printf '\nActivation interrompue. Relancez cette commande pour generer un nouveau code.\n' >&2
  exit 4
fi

printf '\nActivation confirmee. Demarrage du service NexaAgent...\n'
cat >/etc/systemd/system/nexa-agent.service <<EOF
[Unit]
Description=NexaCloud node agent
After=docker.service network-online.target
Requires=docker.service

[Service]
Type=simple
Environment=NEXA_NODE_NAME=$NODE_NAME
Environment=NEXA_STATE_PATH=/var/lib/nexacloud/agent.json
ExecStart=/usr/local/bin/nexa-agent run
Restart=always
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now nexa-agent

systemctl --no-pager --full status nexa-agent || true
echo "NexaAgent installe, active et demarre."
