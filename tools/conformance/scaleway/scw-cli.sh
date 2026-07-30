#!/usr/bin/env bash
# Conformance check: drive the emulator with the real Scaleway CLI.
#
# This is the only proof that matters. Unit tests assert what we believe the API does; scw asserts
# what the official client accepts. If the CLI parses our answers, the wire format is right.
#
# Usage: tools/conformance/scaleway/scw-cli.sh [endpoint]     (default http://127.0.0.1:4599)
set -euo pipefail

ENDPOINT="${1:-http://127.0.0.1:4599}"

# Never let a client reach anything but the local emulator. Without this, a
# missing endpoint does not fail: every official client falls back to the
# operator's stored credentials, and a test creates billable resources on a real
# account. That is not hypothetical — it happened, to this repository.
# shellcheck source=/dev/null
. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/guard.sh"
guard_local "$ENDPOINT"
ZONE="${ZONE:-fr-par-1}"

# The SDK validates credential FORMAT even though the emulator ignores the values: the access key
# must look like SCWXXXXXXXXXXXXXXXXX and the secret key must be a UUID. Fake but well-formed.
export SCW_ACCESS_KEY="SCWXXXXXXXXXXXXXXXXX"
export SCW_SECRET_KEY="11111111-1111-1111-1111-111111111111"
export SCW_DEFAULT_PROJECT_ID="11111111-1111-1111-1111-111111111111"
export SCW_DEFAULT_ORGANIZATION_ID="11111111-1111-1111-1111-111111111111"
export SCW_DEFAULT_ZONE="$ZONE"
export SCW_API_URL="$ENDPOINT"
export SCW_INSECURE="true"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { echo "  ok: $*"; }

echo "conformance: scw CLI against $ENDPOINT"

echo "- create"
created="$(scw instance server create name=conformance-1 type=DEV1-S zone="$ZONE" -o json 2>&1)" \
  || fail "create rejected by the CLI: $created"
id="$(printf '%s' "$created" | jq -r '.id // empty')"
[ -n "$id" ] || fail "no id in the create response: $created"
ok "server $id"

echo "- list"
list="$(scw instance server list zone="$ZONE" -o json)" || fail "list rejected: $list"
printf '%s' "$list" | jq -e --arg id "$id" 'any(.[]; .id == $id)' >/dev/null \
  || fail "the created server is missing from the list: $list"
ok "server present in the list"

echo "- get"
got="$(scw instance server get "$id" zone="$ZONE" -o json)" || fail "get rejected: $got"
printf '%s' "$got" | jq -e --arg id "$id" '.id == $id' >/dev/null || fail "get returned another server"
ok "read back identical"

# The CLI starts the server right after creating it, and the API refuses to delete a running
# server. Powering off first is what a real user does, so the suite does it too.
echo "- stop"
scw instance server stop "$id" zone="$ZONE" >/dev/null || fail "poweroff rejected"
state="$(scw instance server get "$id" zone="$ZONE" -o json | jq -r '.state')"
[ "$state" = "stopped" ] || fail "expected the server to be stopped, got '$state'"
ok "powered off"

echo "- delete"
scw instance server delete "$id" zone="$ZONE" >/dev/null || fail "delete rejected"
if scw instance server get "$id" zone="$ZONE" -o json >/dev/null 2>&1; then
  fail "the server still exists after delete"
fi
ok "deleted, and gone"

# Security groups. A fresh project already owns one, so the first list must return it: every
# client reads the existing groups before it creates anything.
echo "- security groups: the project default exists"
groups="$(scw instance security-group list zone="$ZONE" -o json)" || fail "list rejected: $groups"
printf '%s' "$groups" | jq -e 'any(.[]; .project_default == true)' >/dev/null \
  || fail "no default security group in a fresh project: $groups"
ok "default group served"

echo "- security group create, get, update"
# The CLI unwraps the response envelope for some commands and not for others, so the suite accepts
# both shapes rather than asserting the CLI's own presentation.
sg="$(scw instance security-group create name=conformance-sg description='conformance' \
        inbound-default-policy=drop zone="$ZONE" -o json)" || fail "create rejected: $sg"
sg_id="$(printf '%s' "$sg" | jq -r '(.security_group // .).id // empty')"
[ -n "$sg_id" ] || fail "no id in the create response: $sg"
read_back="$(scw instance security-group get "$sg_id" zone="$ZONE" -o json)" || fail "get rejected: $read_back"
printf '%s' "$read_back" | jq -e '(.security_group // .) | .inbound_default_policy == "drop" and .name == "conformance-sg"' >/dev/null \
  || fail "the group did not round-trip: $read_back"
scw instance security-group update security-group-id="$sg_id" name=conformance-sg-2 zone="$ZONE" -o json >/dev/null \
  || fail "update rejected"
