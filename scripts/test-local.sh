#!/usr/bin/env bash
set -euo pipefail

#
# scripts/test-local.sh
#
# End-to-end smoke test for Remote Process Manager.
#
# Assumptions:
#   - command-server is already running (HTTP on :8080 by default)
#   - agent is already running and connected (agentID defaults to home-01)
#   - you run this script from anywhere (it auto-cd's to repo root)
#
# This script exercises:
#   - agents list
#   - instances list/create/delete
#   - instance enable/disable
#   - instance start/stop/status
#   - instance rename (including "should fail when running")
#   - instance params set/unset
#   - templates list/inspect
#
# Minecraft EULA:
#   - If you’re using the Minecraft template, the server needs eula.txt with eula=true.
#   - This script will write eula.txt into the instance directory before starting.
#

# ----------------------------
# Defaults
# ----------------------------
AGENT_ID="home-01"
TEMPLATE="minecraft-vanilla"
URL="${GAMESVC_URL:-http://127.0.0.1:8080}"
API_KEY="${GAMESVC_API_KEY:-}"

MEM_MIN="2G"
MEM_MAX="4G"

INSTANCE_A="survival-1"
INSTANCE_B="survival-world"

# If your agent uses the default base dir, instances live here
INSTANCE_BASE_DIR="data/instances"

# Default jar path: try to auto-detect inside repo
DEFAULT_JAR="server-binaries/minecraft-1.21.11/server.jar"

ACCEPT_MINECRAFT_EULA=1

# By default, we run the CLI via "go run"
# IMPORTANT: `go run` needs `--` before CLI args.
CTL_MODE="go-run" # go-run | bin
CTL_BIN="gamesvcctl"

JAR_PATH=""

# CI mode: use lightweight ci-test template instead of Minecraft
CI_MODE=0
CI_SLEEP_DURATION="30"  # seconds for the sleep process to run

# ----------------------------
# locate repo root + cd
# ----------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

if [[ -f "${DEFAULT_JAR}" ]]; then
	JAR_PATH="${REPO_ROOT}/${DEFAULT_JAR}"
fi

# ----------------------------
# arg parsing
# ----------------------------
usage() {
	cat <<EOF
Usage:
  $0 [options]

Options:
  --ci                    CI mode: use lightweight ci-test template (no jar required)
  --agent-id <id>         Agent ID to target (default: ${AGENT_ID})
  --template <name>       Template name (default: ${TEMPLATE})
  --jar-path <path>       Path to Minecraft server.jar (default: auto-detect)
  --url <url>             Command server HTTP URL (default: ${URL})
  --api-key <key>         API key for authentication (or set GAMESVC_API_KEY)
  --ctl-bin <name>        Use installed CLI binary (default: ${CTL_BIN})
  --use-bin               Use installed CLI binary instead of go run
  --no-mc-eula            Do not write Minecraft eula.txt (default: off)

Examples:
  # CI mode (no Minecraft required)
  ./scripts/test-local.sh --ci

  # Minecraft mode (requires jar path)
  ./scripts/test-local.sh --jar-path /opt/minecraft/server.jar

  # with API key
  ./scripts/test-local.sh --jar-path /opt/minecraft/server.jar --api-key rpm_sk_...

  # use installed gamesvcctl
  ./scripts/test-local.sh --use-bin --ctl-bin gamesvcctl --jar-path /opt/minecraft/server.jar

EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--ci)
		CI_MODE=1
		TEMPLATE="ci-test"
		ACCEPT_MINECRAFT_EULA=0
		shift 1
		;;
	--agent-id)
		AGENT_ID="${2:?missing value}"
		shift 2
		;;
	--template)
		TEMPLATE="${2:?missing value}"
		shift 2
		;;
	--jar-path)
		JAR_PATH="${2:?missing value}"
		shift 2
		;;
	--url)
		URL="${2:?missing value}"
		shift 2
		;;
	--api-key)
		API_KEY="${2:?missing value}"
		shift 2
		;;
	--use-bin)
		CTL_MODE="bin"
		shift 1
		;;
	--ctl-bin)
		CTL_BIN="${2:?missing value}"
		shift 2
		;;
	--no-mc-eula)
		ACCEPT_MINECRAFT_EULA=0
		shift 1
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "Unknown arg: $1" >&2
		usage
		exit 2
		;;
	esac
