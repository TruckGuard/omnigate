#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
#  OmniGate — Universal Deploy & Update Script
#
#  Usage (interactive):
#    ./deploy.sh
#
#  Usage (non-interactive via env vars):
#    VERSION=v1.2.3 COMPONENTS=backend MODE=update ./deploy.sh
#
#  Key env vars:
#    VERSION        — release tag to install/update to (e.g. v1.2.3)
#    COMPONENTS     — what to deploy: all | backend | daemon  (default: all)
#    MODE           — install | update  (default: auto-detect)
#    GITHUB_REPO    — GitHub repo slug  (default: TruckGuard/omnigate)
#    GHCR_TOKEN     — token for pulling GHCR images (if repo is private)
#    ENV_FILE       — path to .env file  (default: .env)
# ═══════════════════════════════════════════════════════════════════════════════
set -euo pipefail

# ── Colours & formatting ───────────────────────────────────────────────────────
BOLD='\033[1m';    RESET='\033[0m'
RED='\033[0;31m';  GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BLUE='\033[0;34m';  MAGENTA='\033[0;35m'
DIM='\033[2m'

log_info()    { echo -e "${CYAN}  ●${RESET}  $*"; }
log_ok()      { echo -e "${GREEN}  ✔${RESET}  $*"; }
log_warn()    { echo -e "${YELLOW}  ⚠${RESET}  $*"; }
log_error()   { echo -e "${RED}  ✖${RESET}  $*" >&2; }
log_section() { echo -e "\n${BOLD}${BLUE}▶ $*${RESET}"; }
log_step()    { echo -e "${MAGENTA}  →${RESET}  $*"; }
die()         { log_error "$*"; exit 1; }

banner() {
  echo -e "${BOLD}${CYAN}"
  echo "  ╔═══════════════════════════════════════╗"
  echo "  ║        OmniGate Deploy Script         ║"
  echo "  ║   IoT Data Ingestion Platform         ║"
  echo "  ╚═══════════════════════════════════════╝"
  echo -e "${RESET}"
}

# ── Defaults ───────────────────────────────────────────────────────────────────
GITHUB_REPO="${GITHUB_REPO:-TruckGuard/omnigate}"
ENV_FILE="${ENV_FILE:-.env}"
COMPONENTS="${COMPONENTS:-}"    # filled interactively if empty
MODE="${MODE:-}"                # filled interactively if empty
VERSION="${VERSION:-}"          # filled interactively if empty
GHCR_TOKEN="${GHCR_TOKEN:-}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE_URL="https://github.com/${GITHUB_REPO}/releases/download"

# ── Helpers ────────────────────────────────────────────────────────────────────
ask() {
  # ask VAR_NAME "Prompt" ["default"]
  local var="$1" prompt="$2" default="${3:-}"
  local current="${!var:-}"
  if [[ -n "$current" ]]; then
    log_info "${prompt}: ${DIM}${current} (preset)${RESET}"
    return
  fi
  if [[ -n "$default" ]]; then
    read -rp "  ${prompt} [${default}]: " input
    printf -v "$var" '%s' "${input:-$default}"
  else
    read -rp "  ${prompt}: " input
    while [[ -z "$input" ]]; do
      log_warn "This field is required."
      read -rp "  ${prompt}: " input
    done
    printf -v "$var" '%s' "$input"
  fi
}

ask_optional() {
  # Like ask() but empty input is accepted (field is optional).
  local var="$1" prompt="$2" default="${3:-}"
  local current="${!var:-}"
  if [[ -n "$current" ]]; then
    log_info "${prompt}: ${DIM}${current} (preset)${RESET}"
    return
  fi
  read -rp "  ${prompt} [${default}]: " input
  printf -v "$var" '%s' "${input:-$default}"
}

