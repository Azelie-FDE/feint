#!/usr/bin/env bash
# Fail when the README's generated section no longer matches the artefacts.
#
# The tables in the README used to be written by hand, and they said Scaleway
# served 12 routes when it served 55. Generating them fixed that once; this hook
# is what keeps it fixed, because a generated section nobody regenerates goes
# stale exactly like a hand-written one.
#
# Exit 2 on staleness, matching the drift gate: 0 ok, 1 error, 2 out of date.
set -euo pipefail

BIN="$(mktemp -d)/feint"
trap 'rm -rf "$(dirname "$BIN")"' EXIT

go build -o "$BIN" ./cmd/feint
"$BIN" docs --check
