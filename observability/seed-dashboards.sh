#!/usr/bin/env bash
# Imports OmniGate dashboards into SigNoz via the REST API.
#
# Usage:
#   SIGNOZ_EMAIL=admin@example.com SIGNOZ_PASSWORD=yourpassword ./seed-dashboards.sh
#
# Optional:
#   SIGNOZ_URL=http://localhost:8080  (default)

set -euo pipefail

SIGNOZ_URL="${SIGNOZ_URL:-http://localhost:8080}"
DASHBOARDS_DIR="$(dirname "$0")/dashboards"

if [[ -z "${SIGNOZ_EMAIL:-}" || -z "${SIGNOZ_PASSWORD:-}" ]]; then
  echo "Error: SIGNOZ_EMAIL and SIGNOZ_PASSWORD must be set." >&2
  echo "  SIGNOZ_EMAIL=admin@example.com SIGNOZ_PASSWORD=secret $0" >&2
  exit 1
fi

echo "→ Authenticating with SigNoz at ${SIGNOZ_URL}..."
TOKEN=$(curl -sf -X POST "${SIGNOZ_URL}/api/v1/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${SIGNOZ_EMAIL}\",\"password\":\"${SIGNOZ_PASSWORD}\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['accessJwt'])")

if [[ -z "${TOKEN}" ]]; then
  echo "Error: authentication failed — check email/password." >&2
  exit 1
fi

echo "→ Token acquired."

for f in "${DASHBOARDS_DIR}"/*.json; do
  name=$(basename "${f}" .json)
  echo -n "   Importing ${name}... "
  HTTP_STATUS=$(curl -sf -o /dev/null -w "%{http_code}" \
    -X POST "${SIGNOZ_URL}/api/v1/dashboards" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${TOKEN}" \
    -d @"${f}")
  echo "HTTP ${HTTP_STATUS}"
done

echo "Done."
