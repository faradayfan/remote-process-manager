#!/usr/bin/env bash
set -euo pipefail

#
# gen-certs.sh - Generate CA, server, and agent certificates for mTLS.
#
# Supports URI SAN for agents:
#   spiffe://remote-process-manager/agent/<agent-id>
#
# Works with macOS (LibreSSL) and Linux OpenSSL.
#

# ----------------------------
# Defaults
# ----------------------------
OUT_DIR="configs/certs"
AGENTS_DIR="configs/certs/agents"

COUNTRY="US"
STATE="UT"
LOCALITY="Home"
ORG="Remote Process Manager"

CA_CN="rpm-ca"
SERVER_CN="command-server"
SERVER_OU="Command Server"
AGENT_OU="Agent"

VALID_DAYS_CA=3650
VALID_DAYS_LEAF=825

SPIFFE_PREFIX="spiffe://remote-process-manager/agent/"

FORCE=0

# SAN arrays (server)
SERVER_DNS=()
SERVER_IPS=()

# ----------------------------
# Helpers
# ----------------------------
usage() {
	cat <<EOF
Usage:
  $0 <command> [flags]

Commands:
  init-ca           Generate CA key+cert (one-time)
  gen-server        Generate command-server key+csr+cert (signed by CA)
  gen-agent         Generate agent key+csr+cert (signed by CA)
  all               Run init-ca + gen-server + gen-agent (requires --agent-id)

Common Flags:
  --out-dir <dir>       Output cert directory (default: ${OUT_DIR})
  --country <C>         Subject C  (default: ${COUNTRY})
  --state <ST>          Subject ST (default: ${STATE})
  --locality <L>        Subject L  (default: ${LOCALITY})
  --org <O>             Subject O  (default: ${ORG})
  --force               Overwrite existing keys/certs (default: off)

CA Flags:
  --ca-cn <name>         CA common name (default: ${CA_CN})
  --ca-days <days>       CA validity days (default: ${VALID_DAYS_CA})

Server Flags (gen-server):
  --server-cn <name>     Server CN (default: ${SERVER_CN})
  --server-ou <ou>       Server OU (default: ${SERVER_OU})
  --server-days <days>   Server cert validity days (default: ${VALID_DAYS_LEAF})
  --server-dns <dns>     Add DNS SAN (repeatable)
  --server-ip <ip>       Add IP SAN (repeatable)

Agent Flags (gen-agent):
  --agent-id <id>        Agent id (required)
  --agent-ou <ou>        Agent OU (default: ${AGENT_OU})
  --agent-days <days>    Agent cert validity days (default: ${VALID_DAYS_LEAF})
  --spiffe-prefix <uri>  URI SAN prefix (default: ${SPIFFE_PREFIX})

Examples:
  # 1) Create CA
  $0 init-ca --out-dir configs/certs

  # 2) Create server cert with SANs
  $0 gen-server --server-dns command-server --server-dns localhost --server-ip 127.0.0.1

  # 3) Create an agent cert with URI SAN
  $0 gen-agent --agent-id home-01

  # 4) Everything in one go:
  $0 all --agent-id home-01 --server-dns command-server --server-dns localhost --server-ip 127.0.0.1

Notes:
  - Private keys are written to:
      <out-dir>/ca.key
      <out-dir>/server.key
      <out-dir>/agents/<agent-id>.key
    Do NOT commit *.key to git.
EOF
}

err() {
	echo "error: $*" >&2
	exit 1
}

need_openssl() {
	command -v openssl >/dev/null 2>&1 || err "openssl not found in PATH"
}

mkdirs() {
	mkdir -p "${OUT_DIR}"
	mkdir -p "${AGENTS_DIR}"
}

maybe_rm() {
	local f="$1"
	if [[ -f "$f" && "$FORCE" -eq 1 ]]; then
		rm -f "$f"
	fi
}

file_exists_guard() {
	local f="$1"
	if [[ -f "$f" && "$FORCE" -eq 0 ]]; then
		err "file already exists: $f (use --force to overwrite)"
	fi
}

subject_str() {
	local ou="$1"
	local cn="$2"
	echo "/C=${COUNTRY}/ST=${STATE}/L=${LOCALITY}/O=${ORG}/OU=${ou}/CN=${cn}"
}

