#!/usr/bin/env bash
# Report what the run actually exercised, not just what is mounted.
#
# The coverage report answers "how much of the upstream API is implemented".
# It cannot answer "how much of that has ever been driven by a real client", and
# until this existed nobody could: 74 routes were mounted and the suites touched
# three of them. Microcks names the same pair — a conformance index for what the
# contract could cover, a conformance score for what the last run did.
#
# Printed, not enforced. A threshold here would be a number invented to be met;
# the useful output is the list of routes no client has ever proven, which is a
# backlog somebody can act on.
#
# Usage: tools/conformance/score.sh [endpoint]
set -euo pipefail

ENDPOINT="${1:-http://127.0.0.1:4599}"
command -v jq >/dev/null 2>&1 || { echo "FAIL: jq is not installed" >&2; exit 1; }

report="$(curl -sf "$ENDPOINT/_feint/conformance")" \
  || { echo "FAIL: the emulator did not answer /_feint/conformance" >&2; exit 1; }

served="$(printf '%s' "$report" | jq -r '.served')"
exercised="$(printf '%s' "$report" | jq -r '.exercised')"
probed="$(printf '%s' "$report" | jq -r '.probed // 0')"
echo "conformance score: $exercised of $served routes proven by a real client"
# Never added to the line above. A probe proves the protocol — the route answers
# and its answer matches the provider's schema — and nothing about behaviour, so
# a probed route stays on the backlog until a client drives it. Merging the two
# is how a number goes up without anything being proven.
[ "$probed" = "0" ] || echo "protocol probe:    $probed more probed only (schema-valid, behaviour unproven)"

# A violation here means a response did not match the provider's own API
# description. It is a failure: the whole point of loading the contract is that
# nobody has to notice.
violations="$(printf '%s' "$report" | jq -r '.violations | to_entries[] | "  \(.key): \(.value | join("; "))"')"
if [ -n "$violations" ]; then
  echo "FAIL: responses that do not match the API description:" >&2
  echo "$violations" >&2
  exit 1
fi

# And the same treatment for the other half of the exchange: fields a real client
# sent that no handler read.
#
# The emulator had been recording them all along, and nothing looked. Terraform
# changed a server's commercial_type, the emulator answered 200, ignored the
# field, and the next plan asked for the same change forever — while
# /_feint/conformance listed `commercial_type` and `volumes` under
# unread_request_fields, run after run. An introspection nobody gates on is a
# confession nobody hears.
#
# A field here is not always a defect, so exemptions exist — and each one costs a
# line and a reason, the way Declined() does for operations.
#
# The only one so far: a client that echoes back a whole object it just read,
# where the API decides on the identifier alone. `exo` sends the entire
# instance-type and template objects it got from the catalogue; the emulator
# reads their id and ignores the rest, exactly as the real API does. Listing the
# sub-fields as unread would be reporting the client's verbosity as our defect.
ignored_fields='^(instance-type|template)\.'

unread="$(printf '%s' "$report" | jq -r --arg ignore "$ignored_fields" '
  .unread_request_fields
  | to_entries[]
  | {key: .key, value: [.value[] | select(test($ignore) | not)]}
  | select(.value | length > 0)
  | "  \(.key): \(.value | join(", "))"')"
if [ -n "$unread" ]; then
  echo "FAIL: fields a real client sent that no handler read:" >&2
  echo "$unread" >&2
  echo "       Either honour them, or say why they are ignored." >&2
  exit 1
fi

checked="$(printf '%s' "$report" | jq -r '.contracts | join(", ")')"
[ -z "$checked" ] || echo "  responses checked against the API description of: $checked"

printf '%s' "$report" | jq -r '
  .untouched
  | group_by(split("/")[0])
  | map("  \(length) route(s) no client has driven: \(.[0] | split("/")[0])")
  | .[]'