done

# In CI mode, jar path is not required
if [[ "${CI_MODE}" -eq 0 ]] && [[ -z "${JAR_PATH}" ]]; then
	echo "ERROR: --jar-path is required (could not auto-detect server jar)." >&2
	echo "Hint: ./scripts/test-local.sh --jar-path /path/to/server.jar" >&2
	echo "      or use --ci for CI mode (no jar required)" >&2
	exit 2
fi

export GAMESVC_URL="${URL}"
export GAMESVC_API_KEY="${API_KEY}"

# ----------------------------
# CLI runner
# ----------------------------
ctl() {
	# Runs the CLI with the given arguments.
	if [[ "${CTL_MODE}" == "bin" ]]; then
		"${CTL_BIN}" "$@"
	else
		go run ./cmd/ctl "$@"
	fi
}

# ----------------------------
# helpers
# ----------------------------
GREEN="\033[0;32m"
RED="\033[0;31m"
YELLOW="\033[0;33m"
NC="\033[0m"

step() {
	echo -e "${YELLOW}==> $*${NC}"
}

run_ok() {
	echo "+ $*"
	"$@"
	echo
}

run_expect_fail() {
	echo "+ (expect fail) $*"
	if "$@"; then
		echo -e "${RED}UNEXPECTED SUCCESS${NC}: command should have failed"
		exit 1
	else
		echo -e "${GREEN}expected failure ok${NC}"
	fi
	echo
}

cleanup_instance() {
	local name="$1"
	step "cleanup (best effort): delete instance ${name} if it exists"
	set +e
	ctl instances delete "${AGENT_ID}" "${name}" --force --delete-data >/dev/null 2>&1
	set -e
}

ensure_minecraft_eula() {
	local instance_name="$1"

	if [[ "${ACCEPT_MINECRAFT_EULA}" -ne 1 ]]; then
		return
	fi

	# Only do this when the template looks like Minecraft
	if [[ "${TEMPLATE}" != minecraft* ]]; then
		return
	fi

	local inst_dir="${REPO_ROOT}/${INSTANCE_BASE_DIR}/${instance_name}"
	local eula_file="${inst_dir}/eula.txt"

	step "ensure Minecraft EULA accepted: ${eula_file}"
	mkdir -p "${inst_dir}"
	cat >"${eula_file}" <<EOF
# Automatically written by scripts/test-local.sh
# Minecraft requires you to accept the EULA to run a server.
# See: https://aka.ms/MinecraftEULA
eula=true
EOF
}

# ----------------------------
# preflight
# ----------------------------
echo "Remote Process Manager smoke test"
echo "  Repo root : ${REPO_ROOT}"
echo "  URL       : ${URL}"
echo "  API Key   : ${API_KEY:+(set)}"
echo "  Agent ID  : ${AGENT_ID}"
echo "  Template  : ${TEMPLATE}"
if [[ "${CI_MODE}" -eq 1 ]]; then
	echo "  CI Mode   : enabled (using sleep-based test process)"
else
	echo "  Jar Path  : ${JAR_PATH}"
fi
echo "  CLI mode  : ${CTL_MODE}"
echo

cleanup_instance "${INSTANCE_A}"
cleanup_instance "${INSTANCE_B}"

# ----------------------------
# smoke tests
# ----------------------------

# Test API authentication if key is configured
if [[ -n "${API_KEY}" ]]; then
	step "auth: verify unauthorized request fails"
	# Temporarily unset key to test auth rejection
	(
		unset GAMESVC_API_KEY
		run_expect_fail ctl agents
	)
	step "auth: verify authorized request succeeds"
fi

step "agents list"
run_ok ctl agents

step "templates list"
run_ok ctl templates list "${AGENT_ID}"

step "templates inspect"
run_ok ctl templates inspect "${AGENT_ID}" "${TEMPLATE}"

step "instances list (empty or existing)"
run_ok ctl instances list "${AGENT_ID}"

step "instances create ${INSTANCE_A}"
if [[ "${CI_MODE}" -eq 1 ]]; then
	run_ok ctl instances create "${AGENT_ID}" "${INSTANCE_A}" "${TEMPLATE}" \
		"duration=${CI_SLEEP_DURATION}"
