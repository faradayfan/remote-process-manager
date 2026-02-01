# Local Testing Guide

This guide covers how to run and test Remote Process Manager locally.

## Prerequisites

- Go **1.24+**
- mTLS certificates (see [mTLS Setup](#mtls-setup))
- A game server binary (Minecraft `server.jar` recommended for testing)

## Quick Start

### 1. Generate Certificates (first time only)

```bash
chmod +x scripts/gen-certs.sh

./scripts/gen-certs.sh all \
  --agent-id home-01 \
  --server-dns command-server \
  --server-dns localhost \
  --server-ip 127.0.0.1
```

### 2. Start the Services

**Terminal 1 - Command Server:**

```bash
go run ./cmd/command-server -config configs/command-server.yaml
```

On first run, an admin API key is generated and saved to `configs/api-keys.yaml`.

**Terminal 2 - Agent:**

```bash
go run ./cmd/agent
```

### 3. Get Your API Key

```bash
cat configs/api-keys.yaml
```

Export it for CLI usage:

```bash
export GAMESVC_API_KEY="rpm_sk_..."
export GAMESVC_URL="http://127.0.0.1:8080"
```

### 4. Test the Connection

```bash
go run ./cmd/ctl agents
```

## Running the Smoke Test

The repo includes a comprehensive smoke test script that exercises the entire system.

### What It Tests

- API authentication (401 for unauthorized, 200 for authorized)
- Agents and templates endpoints
- Instance lifecycle (create/start/stop/status/delete)
- Instance updates (enable/disable, params set/unset, rename)
- Validations (start fails if disabled, rename fails while running)
- Minecraft EULA handling

### Running the Test

```bash
# Set your API key
export GAMESVC_API_KEY="$(grep 'key:' configs/api-keys.yaml | head -1 | awk '{print $2}')"

# Run with absolute path to jar (recommended)
./scripts/test-local.sh --jar-path "$(pwd)/server-binaries/minecraft-1.21.11/server.jar"
```

### Script Options

```bash
./scripts/test-local.sh --help
```

| Option | Description | Default |
|--------|-------------|---------|
| `--agent-id <id>` | Agent ID to target | `home-01` |
| `--template <name>` | Template name | `minecraft-vanilla` |
| `--jar-path <path>` | Path to Minecraft server.jar | (required) |
| `--url <url>` | Command server URL | `http://127.0.0.1:8080` |
| `--api-key <key>` | API key (or use `GAMESVC_API_KEY`) | - |
| `--use-bin` | Use installed binary instead of `go run` | - |
| `--no-mc-eula` | Don't write eula.txt automatically | - |

### Important: Use Absolute Paths

The jar path must be absolute because the Minecraft server runs with its working directory set to the instance directory (`data/instances/<name>/`).

```bash
# Good - absolute path
./scripts/test-local.sh --jar-path /full/path/to/server.jar

# Bad - relative path (will fail to find jar)
./scripts/test-local.sh --jar-path server-binaries/minecraft-1.21.11/server.jar
```

## Manual Testing

If you prefer to step through the flow manually:

```bash
# Set environment
export GAMESVC_URL="http://127.0.0.1:8080"
export GAMESVC_API_KEY="rpm_sk_..."

# List agents
go run ./cmd/ctl agents

# List templates
go run ./cmd/ctl templates list home-01

# Create an instance
go run ./cmd/ctl instances create home-01 survival-1 minecraft-vanilla \
  mem_min=2G mem_max=4G jar_path=/full/path/to/server.jar

# Write EULA (required for Minecraft)
mkdir -p data/instances/survival-1
echo "eula=true" > data/instances/survival-1/eula.txt

# Start the instance
go run ./cmd/ctl instances start home-01 survival-1

# Check status
go run ./cmd/ctl instances status home-01 survival-1

# Stop the instance
go run ./cmd/ctl instances stop home-01 survival-1

# Clean up
go run ./cmd/ctl instances delete home-01 survival-1 --force --delete-data
```

## API Authentication Testing

### Test Unauthorized Access

```bash
# Without API key - should return 401
curl http://127.0.0.1:8080/v1/agents
```

### Test Authorized Access

```bash
# With API key - should return agent list
curl -H "Authorization: Bearer rpm_sk_..." http://127.0.0.1:8080/v1/agents
```

### Development Mode (No Auth)

For development, you can disable authentication:

```yaml
# configs/command-server.yaml
api:
  keys_file: "configs/api-keys.yaml"
  allow_unauthenticated: true  # WARNING: Development only!
```

The server will log a warning when authentication is disabled.

## Troubleshooting

### "Unable to access jarfile"

The jar path is relative but needs to be absolute. Use:

```bash
--jar-path "$(pwd)/server-binaries/minecraft-1.21.11/server.jar"
```

### "missing or invalid authorization header"

Set the API key:

```bash
export GAMESVC_API_KEY="$(grep 'key:' configs/api-keys.yaml | head -1 | awk '{print $2}')"
```

### Agent Not Connecting

1. Check certificates exist in `configs/certs/`
2. Verify `allowed_agents` in `configs/command-server.yaml` includes your agent ID
3. Check agent logs for TLS errors

### Instance Won't Start

1. Ensure the instance is enabled: `instances enable <agent> <instance>`
2. Check the log file: `cat logs/<instance>.log`
3. For Minecraft: ensure `eula.txt` exists with `eula=true`

## mTLS Setup

Generate certificates for local development:

```bash
# Generate CA + server + one agent cert
./scripts/gen-certs.sh all \
  --agent-id home-01 \
  --server-dns command-server \
  --server-dns localhost \
  --server-ip 127.0.0.1

# Add more agent certs
./scripts/gen-certs.sh gen-agent --agent-id home-02
```

Update `configs/command-server.yaml` to allow the agent:

```yaml
tls:
  allowed_agents:
    - "home-01"
    - "home-02"
```