ask_secret() {
  local var="$1" prompt="$2" default="${3:-}"
  local current="${!var:-}"
  if [[ -n "$current" ]]; then
    log_info "${prompt}: ${DIM}******** (preset)${RESET}"
    return
  fi
  if [[ -n "$default" ]]; then
    read -rsp "  ${prompt} [auto-generated, Enter to accept]: " input
    echo
    printf -v "$var" '%s' "${input:-$default}"
  else
    read -rsp "  ${prompt}: " input
    echo
    while [[ -z "$input" ]]; do
      log_warn "This field is required."
      read -rsp "  ${prompt}: " input
      echo
    done
    printf -v "$var" '%s' "$input"
  fi
}

ask_menu() {
  # ask_menu VAR_NAME "Title" option1 option2 ...
  local var="$1" title="$2"; shift 2
  local opts=("$@")
  local current="${!var:-}"
  if [[ -n "$current" ]]; then
    log_info "${title}: ${DIM}${current} (preset)${RESET}"
    return
  fi
  echo -e "  ${BOLD}${title}${RESET}"
  local i=1
  for opt in "${opts[@]}"; do
    echo -e "    ${CYAN}[$i]${RESET} $opt"
    ((i++))
  done
  local choice
  while true; do
    read -rp "  Choice [1]: " choice
    choice="${choice:-1}"
    if [[ "$choice" =~ ^[0-9]+$ ]] && (( choice >= 1 && choice <= ${#opts[@]} )); then
      printf -v "$var" '%s' "${opts[$((choice-1))]}"
      break
    fi
    log_warn "Enter a number between 1 and ${#opts[@]}"
  done
}

check_command() {
  command -v "$1" &>/dev/null || die "'$1' is required but not installed."
}

download() {
  local url="$1" dest="$2"
  log_step "Downloading $(basename "$dest") …"
  if command -v curl &>/dev/null; then
    curl -fsSL "$url" -o "$dest"
  else
    wget -q "$url" -O "$dest"
  fi
}

fetch_latest_version() {
  local api="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"
  if command -v curl &>/dev/null; then
    curl -fsSL "$api" | grep '"tag_name"' | sed 's/.*"tag_name": *"\(.*\)".*/\1/'
  else
    wget -qO- "$api" | grep '"tag_name"' | sed 's/.*"tag_name": *"\(.*\)".*/\1/'
  fi
}

# ── Dependency checks ──────────────────────────────────────────────────────────
check_deps() {
  log_section "Checking dependencies"
  check_command docker
  docker compose version &>/dev/null || die "'docker compose' plugin is required."
  command -v curl &>/dev/null || check_command wget
  command -v unzip &>/dev/null || die "'unzip' is required but not installed."
  log_ok "All dependencies satisfied"
}

# ── Bootstrap: download release bundle if files are missing ───────────────────
bootstrap() {
  # If the infra bundle is already next to this script, we're good.
  if [[ -f "${SCRIPT_DIR}/infra/docker-compose.prod.yaml" ]]; then
    log_ok "Release bundle already present"
    return
  fi

  log_section "Downloading release bundle"
  log_info "No local files found — will download the release archive from GitHub."

  # We need VERSION before we can build the download URL.
  select_version

  local zip_name="omnigate-${VERSION}.zip"
  local zip_url="${BASE_URL}/${VERSION}/${zip_name}"
  local tmp_zip
  tmp_zip="$(mktemp --suffix=.zip)"

  download "$zip_url" "$tmp_zip"

  log_step "Extracting ${zip_name} into ${SCRIPT_DIR} …"
  unzip -q "$tmp_zip" -d "${SCRIPT_DIR}"
  rm -f "$tmp_zip"

  log_ok "Bundle extracted — all files ready"
}

# ── Version selection ──────────────────────────────────────────────────────────
select_version() {
  log_section "Version"
  if [[ -z "$VERSION" ]]; then
    log_step "Fetching latest release from GitHub…"
    local latest
    latest="$(fetch_latest_version 2>/dev/null || echo '')"
    if [[ -n "$latest" ]]; then
      log_info "Latest available: ${BOLD}${latest}${RESET}"
    fi
    ask VERSION "Version to deploy" "${latest:-v1.0.0}"
  fi
  log_ok "Target version: ${BOLD}${VERSION}${RESET}"
}

# ── Component & mode selection ─────────────────────────────────────────────────
select_components() {
  log_section "What to deploy"
  ask_menu COMPONENTS "Components" \
    "all        — backend + observability + scale daemon" \
    "backend    — Docker app services only" \
    "obs        — Observability stack only (SigNoz)" \
    "daemon     — Scale daemon only" \
    "backend+obs — App services + observability"
  # normalise to first word
  COMPONENTS="${COMPONENTS%% *}"
  # alias
  [[ "$COMPONENTS" == "backend+obs" ]] && COMPONENTS="backend_obs"

  log_section "Mode"
  local backend_running=false
  if docker compose --env-file "${ENV_FILE}" -f "${SCRIPT_DIR}/infra/docker-compose.prod.yaml" \
       ps --services --filter status=running 2>/dev/null | grep -q .; then
    backend_running=true
  fi
  local daemon_running=false
  systemctl is-active --quiet scale-daemon 2>/dev/null && daemon_running=true

  if [[ -z "$MODE" ]]; then
    if $backend_running || $daemon_running; then
      ask_menu MODE "Action" \
        "update  — pull new images / binary, restart" \
        "install — fresh install (keeps existing .env)"
    else
      MODE="install"
      log_info "No running services detected — performing fresh install"
    fi
    MODE="${MODE%% *}"
  fi
  log_ok "Components: ${BOLD}${COMPONENTS}${RESET}   Mode: ${BOLD}${MODE}${RESET}"
}

# ── Observability: configure .env ─────────────────────────────────────────────
configure_obs() {
  local obs_env="${SCRIPT_DIR}/observability/.env"

  if [[ -f "$obs_env" && "$MODE" == "update" ]]; then
    log_ok "Existing observability/.env found — skipping reconfiguration"
    return
  fi

  if [[ -f "$obs_env" ]]; then
    log_warn "observability/.env already exists."
    local overwrite
    read -rp "  Reconfigure it? [y/N]: " overwrite
    [[ "${overwrite,,}" == "y" ]] || { log_info "Keeping existing observability/.env"; return; }
  fi

  log_info "Configure the observability stack (SigNoz):"
  echo

  local SIGNOZ_JWT_SECRET
  SIGNOZ_JWT_SECRET="${SIGNOZ_JWT_SECRET:-}"
  ask_secret SIGNOZ_JWT_SECRET "SigNoz JWT secret (≥32 chars)"

  log_step "Writing observability/.env …"
  cat > "$obs_env" <<EOF
# Generated by deploy.sh — $(date -u +"%Y-%m-%dT%H:%M:%SZ")
COMPOSE_PROJECT_NAME=omnigate-obs
VERSION=v0.121.1
OTELCOL_TAG=v0.144.3
SIGNOZ_JWT_SECRET=${SIGNOZ_JWT_SECRET}
EOF
  chmod 600 "$obs_env"
  log_ok "observability/.env written"
}

# ── Observability: deploy ──────────────────────────────────────────────────────
deploy_obs() {
  log_section "Observability — ${MODE}"

  local obs_dir="${SCRIPT_DIR}/observability"
  local compose_file="${obs_dir}/docker-compose.yaml"
  [[ -f "$compose_file" ]] || die "observability/docker-compose.yaml not found in ${SCRIPT_DIR}"

  log_step "Pulling observability images…"
  docker compose --env-file "${obs_dir}/.env" -f "$compose_file" pull

  log_step "Starting observability stack…"
  docker compose --env-file "${obs_dir}/.env" -f "$compose_file" up -d --remove-orphans

  log_ok "Observability stack running"

  if [[ -f "${obs_dir}/seed-dashboards.sh" ]]; then
    log_step "Seeding SigNoz dashboards…"
    bash "${obs_dir}/seed-dashboards.sh" 2>/dev/null \
      && log_ok "Dashboards seeded" \
      || log_warn "Dashboard seeding skipped (SigNoz may not be ready yet — run manually later)"
  fi
}

# ── Backend: configure .env ────────────────────────────────────────────────────
configure_env() {
  log_section "Backend configuration"

  if [[ -f "${ENV_FILE}" && "$MODE" == "update" ]]; then
    log_ok "Existing ${ENV_FILE} found — skipping reconfiguration"
    return
  fi

  if [[ -f "${ENV_FILE}" ]]; then
    log_warn "${ENV_FILE} already exists."
    local overwrite
    read -rp "  Reconfigure it? [y/N]: " overwrite
    [[ "${overwrite,,}" == "y" ]] || { log_info "Keeping existing ${ENV_FILE}"; return; }
  fi

  local WORKER_SYSTEM_KEY PULLER_API_KEY ADMIN_DEFAULT_PASSWORD
  local AUTH_PASSWORD CORE_PASSWORD POSTGRES_PASSWORD
  local GARAGE_RPC_SECRET STORAGE_ACCESS_KEY STORAGE_SECRET_KEY
  local PUBLIC_URL GATEWAY_PORT

  log_info "Secrets below are auto-generated. Press Enter to accept or type your own."
  echo

  ask_secret WORKER_SYSTEM_KEY      "Worker system key"              "$(openssl rand -hex 32)"
  ask_secret PULLER_API_KEY         "Puller API key"                 "$(openssl rand -hex 32)"
  ask_secret ADMIN_DEFAULT_PASSWORD "Admin default password (login)" ""
  ask_secret AUTH_PASSWORD          "Auth DB password"               "$(openssl rand -hex 16)"
  ask_secret CORE_PASSWORD          "Core DB password"               "$(openssl rand -hex 16)"
  ask_secret POSTGRES_PASSWORD      "Postgres superuser password"    "$(openssl rand -hex 16)"
  ask_secret GARAGE_RPC_SECRET      "Garage RPC secret"              "$(openssl rand -hex 32)"
  ask_secret STORAGE_ACCESS_KEY     "Garage storage access key"      "GK$(openssl rand -hex 12)"
  ask_secret STORAGE_SECRET_KEY     "Garage storage secret key"      "$(openssl rand -hex 32)"

  PUBLIC_URL="${PUBLIC_URL:-}"
  GATEWAY_PORT="${GATEWAY_PORT:-}"
  ask_optional PUBLIC_URL  "Public URL (e.g. https://omnigate.example.com, leave empty to skip)"
  ask GATEWAY_PORT "Gateway port" "80"

  log_step "Writing ${ENV_FILE} …"
  cat > "${ENV_FILE}" <<EOF
# Generated by deploy.sh — $(date -u +"%Y-%m-%dT%H:%M:%SZ")

# ── Database ──────────────────────────────────────────────────────────────────
POSTGRES_USER=postgres
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
POSTGRES_DB=postgres

AUTH_DB_NAME=omnigate_auth
AUTH_USER=auth_user
AUTH_PASSWORD=${AUTH_PASSWORD}

CORE_DB_NAME=omnigate_core
CORE_USER=core_user
CORE_PASSWORD=${CORE_PASSWORD}

# ── Storage (Garage / S3) ─────────────────────────────────────────────────────
GARAGE_RPC_SECRET=${GARAGE_RPC_SECRET}
GARAGE_ADMIN_TOKEN=
STORAGE_ENDPOINT=garage:3900
STORAGE_ACCESS_KEY=${STORAGE_ACCESS_KEY}
STORAGE_SECRET_KEY=${STORAGE_SECRET_KEY}
STORAGE_BUCKET=omnigate-data

# ── Cache / Event Bus ─────────────────────────────────────────────────────────
VALKEY_ADDR=valkey:6379

# ── Auth & Security ───────────────────────────────────────────────────────────
WORKER_SYSTEM_KEY=${WORKER_SYSTEM_KEY}
PULLER_API_KEY=${PULLER_API_KEY}
ADMIN_DEFAULT_PASSWORD=${ADMIN_DEFAULT_PASSWORD}

# ── Networking ────────────────────────────────────────────────────────────────
PUBLIC_URL=${PUBLIC_URL}
GATEWAY_PORT=${GATEWAY_PORT}

# ── Service image version ─────────────────────────────────────────────────────
OMNIGATE_VERSION=${VERSION#v}
EOF
  chmod 600 "${ENV_FILE}"
  log_ok "${ENV_FILE} written (permissions: 600)"
}

# ── Backend: validate .env before deploying ───────────────────────────────────
validate_env() {
  log_section "Validating ${ENV_FILE}"
  [[ -f "${ENV_FILE}" ]] || die "${ENV_FILE} not found. Run with MODE=install or create it first."

  local errors=0 val

  _env_get() { grep -E "^$1=" "${ENV_FILE}" | cut -d= -f2- | tr -d '"' || true; }

  local placeholder_keys=(
    WORKER_SYSTEM_KEY PULLER_API_KEY ADMIN_DEFAULT_PASSWORD
    GARAGE_RPC_SECRET STORAGE_ACCESS_KEY STORAGE_SECRET_KEY
    POSTGRES_PASSWORD AUTH_PASSWORD CORE_PASSWORD
  )
  for key in "${placeholder_keys[@]}"; do
    val="$(_env_get "$key")"
    if [[ -z "$val" || "$val" == change_me* || "$val" == GKxxxxx* ]]; then
      log_error "${key} is not set or still contains a placeholder value"
      (( errors++ )) || true
    fi
  done

  # Garage key ID must be GK + exactly 24 lowercase hex chars
  val="$(_env_get STORAGE_ACCESS_KEY)"
  if [[ -n "$val" && "$val" != change_me* && "$val" != GKxxxxx* ]]; then
    if ! [[ "$val" =~ ^GK[0-9a-f]{24}$ ]]; then
      log_error "STORAGE_ACCESS_KEY has invalid format (got: ${val})"
      log_error "  Required: GK followed by exactly 24 lowercase hex chars"
      log_error "  Generate: echo \"GK\$(openssl rand -hex 12)\""
      (( errors++ )) || true
    fi
  fi

  # Garage RPC secret must be exactly 64 lowercase hex chars
  val="$(_env_get GARAGE_RPC_SECRET)"
  if [[ -n "$val" && "$val" != change_me* ]]; then
    if ! [[ "$val" =~ ^[0-9a-f]{64}$ ]]; then
      log_error "GARAGE_RPC_SECRET must be exactly 64 lowercase hex chars"
      log_error "  Generate: openssl rand -hex 32"
      (( errors++ )) || true
    fi
  fi

  (( errors == 0 )) || die "Fix the ${errors} error(s) above in ${ENV_FILE} before deploying."
  log_ok "${ENV_FILE} looks good"
}

# ── Backend: set image tags in compose ────────────────────────────────────────
pin_image_version() {
  # CI tags images as "1.2.3" (semver without the leading v).
  # Strip the v prefix so the compose image tag matches what was pushed.
  export OMNIGATE_VERSION="${VERSION#v}"
}

# ── Shared network ────────────────────────────────────────────────────────────
ensure_observability_network() {
  if ! docker network inspect omnigate-observability &>/dev/null; then
    log_step "Creating shared network omnigate-observability…"
    docker network create omnigate-observability
    log_ok "Network created"
  else
    log_ok "Network omnigate-observability already exists"
  fi
}

# ── Backend: deploy ────────────────────────────────────────────────────────────
deploy_backend() {
  log_section "Backend — ${MODE}"

  validate_env

  local compose_file="${SCRIPT_DIR}/infra/docker-compose.prod.yaml"
  [[ -f "$compose_file" ]] || die "infra/docker-compose.prod.yaml not found in ${SCRIPT_DIR}"

  if [[ -n "$GHCR_TOKEN" ]]; then
    log_step "Logging in to GHCR…"
    echo "$GHCR_TOKEN" | docker login ghcr.io -u _ --password-stdin
    log_ok "GHCR authenticated"
  fi

  pin_image_version

  log_step "Pulling images for ${VERSION}…"
  docker compose --env-file "${ENV_FILE}" -f "$compose_file" pull

  log_step "Starting services…"
  docker compose --env-file "${ENV_FILE}" -f "$compose_file" up -d --remove-orphans

  log_step "Waiting for health checks…"
  local timeout=60 elapsed=0
  while (( elapsed < timeout )); do
    local unhealthy
    unhealthy=$(docker compose --env-file "${ENV_FILE}" -f "$compose_file" \
      ps --format json 2>/dev/null \
      | grep -c '"Health":"unhealthy"' || true)
    [[ "$unhealthy" -eq 0 ]] && break
    sleep 3; (( elapsed += 3 ))
  done

  log_ok "Backend services running"
  docker compose --env-file "${ENV_FILE}" -f "$compose_file" ps
}

# ── Daemon: configure ──────────────────────────────────────────────────────────
configure_daemon() {
  local cfg_file="/etc/omnigate/scale-daemon.json"

  if [[ -f "$cfg_file" && "$MODE" == "update" ]]; then
    log_ok "Existing ${cfg_file} found — skipping reconfiguration"
    return
  fi

  if [[ -f "$cfg_file" ]]; then
    log_warn "${cfg_file} already exists."
    local overwrite
    read -rp "  Reconfigure it? [y/N]: " overwrite
    [[ "${overwrite,,}" == "y" ]] || { log_info "Keeping existing ${cfg_file}"; return; }
  fi

  log_info "Configure the scale daemon:"
  echo

  local SCALE_HOST SCALE_PORT INGESTOR_URL DEVICE_ID DAEMON_API_KEY
  local DEBOUNCE_MS MIN_WEIGHT_KG LOG_LEVEL

  SCALE_HOST="${SCALE_HOST:-}"; SCALE_PORT="${SCALE_PORT:-}"
  INGESTOR_URL="${INGESTOR_URL:-}"; DEVICE_ID="${DEVICE_ID:-}"
  DAEMON_API_KEY="${DAEMON_API_KEY:-}"
  DEBOUNCE_MS="${DEBOUNCE_MS:-5000}"; MIN_WEIGHT_KG="${MIN_WEIGHT_KG:-500}"
  LOG_LEVEL="${LOG_LEVEL:-info}"

  ask SCALE_HOST    "Scale TCP host (IP address)"
  ask SCALE_PORT    "Scale TCP port" "5001"
  ask INGESTOR_URL  "Ingestor URL" "http://localhost:8090/ingest/event"
  ask DEVICE_ID     "Device ID (e.g. scale-gate-01)"
  ask_secret DAEMON_API_KEY "API key for this device"
  ask DEBOUNCE_MS   "Debounce window (ms)" "5000"
  ask MIN_WEIGHT_KG "Minimum weight to report (kg)" "500"
  ask LOG_LEVEL     "Log level" "info"

  mkdir -p /etc/omnigate
  cat > "$cfg_file" <<EOF
{
  "scale_host":       "${SCALE_HOST}",
  "scale_port":       "${SCALE_PORT}",
  "ingestor_url":     "${INGESTOR_URL}",
  "device_id":        "${DEVICE_ID}",
  "api_key":          "${DAEMON_API_KEY}",
  "debounce_ms":      ${DEBOUNCE_MS},
  "min_weight_kg":    ${MIN_WEIGHT_KG},
  "reconnect_sec":    5,
  "log_level":        "${LOG_LEVEL}",
  "http_timeout_sec": 10
}
EOF
  chmod 640 "$cfg_file"
  chown root:omnigate "$cfg_file" 2>/dev/null || true
  log_ok "${cfg_file} written"
}

# ── Daemon: install/update binary ──────────────────────────────────────────────
deploy_daemon() {
  log_section "Scale daemon — ${MODE}"

  [[ $EUID -eq 0 ]] || die "Installing the scale daemon requires root (sudo ./deploy.sh)"

  local arch
  case "$(uname -m)" in
    x86_64)  arch="amd64" ;;
    aarch64) arch="arm64" ;;
    *)       die "Unsupported architecture: $(uname -m)" ;;
  esac

  local tmp_dir
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT

  local binary_url="${BASE_URL}/${VERSION}/scale-daemon-linux-${arch}"
  local install_url="${BASE_URL}/${VERSION}/install-daemon.sh"
  local service_url="${BASE_URL}/${VERSION}/scale-daemon.service"
  local config_url="${BASE_URL}/${VERSION}/config.example.json"

  download "$binary_url" "${tmp_dir}/scale-daemon"
  download "$install_url" "${tmp_dir}/install-daemon.sh"
  download "$service_url" "${tmp_dir}/scale-daemon.service"
  download "$config_url"  "${tmp_dir}/config.example.json"

  chmod +x "${tmp_dir}/scale-daemon" "${tmp_dir}/install-daemon.sh"

  # Install binary + systemd unit first — this creates the omnigate user.
  log_step "Running install-daemon.sh…"
  "${tmp_dir}/install-daemon.sh"

  # Configure AFTER install so the omnigate user already exists for chown.
  configure_daemon

  # Restart to pick up the freshly written config.
  systemctl restart scale-daemon

  log_ok "Scale daemon ${VERSION} installed and started"
  log_info "Logs: sudo journalctl -u scale-daemon -f"
}

# ── Status ─────────────────────────────────────────────────────────────────────
show_status() {
  log_section "Deployment summary"

  if [[ "$COMPONENTS" == "all" || "$COMPONENTS" == "backend" || "$COMPONENTS" == "backend_obs" ]]; then
    log_info "Backend services:"
    docker compose --env-file "${ENV_FILE}" \
      -f "${SCRIPT_DIR}/infra/docker-compose.prod.yaml" ps 2>/dev/null || true
  fi

  if [[ "$COMPONENTS" == "all" || "$COMPONENTS" == "obs" || "$COMPONENTS" == "backend_obs" ]]; then
    log_info "Observability (SigNoz):"
    docker compose --env-file "${SCRIPT_DIR}/observability/.env" \
      -f "${SCRIPT_DIR}/observability/docker-compose.yaml" ps 2>/dev/null || true
  fi

  if [[ "$COMPONENTS" == "all" || "$COMPONENTS" == "daemon" ]]; then
    log_info "Scale daemon:"
    systemctl is-active --quiet scale-daemon 2>/dev/null \
      && log_ok "  scale-daemon is running" \
      || log_warn "  scale-daemon is not running"
  fi
  echo
}

# ── Main ───────────────────────────────────────────────────────────────────────
main() {
  banner
  check_deps
  bootstrap        # downloads ZIP if files not present; calls select_version internally if needed
  select_version   # no-op if VERSION already set by bootstrap
  select_components

  case "$COMPONENTS" in
    all)
      ensure_observability_network
      configure_env
      deploy_backend
      configure_obs
      deploy_obs
      deploy_daemon
      ;;
    backend)
      ensure_observability_network
      configure_env
      deploy_backend
      ;;
    obs)
      ensure_observability_network
      configure_obs
      deploy_obs
      ;;
    backend_obs)
      ensure_observability_network
      configure_env
      deploy_backend
      configure_obs
      deploy_obs
      ;;
    daemon)
      deploy_daemon
      ;;
    *)
      die "Unknown component: ${COMPONENTS}. Use: all | backend | obs | backend+obs | daemon"
      ;;
  esac

  show_status
  echo -e "${BOLD}${GREEN}  ✔  OmniGate ${VERSION} deployed successfully!${RESET}\n"
}

main "$@"
