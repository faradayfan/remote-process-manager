# Remote Process Manager

Remote Process Manager is a Go-based **control plane + agent** system for managing game servers (and other long-running processes) on remote machines.

## Why this exists

- **No port forwarding** required to your home network
- Remote machines run an **agent** that connects outbound to a **command server**
- A **control plane (HTTP API)** routes commands to the correct agent
- Supports **instance templates** and **many instances per template**
- Supports **instance lifecycle + updates** (create/delete/start/stop/status/enable/disable/params/rename)
- Uses **mTLS** between agent and command server (URI SAN identity + allowlist)

---

## Architecture

### Components

- **Agent** (`cmd/agent`)
  - Runs on the machine that hosts game servers
  - Loads templates + instances
  - Connects outbound to the command server over TCP (**mTLS**)
  - Starts/stops processes locally via `internal/manager`

- **Command Server** (`cmd/command-server`)
  - Runs in the cloud or any reachable host
  - Accepts outbound agent connections (**mTLS**)
  - Maintains a registry of connected agents
  - Exposes an HTTP API that relays commands to agents

- **CLI** (`cmd/ctl`)
  - Talks to the command server HTTP API
  - Lists agents / instances / templates
  - Creates, deletes, and updates instances

### Data flow

1. Agent loads `instance-templates.yaml` and `instances.yaml`
2. Agent connects outbound to the command server TCP listener and registers its instances
3. CLI calls HTTP endpoints on the command server
4. Command server relays a command over the active agent connection
5. Agent executes the command and replies with the result

---

## Project Layout

```
cmd/
  agent/                # runs on game host machine
  command-server/       # control plane + agent relay
  ctl/                  # CLI client (Cobra-based)

configs/
  agent.yaml              # agent identity + command server address + mTLS client config
  command-server.yaml     # command server listener config + mTLS allowlist
  instance-templates.yaml # templates (manually edited)
  instances.yaml          # instance state (managed by control plane)

internal/
  config/               # yaml loaders
  control/              # agent command handlers
  instances/            # template -> instance rendering + persistence
  manager/              # process spawning/stopping/status
  protocol/             # shared command schemas
  server/               # command server registry + http api + agent listener
  transport/            # tcp json framing utilities
```

---

## Requirements

- Go **1.24+**
- A machine capable of running your desired server process (Java, Valheim binary, etc.)

---

## Install

### Option A: Download from GitHub Releases

Each release publishes binaries for:

- Linux / macOS
- amd64 / arm64

Artifacts include:

- `command-server_*`
- `agent_*`
- `ctl_*`

> Windows binaries are not published yet (planned).

### Option B: Build from source

```bash
go build ./...
```

---

## Configuration

### `configs/command-server.yaml`

```yaml
tcp_addr: "0.0.0.0:9090"
http_addr: "0.0.0.0:8080"

tls:
  ca_file: "configs/certs/ca.crt"
  cert_file: "configs/certs/server.crt"
  key_file: "configs/certs/server.key"

  # Allowlist of agents (derived from client cert URI SAN)
  allowed_agents:
    - "home-01"
```

### `configs/agent.yaml`

```yaml
agent_id: "home-01"
command_server_addr: "127.0.0.1:9090"

tls:
  ca_file: "configs/certs/ca.crt"
  cert_file: "configs/certs/agents/home-01.crt"
  key_file: "configs/certs/agents/home-01.key"

  # Optional but recommended (must match a server DNS SAN)
  server_name: "command-server"
```

### `configs/instance-templates.yaml`

Templates define how to run a class of server. They support Go `text/template` substitution using instance params.

Example (Minecraft Vanilla):

```yaml
templates:
  minecraft-vanilla:
    command: "java"
    args:
      - "-Xms{{.mem_min}}"
      - "-Xmx{{.mem_max}}"
      - "-jar"
      - "{{.jar_path}}"
      - "nogui"
    cwd: "{{.instance_dir}}"
    env: []
    stop:
      type: "stdin"
      command: "stop"
      grace_period: "15s"
```

Stop strategies:

- `stdin` (send a command like `stop`)
- `signal` (send POSIX signal like `SIGTERM`)

### `configs/instances.yaml`

Instances are stored state on the agent and are managed via CLI/control plane commands.