ok "group $sg_id"

# Every rule subcommand takes named arguments only: the CLI has no positional form for them.
echo "- rules: create, list, update, delete"
rule="$(scw instance security-group create-rule security-group-id="$sg_id" protocol=TCP direction=inbound \
          action=accept ip-range=10.0.0.0/8 dest-port-from=22 zone="$ZONE" -o json)" || fail "create-rule rejected: $rule"
rule_id="$(printf '%s' "$rule" | jq -r '(.rule // .).id // empty')"
[ -n "$rule_id" ] || fail "no id in the create-rule response: $rule"
rules="$(scw instance security-group list-rules security-group-id="$sg_id" zone="$ZONE" -o json)" \
  || fail "list-rules rejected: $rules"
printf '%s' "$rules" | jq -e --arg id "$rule_id" 'any(.[]; .id == $id)' >/dev/null \
  || fail "the created rule is missing from the list: $rules"
got_rule="$(scw instance security-group get-rule security-group-id="$sg_id" security-group-rule-id="$rule_id" \
              zone="$ZONE" -o json)" || fail "get-rule rejected: $got_rule"
printf '%s' "$got_rule" | jq -e '(.rule // .) | .ip_range == "10.0.0.0/8" and .dest_port_from == 22' >/dev/null \
  || fail "the rule did not round-trip: $got_rule"
scw instance security-group update-rule security-group-id="$sg_id" security-group-rule-id="$rule_id" \
  action=drop zone="$ZONE" -o json >/dev/null || fail "update-rule rejected"
scw instance security-group delete-rule security-group-id="$sg_id" security-group-rule-id="$rule_id" \
  zone="$ZONE" >/dev/null || fail "delete-rule rejected"
ok "rule $rule_id"

echo "- a server carries its group, and the group refuses to be deleted under it"
sg_server="$(scw instance server create name=conformance-sg-server type=DEV1-S zone="$ZONE" \
               security-group-id="$sg_id" -o json)" || fail "create with a security group rejected: $sg_server"
sg_server_id="$(printf '%s' "$sg_server" | jq -r '.id')"
printf '%s' "$sg_server" | jq -e --arg id "$sg_id" '.security_group.id == $id' >/dev/null \
  || fail "the server did not take the group: $sg_server"
if scw instance security-group delete security-group-id="$sg_id" zone="$ZONE" >/dev/null 2>&1; then
  fail "the group was deleted while a server still used it"
fi
scw instance server stop "$sg_server_id" zone="$ZONE" >/dev/null || fail "cleanup: poweroff rejected"
scw instance server delete "$sg_server_id" zone="$ZONE" >/dev/null || fail "cleanup: server delete rejected"
scw instance security-group delete security-group-id="$sg_id" zone="$ZONE" >/dev/null || fail "delete rejected once free"
ok "attachment and precondition honoured"

# The CLI reserves an address and names it on the create. The emulator used to
# drop that argument and always answer an empty public_ips, so a server reported
# no public address whatever was attached to it. Found by the unread-field
# report; asserted here so it stays fixed.
echo "- an address reserved before the create comes back on the server"
ip="$(scw instance ip create zone="$ZONE" -o json)" || fail "ip create rejected: $ip"
ip_id="$(printf '%s' "$ip" | jq -r '(.ip // .).id')"
ip_address="$(printf '%s' "$ip" | jq -r '(.ip // .).address')"
[ -n "$ip_id" ] && [ "$ip_id" != null ] || fail "no id in the ip create response: $ip"

with_ip="$(scw instance server create name=conformance-ip type=DEV1-S zone="$ZONE" \
             ip="$ip_id" -o json)" || fail "create with a reserved ip rejected: $with_ip"
with_ip_id="$(printf '%s' "$with_ip" | jq -r '.id')"
printf '%s' "$with_ip" | jq -e --arg a "$ip_address" 'any(.public_ips[]; .address == $a)' >/dev/null \
  || fail "the server does not carry the address that was reserved for it: $with_ip"
# Read back rather than trusting the create: the list is computed from the IP
# resources, and a create that answered right could still read wrong.
again="$(scw instance server get "$with_ip_id" zone="$ZONE" -o json)"
printf '%s' "$again" | jq -e --arg a "$ip_address" 'any(.public_ips[]; .address == $a)' >/dev/null \
  || fail "the address is gone on a re-read: $again"
scw instance server stop "$with_ip_id" zone="$ZONE" >/dev/null || fail "cleanup: poweroff rejected"
scw instance server delete "$with_ip_id" zone="$ZONE" >/dev/null || fail "cleanup: delete rejected"
scw instance ip delete "$ip_id" zone="$ZONE" >/dev/null || fail "cleanup: ip delete rejected"
ok "reserved, attached, and reported"

echo "conformance: scw CLI passed"
