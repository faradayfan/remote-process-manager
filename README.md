# Remote Process Manager

Remote Process Manager is a Go-based control plane + agent system for managing game servers (and other long-running processes) on remote machines.

The key goals are:

- **No port forwarding required** to your home network
- Remote machines run an **agent** that connects outbound to a **command server**
- A **control plane** (HTTP API) routes user/automation commands to the correct agent
- Supports **server templates** + **multiple instances** per template
- Supports start/stop/status/log collection for managed processes

This project is designed to grow into integrations such as Slack/Discord/Web UI while keeping the core process management reliable and testable.

---

## Architecture Overview

### Components

- **Agent** (`cmd/agent`)

  - Runs on the machine that hosts game servers
  - Reads configuration templates and instances
  - Connects outbound to the command server over TCP
  - Registers itself and listens for commands
  - Starts/stops processes locally via `internal/manager`

- **Command Server** (`cmd/command-server`)

  - Runs in the cloud or any reachable host
  - Accepts outbound agent TCP connections
  - Maintains a registry of connected agents
  - Exposes an HTTP API that relays commands to agents

- **CLI** (`cmd/ctl`)
  - Talks to the command server HTTP API
  - Lists agents and instances
  - Creates/deletes instances
  - Starts/stops instances

### Data Flow

1. Agent boots, loads `server-templates.yaml` + `instances.yaml`
2. Agent connects outbound to command-server TCP listener and registers:
   - `agent_id`
   - instance list (currently called “servers” in protocol)
3. CLI calls command-server HTTP endpoints
4. command-server relays commands over the active agent TCP connection
5. agent executes commands and replies with results

---

## Project Layout

```
cmd/
  agent/                # runs on game host machine
  command-server/       # control plane + agent relay
  ctl/                  # CLI client

configs/
  agent.yaml            # agent identity + command server address
  server-templates.yaml # templates (manually edited)
  instances.yaml        # instance state (managed by control plane)

internal/
  config/               # yaml loaders
  control/              # agent command handlers
  instances/            # template -> instance rendering + persistence
  manager/              # process spawning/stopping/status
  protocol/             # shared command schemas
  server/               # command server registry + http api
  transport/            # tcp json framing utilities
```

---

## Requirements

- Go **1.24+**
- A machine capable of running your desired game server process (Java, Valheim binary, etc.)

---

## Configuration

### 1) `configs/agent.yaml`

Defines the agent identity and where to connect:

```yaml
agent_id: "home-01"
command_server_addr: "127.0.0.1:9090"
```

- `agent_id`: Stable identifier for the agent
- `command_server_addr`: TCP address of the command-server agent listener

---

### 2) `configs/server-templates.yaml`

Templates are **manually edited** and define how to run a class of server.

Templates can reference instance parameters using Go `text/template` syntax:

- `{{.mem_min}}`
- `{{.mem_max}}`
- `{{.jar_path}}`
- `{{.instance_dir}}` (built-in)
- `{{.instance_name}}` (built-in)
- `{{.log_path}}` (built-in)

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
      command: "stop\n"
      grace_period: "15s"
```

Stop strategies:

- **stdin**
  - Sends a command over process stdin (defaults to `stop\n`)
- **signal**
  - Sends a POSIX signal (e.g. `SIGTERM`)

Example signal stop:

```yaml
stop:
  type: "signal"
  signal: "SIGTERM"
  grace_period: "15s"
```

---

### 3) `configs/instances.yaml`

Instances are **stored state** on the agent and are managed via control plane commands.

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

Fields:

- `template`: which template to use
- `enabled`: if false, starting the instance will return an error
- `params`: key/value parameters referenced by the template

---

## Running Locally (Development)

### 1) Start the command server

Terminal 1:

```bash
go run ./cmd/command-server
```

By default it starts:

- Agent TCP listener: `0.0.0.0:9090`
- HTTP API for CLI: `0.0.0.0:8080`

---

### 2) Start the agent

Terminal 2:

```bash
go run ./cmd/agent
```

The agent will connect to the command server and register its instances.

---

### 3) Run CLI commands

Terminal 3:

```bash
go run ./cmd/ctl agents
```

---

## CLI Usage

The CLI talks to the command-server HTTP API.

Set the command server URL:

```bash
export GAMESVC_URL="http://127.0.0.1:8080"
```

### List connected agents

```bash
gamesvcctl agents
```

Example:

```bash
go run ./cmd/ctl agents
```

---

### List instances on an agent

```bash
gamesvcctl instances <agentID>
```

Example:

```bash
go run ./cmd/ctl instances home-01
```

---

### Create an instance on an agent

```bash
gamesvcctl instance-create <agentID> <name> <template> [key=value ...]
```

Example:

```bash
go run ./cmd/ctl instance-create home-01 survival-2 minecraft-vanilla \
  mem_min=2G mem_max=4G jar_path=/opt/minecraft/server.jar
```

This will:

- add the instance into `configs/instances.yaml` on the agent
- create instance directories on disk (best effort)

---

### Delete an instance

```bash
gamesvcctl instance-delete <agentID> <name> [--force] [--delete-data]
```

Options:

- `--force`: stop the instance first if running
- `--delete-data`: remove the instance directory from disk

Example:

```bash
go run ./cmd/ctl instance-delete home-01 survival-2 --force --delete-data
```

---

### Start an instance

```bash
gamesvcctl start <agentID> <instance>
```

Example:

```bash
go run ./cmd/ctl start home-01 survival-1
```

---

### Stop an instance

```bash
gamesvcctl stop <agentID> <instance>
```

Example:

```bash
go run ./cmd/ctl stop home-01 survival-1
```

---

### Get status

```bash
gamesvcctl status <agentID> <instance>
```

Example:

```bash
go run ./cmd/ctl status home-01 survival-1
```

---

## Instance Directories & Logs

By default:

- Instance working directory:

  - `data/instances/<instance-name>/`

- Logs are written to:
  - `logs/<instance-name>.log`

Many servers can be configured to store their world/data under the working directory.
For others, you may need to pass explicit arguments or environment variables.

---

## Notes on Security (Current State)

The current implementation focuses on functionality and development velocity.

**Important:** This v1 design does not yet include:

- authentication / authorization
- TLS/mTLS
- rate limiting
- audit logging

For real deployment, the command server should be hardened with:

- TLS + **mTLS** between agent and command server
- identity controls (agent allowlist)
- authenticated CLI access (JWT, API tokens)
- strict action allowlisting

---

## Roadmap Ideas

- Add `templates list` and `templates inspect` endpoints
- Add instance update:
  - enable/disable
  - update params
  - rename
- Automatic port allocation
- Log tailing via control plane
- AuthN/AuthZ + TLS/mTLS
- Discord / Slack / Web UI integrations

---

## License

TBD
