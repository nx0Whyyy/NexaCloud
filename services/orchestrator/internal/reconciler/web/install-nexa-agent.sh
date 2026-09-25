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
echo "10a17dce4143df520ddb3df262a3e966a32d0dc8b5634de7ce2757ad49c18f22  /usr/local/bin/nexa-agent" | sha256sum -c -
chmod 0755 /usr/local/bin/nexa-agent
install -d -m 0700 /var/lib/nexacloud

NEXA_NODE_NAME="$NODE_NAME" NEXA_STATE_PATH=/var/lib/nexacloud/agent.json nexa-agent register
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

echo "NexaAgent installed and started."