```yaml
instances:
  survival-1:
    template: "minecraft-vanilla"
    enabled: true
    params:
      mem_min: "2G"
      mem_max: "4G"
      jar_path: "/opt/minecraft/server.jar"
```

---

## mTLS Setup (Agent Trust)

The agent ↔ command-server TCP connection is protected with **mTLS**.

### Agent identity via URI SAN

Agent certificates must include a URI SAN like:

```
spiffe://remote-process-manager/agent/<agent-id>
```

Example:

```
spiffe://remote-process-manager/agent/home-01
```

The command server extracts `<agent-id>` and enforces the allowlist in `configs/command-server.yaml`.

### Generating certs

This repo includes a helper script:

```bash
scripts/gen-certs.sh
```

Example (local dev):

```bash
chmod +x scripts/gen-certs.sh

./scripts/gen-certs.sh all   --agent-id home-01   --server-dns command-server   --server-dns localhost   --server-ip 127.0.0.1
```

Generate more agents:

```bash
./scripts/gen-certs.sh gen-agent --agent-id garage-01
```

> You said you plan to gitignore the entire cert directory — that’s a good default. Do not commit private keys.

---

## Running Locally

See [docs/test-local.md](docs/test-local.md) for the complete local testing guide.

**Quick start:**

```bash
# Terminal 1: Command server
go run ./cmd/command-server -config configs/command-server.yaml

# Terminal 2: Agent
go run ./cmd/agent

# Terminal 3: CLI (get API key from configs/api-keys.yaml)
export GAMESVC_API_KEY="rpm_sk_..."
export GAMESVC_URL="http://127.0.0.1:8080"
go run ./cmd/ctl agents
```

---

## CLI Cheatsheet

Set URL:

```bash
export GAMESVC_URL="http://127.0.0.1:8080"
# or per-command:
gamesvcctl --url http://127.0.0.1:8080 agents
```

Agents:

```bash
gamesvcctl agents
```

Instances:

```bash
# list
gamesvcctl instances list <agentID>

# create (params are key=value)
gamesvcctl instances create <agentID> <name> <template> [key=value ...]

# delete
gamesvcctl instances delete <agentID> <name> [--force] [--delete-data]

# enable/disable
gamesvcctl instances enable  <agentID> <instance>
gamesvcctl instances disable <agentID> <instance>

# set/unset params (applies on next start)
gamesvcctl instances params set   <agentID> <instance> key=value [key=value ...]
gamesvcctl instances params unset <agentID> <instance> key [key ...]

# rename (must be stopped)
gamesvcctl instances rename <agentID> <oldName> <newName>

# start/stop/status
gamesvcctl instances start  <agentID> <instance>
gamesvcctl instances stop   <agentID> <instance>
gamesvcctl instances status <agentID> <instance>
```

Templates:

```bash
gamesvcctl templates list <agentID>
gamesvcctl templates inspect <agentID> <templateName>
```

---

## Releases

This repo uses:

- **Release Please** for versioning + changelog + GitHub Releases
- A build workflow that compiles and uploads binaries for each release

---

## Security Notes

- Agent ↔ command-server uses **mTLS** for identity + transport security.
- The HTTP API uses **API key authentication** with role-based access control.

---

## API Authentication

The command server HTTP API requires authentication via API keys.

Add an `api` section to `configs/command-server.yaml`:

```yaml
api:
  keys_file: "configs/api-keys.yaml"

  # Set to true to disable authentication (development only!)
  # allow_unauthenticated: true
```

On first startup, an admin API key is automatically generated and saved to the keys file. Retrieve it with:

```bash
cat configs/api-keys.yaml
```

### Using the API Key

Via environment variable (recommended):

```bash
export GAMESVC_API_KEY="rpm_sk_..."
gamesvcctl agents
```

Via CLI flag:

```bash
gamesvcctl --api-key "rpm_sk_..." agents
```

### Roles

- **admin** - Full access (create, delete, start, stop, enable, disable, rename, params)
- **read** - Read-only access (list agents, instances, templates, status)

### API Keys File Format

```yaml
keys:
  - name: admin
    key: rpm_sk_...
    roles:
      - admin
    created_at: 2026-01-24T00:00:00Z
```

---

## License

MIT
