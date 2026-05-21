#!/usr/bin/env bash
# install.sh — install scale-daemon on a Linux edge device (requires root).
#
# Usage:
#   sudo ./install.sh
#
# The script expects the pre-compiled binary (scale-daemon or
# scale-daemon-linux-amd64 / scale-daemon-linux-arm64) to be in the same
# directory as this script.  Download the appropriate binary from the GitHub
# Release page before running.

set -euo pipefail

BINARY_DEST="/usr/local/bin/scale-daemon"
SERVICE_DEST="/etc/systemd/system/scale-daemon.service"
ENV_DIR="/etc/omnigate"
ENV_FILE="${ENV_DIR}/scale.env"
LOG_DIR="/var/log/omnigate"
SERVICE_USER="omnigate"

# ── Colour helpers ────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; exit 1; }

# ── Pre-flight checks ─────────────────────────────────────────────────────────
[[ $EUID -eq 0 ]] || error "This script must be run as root (sudo ./install.sh)"
command -v systemctl >/dev/null 2>&1 || error "systemd is required"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Locate the binary — accept arch-suffixed names as well.
BINARY=""
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64)  BINARY_SUFFIX="amd64" ;;
  aarch64) BINARY_SUFFIX="arm64" ;;
  *)       BINARY_SUFFIX="" ;;
esac

for candidate in \
    "${SCRIPT_DIR}/scale-daemon" \
    "${SCRIPT_DIR}/scale-daemon-linux-${BINARY_SUFFIX}"; do
  if [[ -f "${candidate}" ]]; then
    BINARY="${candidate}"
    break
  fi
done

[[ -n "${BINARY}" ]] || error "Binary not found in ${SCRIPT_DIR}. Download scale-daemon-linux-${BINARY_SUFFIX} from the GitHub Release."

# ── Create system user ────────────────────────────────────────────────────────
if ! id -u "${SERVICE_USER}" &>/dev/null; then
  info "Creating system user '${SERVICE_USER}'"
  useradd --system --no-create-home --shell /usr/sbin/nologin "${SERVICE_USER}"
fi

# ── Install binary ────────────────────────────────────────────────────────────
info "Installing binary → ${BINARY_DEST}"
install -m 755 "${BINARY}" "${BINARY_DEST}"

# ── Create config directory and env file ──────────────────────────────────────
mkdir -p "${ENV_DIR}"

if [[ -f "${ENV_FILE}" ]]; then
  warn "${ENV_FILE} already exists — skipping (edit manually to update)"
else
  info "Creating default env file → ${ENV_FILE}"
  cat > "${ENV_FILE}" <<'EOF'
# OmniGate Scale Daemon — environment configuration
# Edit this file then run: sudo systemctl restart scale-daemon

SCALE_HOST=192.168.1.100
SCALE_PORT=5001

INGESTOR_URL=http://omnigate.example.com:8090/ingest/event
DEVICE_ID=scale-gate-01
API_KEY=

# Milliseconds the weight must be stable before sending (default: 2000)
# DEBOUNCE_MS=2000

# Ignore readings below this weight in kg (default: 0 = report all)
# MIN_WEIGHT_KG=0

# OTLP HTTP collector for logs and traces (default: localhost:4318)
# OTEL_ENDPOINT=localhost:4318
EOF
  chmod 640 "${ENV_FILE}"
  chown root:"${SERVICE_USER}" "${ENV_FILE}"
fi

# ── Create log directory ──────────────────────────────────────────────────────
mkdir -p "${LOG_DIR}"
chown "${SERVICE_USER}":"${SERVICE_USER}" "${LOG_DIR}"

# ── Install systemd unit ──────────────────────────────────────────────────────
info "Installing systemd unit → ${SERVICE_DEST}"
install -m 644 "${SCRIPT_DIR}/scale-daemon.service" "${SERVICE_DEST}"

systemctl daemon-reload
systemctl enable scale-daemon
systemctl restart scale-daemon

echo ""
info "scale-daemon installed and started successfully."
echo ""
echo "  Status : sudo systemctl status scale-daemon"
echo "  Logs   : sudo journalctl -u scale-daemon -f"
echo "  Config : sudo nano ${ENV_FILE}"
echo ""
warn "Make sure to set API_KEY and INGESTOR_URL in ${ENV_FILE}!"
