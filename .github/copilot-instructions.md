# Copilot Instructions — Remote Process Manager

These instructions describe the structure, conventions, and intent of this repository so an AI agent (Copilot Chat / VS Code agent mode) can make correct changes with minimal back-and-forth.

---

## Project summary

This repo provides a Go-based **control plane + agent** system for managing **game server processes** on remote machines.

- **Agent** runs on a host machine that can start/stop local server processes.
- **Command Server** runs somewhere reachable (cloud/VPS) and maintains outbound connections from agents.
- **CLI** talks to the command server HTTP API to manage agents/instances.

Primary goals:

- Avoid port-forwarding into home networks: agents connect **outbound** to the control plane.
- Support **server templates** and multiple **instances** of those templates.
- Provide start/stop/status + instance CRUD via a control plane.

---

## Go module + version

- Module: `github.com/faradayfan/remote-process-manager`
- Go version: **1.24+**

Prefer standard library and keep dependencies minimal.

---

## Repository layout

### `cmd/`
Entry points (binaries):

- `cmd/agent/` — agent process that manages servers on a machine
- `cmd/command-server/` — control plane + agent relay (TCP listener + HTTP API)
- `cmd/ctl/` — CLI client that calls the control plane HTTP API

When adding new executables, keep them under `cmd/<name>/main.go`.

### `configs/`
Configuration and templates:

- `configs/agent.yaml` — agent identity + command server address
- `configs/server-templates.yaml` — **manually edited templates** describing how to start/stop servers
- `configs/instances.yaml` — **agent-managed state** for server instances (often gitignored)

Templates should remain **human-friendly** and stable over time.

### `internal/`
Core libraries (not imported by external projects):

- `internal/manager/` — process lifecycle management (start/stop/status/log)
  - Must stay free of network/control-plane concerns.
- `internal/instances/` — instance CRUD + template rendering + persistence
- `internal/protocol/` — command/request/response types shared by agent/server/cli
- `internal/server/` — command-server registry, agent session management, HTTP handlers
- `internal/control/` — agent-side command handlers that call manager/instances
- `internal/config/` — YAML loading/parsing helpers
- `internal/transport/` — agent<->server transport utilities (TCP framing today)

**Rule of thumb:** keep boundaries clean:
- `manager` should not know about the control plane.
- the control plane should not embed server-specific logic; it should relay commands to the agent.

---

## Core concepts and definitions

### Server Templates (`configs/server-templates.yaml`)
Templates describe how to run a type of server (Minecraft, Valheim, etc.):

- `command` (string)
- `args` (list of strings)
- `cwd` (string)
- `env` (list of `KEY=value`)
- `stop` strategy:
  - `stdin` (send stop command text to stdin, e.g. `stop\n`)
  - `signal` (send a POSIX signal, e.g. `SIGTERM`)

Template strings may use Go `text/template` placeholders like:
- `{{.mem_min}}`, `{{.mem_max}}`, `{{.jar_path}}`
- Built-ins: `{{.instance_dir}}`, `{{.instance_name}}`, `{{.log_path}}`

### Instances (`configs/instances.yaml`)
Instances bind a name to a template + params:

- `template`: template name
- `enabled`: if false, instance should not start
- `params`: key/value used when rendering the template

Instances can be created/deleted via control plane commands.

---

## Agent ↔ Command Server communication

The agent establishes an outbound, long-lived connection to the command server, registers itself, and listens for commands.

The command server maintains a registry: `agentID -> active session`.
The HTTP API sends commands to the appropriate agent session.

When modifying this area:
- Preserve backward compatibility where reasonable.
- Prefer adding new fields rather than changing semantics of existing ones.
- Log errors with enough context (agent_id, instance name, command id).

---

## Coding style expectations

### Go style
- Prefer clear, explicit code over cleverness.
- Return errors with context using `fmt.Errorf("...: %w", err)`.
- Keep structs small and cohesive.
- Avoid global state (except process-wide loggers or registries owned by main).

### Context + cancellation
Where applicable:
- Use `context.Context` for server shutdown and long-running operations.
- HTTP handlers should respect request context.

### Logging
Use consistent prefixes so operators can grep logs easily:
- `[agent] ...`
- `[command-server] ...`
- `[ctl] ...`

### Testing
- Prefer table-driven tests for core logic.
- Focus on pure logic layers (`instances`, `protocol`, `manager` helpers).
- Avoid heavy integration tests unless clearly necessary.

---

## Build & run commands (common tasks)

### Run locally (dev)
Terminal 1:
```bash
go run ./cmd/command-server
```

Terminal 2:
```bash
go run ./cmd/agent
```

Terminal 3:
```bash
go run ./cmd/ctl agents
```

### Run tests
```bash
go test ./...
```

---

## Release strategy

This repo uses **Release Please**.

- Conventional Commits drive release notes and semantic version bumps.
- Release Please opens a Release PR; merging it creates a tag and GitHub Release.

Commit examples:
- `feat(agent): add instance create/delete`
- `fix(command-server): handle reconnect`
- `feat(ctl): add status command`
- `docs(readme): update usage`

---

## Safe change guidelines (important)

When implementing changes, follow these rules:

1. **Do not break existing YAML formats** unless also providing migration logic and docs.
2. Keep `internal/manager` transport-agnostic (no HTTP/TCP/gRPC dependencies).
3. When adding new commands, define the request/response in `internal/protocol`.
4. Prefer **small PRs** with one theme each: instance CRUD, start/stop behavior, logging, etc.
5. Avoid adding new dependencies unless they provide clear value.
6. If a change affects runtime behavior, update `README.md` usage docs.

---

## Common pitfalls

- **Port collisions** (Valheim uses ranges; Minecraft uses one port by default). Add guardrails if/when auto-port allocation is introduced.
- **Instance persistence**: `configs/instances.yaml` is machine-local state; do not assume it is committed to git.
- **Stop behavior**: Some servers need stdin stop, others need SIGTERM; do not hardcode stop strategies in Go.
- **Cross-platform builds**: Command paths and signals differ on Windows; keep Windows support best-effort unless explicitly required.

---

## What to do when asked to implement something

When the user requests changes:

1. Identify which layer is affected:
   - CLI vs command-server vs agent vs manager vs config
2. Propose a small, safe plan
3. Implement minimal viable changes first
4. Update docs/tests if relevant
5. Prefer backward-compatible changes over breaking ones