write_server_cnf() {
	local cnf_path="$1"
	local cn="$2"

	local alt=""
	local i=1
	for d in "${SERVER_DNS[@]:-}"; do
		alt+="DNS.${i} = ${d}"$'\n'
		i=$((i + 1))
	done

	local j=1
	for ip in "${SERVER_IPS[@]:-}"; do
		alt+="IP.${j} = ${ip}"$'\n'
		j=$((j + 1))
	done

	# Basic fallback SANs if none provided
	if [[ ${#SERVER_DNS[@]} -eq 0 && ${#SERVER_IPS[@]} -eq 0 ]]; then
		alt+="DNS.1 = command-server"$'\n'
		alt+="DNS.2 = localhost"$'\n'
		alt+="IP.1  = 127.0.0.1"$'\n'
	fi

	cat >"$cnf_path" <<EOF
[req]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = req_distinguished_name
req_extensions = req_ext

[req_distinguished_name]
C  = ${COUNTRY}
ST = ${STATE}
L  = ${LOCALITY}
O  = ${ORG}
OU = ${SERVER_OU}
CN = ${cn}

[req_ext]
subjectAltName = @alt_names

[alt_names]
${alt}
EOF
}

write_agent_cnf() {
	local cnf_path="$1"
	local agent_id="$2"

	cat >"$cnf_path" <<EOF
[req]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = req_distinguished_name
req_extensions = req_ext

[req_distinguished_name]
C  = ${COUNTRY}
ST = ${STATE}
L  = ${LOCALITY}
O  = ${ORG}
OU = ${AGENT_OU}
CN = ${agent_id}

[req_ext]
subjectAltName = @alt_names

[alt_names]
URI.1 = ${SPIFFE_PREFIX}${agent_id}
EOF
}

# ----------------------------
# Commands
# ----------------------------
cmd_init_ca() {
	mkdirs
	need_openssl

	local ca_key="${OUT_DIR}/ca.key"
	local ca_crt="${OUT_DIR}/ca.crt"

	maybe_rm "$ca_key"
	maybe_rm "$ca_crt"

	file_exists_guard "$ca_key"
	file_exists_guard "$ca_crt"

	echo "[+] Generating CA key: ${ca_key}"
	openssl genrsa -out "${ca_key}" 4096

	echo "[+] Generating CA cert: ${ca_crt}"
	openssl req -x509 -new -nodes \
		-key "${ca_key}" \
		-sha256 -days "${VALID_DAYS_CA}" \
		-out "${ca_crt}" \
		-subj "$(subject_str "CA" "${CA_CN}")"

	echo "[✓] CA created:"
	echo "    ${ca_crt}"
	echo "    ${ca_key}"
}

cmd_gen_server() {
	mkdirs
	need_openssl

	local ca_key="${OUT_DIR}/ca.key"
	local ca_crt="${OUT_DIR}/ca.crt"

	[[ -f "${ca_key}" ]] || err "missing CA key: ${ca_key} (run init-ca first)"
	[[ -f "${ca_crt}" ]] || err "missing CA cert: ${ca_crt} (run init-ca first)"

	local server_key="${OUT_DIR}/server.key"
	local server_csr="${OUT_DIR}/server.csr"
	local server_crt="${OUT_DIR}/server.crt"
	local server_cnf="${OUT_DIR}/server-openssl.cnf"
	local ca_srl="${OUT_DIR}/ca.srl"

	maybe_rm "$server_key"
	maybe_rm "$server_csr"
	maybe_rm "$server_crt"
	maybe_rm "$server_cnf"

	file_exists_guard "$server_key"
	file_exists_guard "$server_crt"

	echo "[+] Writing server OpenSSL config: ${server_cnf}"
	write_server_cnf "${server_cnf}" "${SERVER_CN}"

	echo "[+] Generating server key: ${server_key}"
	openssl genrsa -out "${server_key}" 2048

	echo "[+] Generating server CSR: ${server_csr}"
	openssl req -new \
		-key "${server_key}" \
		-out "${server_csr}" \
		-config "${server_cnf}" \
		-reqexts req_ext

	echo "[+] Signing server cert: ${server_crt}"
	openssl x509 -req \
		-in "${server_csr}" \
		-CA "${ca_crt}" \
		-CAkey "${ca_key}" \
		-CAcreateserial \
		-out "${server_crt}" \
		-days "${VALID_DAYS_LEAF}" -sha256 \
		-extensions req_ext \
		-extfile "${server_cnf}"

	# Keep ca.srl around (normal). If you want to remove CSR/CNF, you can.
	echo "[✓] Server cert created:"
	echo "    ${server_crt}"
	echo "    ${server_key}"
}

cmd_gen_agent() {
	mkdirs
	need_openssl

	local ca_key="${OUT_DIR}/ca.key"
	local ca_crt="${OUT_DIR}/ca.crt"

	[[ -f "${ca_key}" ]] || err "missing CA key: ${ca_key} (run init-ca first)"
	[[ -f "${ca_crt}" ]] || err "missing CA cert: ${ca_crt} (run init-ca first)"

	[[ -n "${AGENT_ID:-}" ]] || err "--agent-id is required"

	local agent_key="${AGENTS_DIR}/${AGENT_ID}.key"
	local agent_csr="${AGENTS_DIR}/${AGENT_ID}.csr"
	local agent_crt="${AGENTS_DIR}/${AGENT_ID}.crt"
	local agent_cnf="${AGENTS_DIR}/${AGENT_ID}-openssl.cnf"

	maybe_rm "$agent_key"
	maybe_rm "$agent_csr"
	maybe_rm "$agent_crt"
	maybe_rm "$agent_cnf"

	file_exists_guard "$agent_key"
	file_exists_guard "$agent_crt"

	echo "[+] Writing agent OpenSSL config: ${agent_cnf}"
	write_agent_cnf "${agent_cnf}" "${AGENT_ID}"

	echo "[+] Generating agent key: ${agent_key}"
	openssl genrsa -out "${agent_key}" 2048

	echo "[+] Generating agent CSR: ${agent_csr}"
	openssl req -new \
		-key "${agent_key}" \
		-out "${agent_csr}" \
		-config "${agent_cnf}" \
		-reqexts req_ext

	echo "[+] Signing agent cert: ${agent_crt}"
	openssl x509 -req \
		-in "${agent_csr}" \
		-CA "${ca_crt}" \
		-CAkey "${ca_key}" \
		-CAcreateserial \
		-out "${agent_crt}" \
		-days "${AGENT_DAYS}" -sha256 \
		-extensions req_ext \
		-extfile "${agent_cnf}"

	echo "[✓] Agent cert created:"
	echo "    ${agent_crt}"
	echo "    ${agent_key}"
	echo "    URI SAN: ${SPIFFE_PREFIX}${AGENT_ID}"
}

cmd_all() {
	cmd_init_ca
	cmd_gen_server
	cmd_gen_agent
}

# ----------------------------
# Argument parsing
# ----------------------------
COMMAND="${1:-}"
shift || true

# Command-local defaults that can be overridden
AGENT_ID=""
AGENT_DAYS="${VALID_DAYS_LEAF}"

parse_args() {
	while [[ $# -gt 0 ]]; do
		case "$1" in
		--out-dir)
			OUT_DIR="${2:?missing value for --out-dir}"
			AGENTS_DIR="${OUT_DIR}/agents"
			shift 2
			;;
		--country)
			COUNTRY="${2:?missing value for --country}"
			shift 2
			;;
		--state)
			STATE="${2:?missing value for --state}"
			shift 2
			;;
		--locality)
			LOCALITY="${2:?missing value for --locality}"
			shift 2
			;;
		--org)
			ORG="${2:?missing value for --org}"
			shift 2
			;;
		--force)
			FORCE=1
			shift 1
			;;

		# CA
		--ca-cn)
			CA_CN="${2:?missing value for --ca-cn}"
			shift 2
			;;
		--ca-days)
			VALID_DAYS_CA="${2:?missing value for --ca-days}"
			shift 2
			;;

		# Server
		--server-cn)
			SERVER_CN="${2:?missing value for --server-cn}"
			shift 2
			;;
		--server-ou)
			SERVER_OU="${2:?missing value for --server-ou}"
			shift 2
			;;
		--server-days)
			VALID_DAYS_LEAF="${2:?missing value for --server-days}"
			shift 2
			;;
		--server-dns)
			SERVER_DNS+=("${2:?missing value for --server-dns}")
			shift 2
			;;
		--server-ip)
			SERVER_IPS+=("${2:?missing value for --server-ip}")
			shift 2
			;;

		# Agent
		--agent-id)
			AGENT_ID="${2:?missing value for --agent-id}"
			shift 2
			;;
		--agent-ou)
			AGENT_OU="${2:?missing value for --agent-ou}"
			shift 2
			;;
		--agent-days)
			AGENT_DAYS="${2:?missing value for --agent-days}"
			shift 2
			;;
		--spiffe-prefix)
			SPIFFE_PREFIX="${2:?missing value for --spiffe-prefix}"
			shift 2
			;;

		-h | --help)
			usage
			exit 0
			;;
		*)
			err "unknown flag: $1 (use --help)"
			;;
		esac
	done
}

# ----------------------------
# Dispatch
# ----------------------------
case "${COMMAND}" in
init-ca | gen-server | gen-agent | all)
	parse_args "$@"
	;;
"" | -h | --help)
	usage
	exit 0
	;;
*)
	err "unknown command: ${COMMAND} (use --help)"
	;;
esac

# Recompute derived paths after parsing
AGENTS_DIR="${OUT_DIR}/agents"

# Run
case "${COMMAND}" in
init-ca) cmd_init_ca ;;
gen-server) cmd_gen_server ;;
gen-agent) cmd_gen_agent ;;
all) cmd_all ;;
esac
