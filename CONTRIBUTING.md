# Contributing

Thanks for your interest in contributing! 🎉

Remote Process Manager is intentionally small and modular. If you want to add support for new game servers, transports, auth, or UX improvements (CLI/Discord/Web), contributions are welcome.

---

## Quick Start (dev)

### Requirements

- Go **1.24+**
- A Linux/macOS machine for running the agent in development
- A game server binary (optional) if you want to test real process start/stop (Minecraft is the easiest starting point)

### Run locally

Terminal 1 (command server):

```bash
go run ./cmd/command-server
```

Terminal 2 (agent):

```bash
go run ./cmd/agent
```

Terminal 3 (CLI):

```bash
go run ./cmd/ctl agents
```

> Tip: The CLI talks to the command-server HTTP API. You can point it at a non-default URL using:
>
> ```bash
> export GAMESVC_URL="http://127.0.0.1:8080"
> ```

---

## Development workflow

1. Fork the repo
2. Create a feature branch:
   ```bash
   git checkout -b feat/my-change
   ```
3. Make changes + add tests when possible
4. Run checks:
   ```bash
   go test ./...
   ```
5. Run the smoke test (recommended) – see below
6. Open a Pull Request

---

## Testing

### Unit tests

Run all unit tests:

```bash
go test ./...
```

### End-to-end smoke test (recommended)

The repo includes a local smoke test script that exercises the whole system:

- Agents + templates endpoints
- Instance lifecycle (create/start/stop/status/delete)
- Instance updates (enable/disable, params set/unset, rename)
- Validations (start fails if disabled, rename fails while running)
- Minecraft EULA handling (writes `eula=true` into the instance directory)
- Optional sleeps to allow the game server to actually start/stop cleanly

#### Script location

```
scripts/test-local.sh
```

#### Prerequisites

- The **command-server** is running
- The **agent** is running and connected
- You have a Minecraft `server.jar` available locally (or change to your own template)

#### Run the smoke test

```bash
chmod +x ./scripts/test-local.sh

./scripts/test-local.sh \
  --agent-id home-01 \
  --jar-path /opt/minecraft/server.jar
```

If you keep the jar inside this repo (recommended for dev), you can point at the included path:

```bash
./scripts/test-local.sh \
  --jar-path ./server-binaries/minecraft-1.21.11/server.jar
```

#### Useful flags

```bash
./scripts/test-local.sh --help
```

Common options:

- `--agent-id <id>`: agent target (default: `home-01`)
- `--template <name>`: template name (default: `minecraft-vanilla`)
- `--jar-path <path>`: path to the Minecraft `server.jar`
- `--url <url>`: command-server base URL (default: `http://127.0.0.1:8080`)
- `--use-bin`: use an installed `gamesvcctl` binary instead of `go run`
- `--no-mc-eula`: disable automatic `eula.txt` writing

> Note: The script assumes the default agent instance directory layout:
> `data/instances/<instance-name>/`.

---

## Manual testing (CLI)

If you want to manually step through the flow instead of using the script, here’s the canonical sequence:

```bash
go run ./cmd/ctl agents
go run ./cmd/ctl instances list home-01
go run ./cmd/ctl templates list home-01
go run ./cmd/ctl templates inspect home-01 minecraft-vanilla

go run ./cmd/ctl instances create home-01 survival-1 minecraft-vanilla \
  mem_min=2G mem_max=4G jar_path=/opt/minecraft/server.jar

go run ./cmd/ctl instances list home-01
go run ./cmd/ctl instances disable home-01 survival-1

go run ./cmd/ctl instances start home-01 survival-1
# confirm it doesn't start because it's disabled

go run ./cmd/ctl instances enable home-01 survival-1
go run ./cmd/ctl instances start home-01 survival-1

go run ./cmd/ctl instances rename home-01 survival-1 survival-world
# confirm it won't rename while running

go run ./cmd/ctl instances status home-01 survival-1
go run ./cmd/ctl instances stop home-01 survival-1

go run ./cmd/ctl instances rename home-01 survival-1 survival-world
go run ./cmd/ctl instances start home-01 survival-world
go run ./cmd/ctl instances stop home-01 survival-world

go run ./cmd/ctl instances delete home-01 survival-world --force --delete-data
```

### Minecraft EULA note

Minecraft requires `eula.txt` containing `eula=true` inside the instance directory before it will start successfully.

If you’re testing manually, create this file under:

```
data/instances/<instance-name>/eula.txt
```

Contents:

```txt
eula=true
```

The smoke test script handles this automatically.

---

## Project conventions

### Code layout

- `internal/manager`: process lifecycle and state tracking (**keep networking out of here**)
- `internal/instances`: templates + instance storage/rendering + validations
- `internal/control`: agent-side request handler that calls instance/manager services
- `internal/server`: command-server relay and HTTP API
- `internal/protocol`: message types shared by agent/server/CLI

### Logging

Prefer consistent log prefixes:

- `[agent] ...`
- `[command-server] ...`

### Configuration files

- `configs/instance-templates.yaml` is **manually edited** and should remain human-friendly.
- `configs/instances.yaml` is **agent-managed state** and should be updated via instance CRUD/update APIs when possible.

---

## Adding a new server type

In most cases, adding a new game server requires **no code changes**:

1. Add a template to `configs/instance-templates.yaml`
2. Create an instance (via CLI):
   ```bash
   go run ./cmd/ctl instances create <agentID> <name> <template> key=value ...
   ```

If a server needs special behavior (signals, working directory, environment, savedir), prefer extending templates before adding code.

---

## Reporting issues

Please include:

- OS (Linux/macOS/Windows)
- Go version
- agent logs
- command server logs
- template + instance snippets (remove secrets)

---

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
