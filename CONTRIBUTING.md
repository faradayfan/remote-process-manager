# Contributing

Thanks for your interest in contributing! 🎉

This project is intentionally small and modular. If you want to add support for new game servers, transports, auth, or UX improvements (CLI/Discord/Web), contributions are welcome.

## Quick Start (dev)

### Requirements

- Go 1.24+
- A Linux/macOS machine for running the agent in development
- A game server binary (optional) if you want to test actual process start/stop

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
5. Open a Pull Request

### Manual Testing

Currently, there is no automated end-to-end test suite. You can manually test changes using the CLI commands below:

```bash
go run ./cmd/ctl agents
go run ./cmd/ctl instances list home-01
go run ./cmd/ctl templates list home-01
go run ./cmd/ctl templates inspect home-01 minecraft-vanilla
go run ./cmd/ctl instances create home-01 survival-1 minecraft-vanilla mem_min=2G mem_max=4G jar_path=/opt/minecraft/server.jar
go run ./cmd/ctl instances list home-01
go run ./cmd/ctl instances disable home-01 survival-1
go run ./cmd/ctl instances start home-01 survival-1 # confirm it doesn't start because it's disabled
go run ./cmd/ctl instances enable home-01 survival-1
go run ./cmd/ctl instances start home-01 survival-1
go run ./cmd/ctl instances rename home-01 survival-1 survival-world # confirm it wont rename while running
go run ./cmd/ctl instances status home-01 survival-1
go run ./cmd/ctl instances stop home-01 survival-1
go run ./cmd/ctl instances rename home-01 survival-1 survival-world
go run ./cmd/ctl instances start home-01 survival-world
go run ./cmd/ctl instances stop home-01 survival-world
go run ./cmd/ctl instances delete home-01 survival-world --force --delete-data
```

## Project conventions

### Code layout

- `internal/manager`: process lifecycle and state tracking (keep networking out of here)
- `internal/instances`: templates + instance storage/rendering
- `internal/server`: command-server relay and HTTP API
- `internal/protocol`: message types shared by agent/server/CLI

### Logging

Prefer consistent log prefixes:

- `[agent] ...`
- `[command-server] ...`

### Configuration files

- `configs/instance-templates.yaml` is **manually edited** and should remain human-friendly.
- `configs/instances.yaml` is **agent-managed state** and should be updated using the instance CRUD APIs when possible.

## Adding a new server type

In most cases, adding a new game server requires **no code changes**:

1. Add a template to `configs/instance-templates.yaml`
2. Create an instance (via CLI):
   ```bash
   go run ./cmd/ctl instance-create <agentID> <name> <template> key=value ...
   ```

If a server needs special behavior (signals, working directory, environment, savedir), prefer extending templates before adding code.

## Reporting issues

Please include:

- OS (Linux/macOS/Windows)
- Go version
- agent logs
- command server logs
- template + instance snippets (remove secrets)

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
