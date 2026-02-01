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

See [docs/test-local.md](docs/test-local.md) for the complete local testing guide.

**Quick start:**

```bash
# Set API key (generated on first server start)
export GAMESVC_API_KEY="$(grep 'key:' configs/api-keys.yaml | head -1 | awk '{print $2}')"

# Run smoke test (use absolute path for jar)
./scripts/test-local.sh --jar-path "$(pwd)/server-binaries/minecraft-1.21.11/server.jar"
```

The smoke test exercises:

- API authentication (401/200)
- Agents + templates endpoints
- Instance lifecycle (create/start/stop/status/delete)
- Instance updates (enable/disable, params set/unset, rename)
- Validations (start fails if disabled, rename fails while running)

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
