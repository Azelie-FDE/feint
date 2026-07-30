#!/usr/bin/env bash
# oapi-cli, pointed at the local emulator, for the network demo.
#
# It exists because oapi-cli has no --endpoint flag: it is configured through a
# JSON profile, and writing that profile inside a vhs tape is not possible (the
# parser has no escape for a quote inside a typed line). So the profile is
# written here, from the same fake credentials the conformance suite uses, and
# the tape calls this wrapper as `osc`.
#
# The endpoint is the bare host on purpose. oapi-cli appends /api/v1/<Call>
# itself, so passing http://host/api/v1 asks for /api/v1/api/v1/<Call> and gets a
# 404 that looks exactly like a missing route.
set -euo pipefail

ENDPOINT="${FEINT_DEMO_ENDPOINT:-http://127.0.0.1:4599}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CONFIG_DIR="${TMPDIR:-/tmp}/feint-demo-osc"
mkdir -p "$CONFIG_DIR"

set -a
# shellcheck source=/dev/null
. "$ROOT/tools/conformance/outscale/fake-credentials.env"
set +a

cat > "$CONFIG_DIR/config.json" <<EOF
{
  "default": {
    "access_key": "$OSC_ACCESS_KEY",
    "secret_key": "$OSC_SECRET_KEY",
    "region": "$OSC_REGION",
    "protocol": "http",
    "endpoints": { "api": "$ENDPOINT" }
  }
}
EOF

exec oapi-cli --config "$CONFIG_DIR/config.json" "$@"
