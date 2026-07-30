#!/usr/bin/env bash
# Refuse to run a client that is not pointed at a local emulator.
#
# This exists because it happened. A test script called `eval "$(feint env
# scaleway)"`, the command was broken and printed nothing, the eval silently
# succeeded on an empty string, and `scw instance server create` fell back to the
# operator's real ~/.config/scw/config.yaml. A DEV1-S server and a flexible IP
# were created on a paying account, and the command exited 0 the whole way.
#
# That is the exact failure this project exists to prevent — a client leaving for
# the real cloud because an address was missing — produced by the tool meant to
# prevent it. The lesson is not "be careful with eval". It is that **an empty
# environment must fail loudly rather than fall through to a real account**, and
# nothing but an explicit check can enforce that: every official client is
# designed to find credentials elsewhere when the environment has none.
#
# Source this at the top of any script that drives a real client:
#
#   . "$(dirname "${BASH_SOURCE[0]}")/guard.sh"
#   guard_local "$ENDPOINT"
#   guard_no_real_profile SCW_API_URL scw
#
# Usage of the two functions is deliberately separate: the first checks where the
# script intends to go, the second checks that the client cannot go anywhere else.

# guard_local refuses an endpoint that is not on this machine.
#
# A conformance suite pointed at a remote address is either a mistake or an
# attack on somebody's account; there is no legitimate third case. Loopback and
# a bare host with no dots are what a local emulator looks like.
guard_local() {
  local endpoint="${1:-}"
  case "$endpoint" in
    http://127.0.0.1:*|http://localhost:*|http://\[::1\]:*|http://0.0.0.0:*) return 0 ;;
    "")
      echo "FAIL: no endpoint given; refusing to run a client that would find its own" >&2
      exit 1 ;;
    *)
      echo "FAIL: endpoint $endpoint is not local; this suite drives an emulator, never a real cloud" >&2
      exit 1 ;;
  esac
}

# guard_no_real_profile refuses to run when the variable that redirects a client
# is unset, because every one of these clients falls back to a stored profile.
#
# The variable name and the client are passed in rather than listed here: the
# core of this project knows no provider and neither does this file.
guard_no_real_profile() {
  local var="$1" client="${2:-the client}"
  if [ -z "${!var:-}" ]; then
    cat >&2 <<EOF
FAIL: $var is not set.

$client falls back to its stored credentials when the environment says nothing,
so running it now would drive a real account. This is not hypothetical: it is how
a server and a flexible IP were once created on a paying account by a test of
this emulator.

Set $var to the local emulator, or use a client flag that pins the endpoint.
EOF
    exit 1
  fi
  case "${!var}" in
    http://127.0.0.1:*|http://localhost:*|http://\[::1\]:*|http://0.0.0.0:*) return 0 ;;
    *)
      echo "FAIL: $var is ${!var}, which is not local; refusing to run $client" >&2
      exit 1 ;;
  esac
}