else
	run_ok ctl instances create "${AGENT_ID}" "${INSTANCE_A}" "${TEMPLATE}" \
		"mem_min=${MEM_MIN}" "mem_max=${MEM_MAX}" "jar_path=${JAR_PATH}"
fi

step "instances list (should include ${INSTANCE_A})"
run_ok ctl instances list "${AGENT_ID}"

step "disable instance"
run_ok ctl instances disable "${AGENT_ID}" "${INSTANCE_A}"

step "start should fail when disabled"
run_expect_fail ctl instances start "${AGENT_ID}" "${INSTANCE_A}"

step "enable instance"
run_ok ctl instances enable "${AGENT_ID}" "${INSTANCE_A}"

# Minecraft needs eula.txt before starting
ensure_minecraft_eula "${INSTANCE_A}"

step "start instance"
run_ok ctl instances start "${AGENT_ID}" "${INSTANCE_A}"

# Give it a moment to start up (shorter in CI mode)
if [[ "${CI_MODE}" -eq 1 ]]; then
	sleep 2
else
	sleep 10
fi

step "rename should fail when running"
run_expect_fail ctl instances rename "${AGENT_ID}" "${INSTANCE_A}" "${INSTANCE_B}"

step "status (running)"
run_ok ctl instances status "${AGENT_ID}" "${INSTANCE_A}"

step "params set (apply next start)"
if [[ "${CI_MODE}" -eq 1 ]]; then
	run_ok ctl instances params set "${AGENT_ID}" "${INSTANCE_A}" "duration=60"
else
	run_ok ctl instances params set "${AGENT_ID}" "${INSTANCE_A}" "mem_max=6G"
fi

step "params unset (apply next start)"
if [[ "${CI_MODE}" -eq 1 ]]; then
	# In CI mode, unset duration to test missing required param
	run_ok ctl instances params unset "${AGENT_ID}" "${INSTANCE_A}" "duration"
else
	run_ok ctl instances params unset "${AGENT_ID}" "${INSTANCE_A}" "jar_path"
fi

step "status (still running; params changes won't apply until restart)"
run_ok ctl instances status "${AGENT_ID}" "${INSTANCE_A}"

step "stop instance"
run_ok ctl instances stop "${AGENT_ID}" "${INSTANCE_A}"

# Wait for process to fully stop
if [[ "${CI_MODE}" -eq 1 ]]; then
	sleep 2
else
	sleep 10
fi

step "rename instance (now stopped)"
run_ok ctl instances rename "${AGENT_ID}" "${INSTANCE_A}" "${INSTANCE_B}"

step "instances list (should include ${INSTANCE_B})"
run_ok ctl instances list "${AGENT_ID}"

# ensure EULA under the new instance dir too (Minecraft stores it per instance dir)
ensure_minecraft_eula "${INSTANCE_B}"

step "fail to start renamed instance with unset param"
run_expect_fail ctl instances start "${AGENT_ID}" "${INSTANCE_B}"

step "restore required param for renamed instance"
if [[ "${CI_MODE}" -eq 1 ]]; then
	run_ok ctl instances params set "${AGENT_ID}" "${INSTANCE_B}" "duration=${CI_SLEEP_DURATION}"
else
	run_ok ctl instances params set "${AGENT_ID}" "${INSTANCE_B}" "jar_path=${JAR_PATH}"
fi

step "start renamed instance"
run_ok ctl instances start "${AGENT_ID}" "${INSTANCE_B}"

# Wait for startup
if [[ "${CI_MODE}" -eq 1 ]]; then
	sleep 2
else
	sleep 10
fi

step "status (running renamed instance)"
run_ok ctl instances status "${AGENT_ID}" "${INSTANCE_B}"

step "stop renamed instance"
run_ok ctl instances stop "${AGENT_ID}" "${INSTANCE_B}"

# Wait for shutdown
if [[ "${CI_MODE}" -eq 1 ]]; then
	sleep 2
else
	sleep 10
fi

step "delete instance (force + delete-data)"
run_ok ctl instances delete "${AGENT_ID}" "${INSTANCE_B}" --force --delete-data

step "instances list (should be cleaned up)"
run_ok ctl instances list "${AGENT_ID}"

echo -e "${GREEN}All smoke tests completed successfully.${NC}"
