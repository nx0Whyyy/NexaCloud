# NexaAgent

NexaAgent relie un hôte Docker au control plane NexaCloud uniquement via HTTPS. PostgreSQL et NATS restent privés.

## Installation et enrôlement

```bash
export NEXA_CONTROL_PLANE_URL=https://cloud.nexastudio.dev
export NEXA_NODE_NAME=node-paris-01
nexa-agent register
```

Saisir le code `NXA-XXX-XXX` affiché dans **Dashboard > Infrastructure > Activer un node**, puis démarrer l'agent :

```bash
nexa-agent check
nexa-agent run
```

L'état est stocké dans `~/.nexacloud/agent.json` avec des permissions `0600`. Le node passe hors ligne après 45 secondes sans heartbeat.

## Serveur Minecraft de test

Docker doit être installé et accessible à l'utilisateur du service.

```bash
nexa-agent minecraft create survival 25565 4G
export NEXA_MINECRAFT_ADDRESS=127.0.0.1:25565
nexa-agent run
```

Commandes disponibles : `start`, `stop` et `remove`. Les données Minecraft sont conservées dans le volume Docker `nexacloud-survival-data`.

Pour la production, exécuter NexaAgent avec systemd, un utilisateur dédié membre du groupe Docker et un fichier d'environnement lisible uniquement par root.
