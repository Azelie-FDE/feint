#!/usr/bin/env bash
# The issue taxonomy, in one executable place, because a taxonomy applied by
# hand is a taxonomy that drifts. The weekly drift job already creates a `drift`
# label as a fallback (.github/workflows/drift.yml, "Open or update the issue");
# this file is what keeps that label's name, colour and description from
# existing in two versions, and what makes the rest of the system — the labels
# the issue forms reference, the milestones the roadmap batches land in —
# reproducible on a fresh repository instead of remembered.
#
# What it does, idempotently:
#   - creates or updates every label the system uses (`gh label create --force`
#     updates colour and description in place);
#   - deletes the GitHub default labels the system replaced (native close
#     reasons cover duplicate/invalid/wontfix, and the project's word for
#     "wontfix" is a decline in the pack, with a reason, not a label);
#   - creates the six wave milestones from docs/roadmap.md if absent, and
#     leaves existing ones alone.
#
# The eighteen batch definitions in tools/issues/batches/ reference these
# labels and milestones by name; run this before turning them into issues.
#
# Usage: tools/issues/setup.sh [--dry-run]
#        --dry-run prints every change it would make and writes nothing.
# Exit:  0 done, 1 error.
set -euo pipefail

DRY_RUN=0
case "${1:-}" in
  --dry-run) DRY_RUN=1 ;;
  "") ;;
  *) echo "usage: $0 [--dry-run]" >&2; exit 1 ;;
esac

run() {
  if [ "$DRY_RUN" -eq 1 ]; then
    printf 'would run: %s\n' "$*"
  else
    "$@"
  fi
}

# ---------------------------------------------------------------------------
# Labels. Four axes, at most one label per axis on an issue:
#   nature    bug / enhancement / roadmap / drift  (plus the kept defaults
#             documentation and good first issue)
#   provider  scaleway / outscale / exoscale — absent means cross-cutting
#   layer     core / machine / conformance — only when a shared layer is the
#             actual subject; a pack change carries its provider label only
#   state     blocked — the blocker is named in the issue body
#
# `drift` must match what .github/workflows/drift.yml creates as a fallback:
# FBCA04, "The upstream API surface moved and needs triage". Change it here and
# there together or not at all.
# ---------------------------------------------------------------------------
while IFS='|' read -r name color desc; do
  [ -n "$name" ] || continue
  run gh label create "$name" --color "$color" --description "$desc" --force
done <<'LABELS'
bug|d73a4a|An official client noticed the difference
enhancement|a2eeef|An operation or behaviour a real client needs
roadmap|0052cc|A batch from docs/roadmap.md, closed by its stated evidence
drift|FBCA04|The upstream API surface moved and needs triage
blocked|e99695|Waits on another issue, named in the body
scaleway|6f42c1|The Scaleway pack (internal/providers/scaleway)
outscale|1d76db|The Outscale pack (internal/providers/outscale)
exoscale|cb2431|The Exoscale pack (internal/providers/exoscale)
core|6a737d|internal/core — the neutral kernel; no provider name enters it
machine|0e8a16|internal/core/machine and cloudinit — the layer that touches the operator's host
conformance|fef2c0|The real-client suites and fixtures under tools/conformance/
LABELS

# The defaults the system replaced. GitHub's native close reasons carry
# "duplicate" and "not planned"; `help wanted` solicits nobody on a
# one-maintainer project; `question` has no entry point with blank issues
# disabled — the config.yml contact links are that path. `bug`, `enhancement`,
# `documentation` and `good first issue` stay, redescribed above where needed.
existing_labels="$(gh label list --limit 100 --json name --jq '.[].name')"
for name in "help wanted" invalid duplicate wontfix question; do
  if printf '%s\n' "$existing_labels" | grep -Fxq "$name"; then
    run gh label delete "$name" --yes
  else
    echo "already absent: label '$name'"
  fi
done

# ---------------------------------------------------------------------------
# Milestones: the six waves of docs/roadmap.md ("The sequence"). Waves, not
# releases, because releases do not exist as a plan and the waves do: each has
# a stated exit condition, and a batch belongs to exactly one. The waves are an
# order, not a schedule, so no due dates.
# ---------------------------------------------------------------------------
existing_milestones="$(gh api 'repos/{owner}/{repo}/milestones?state=all&per_page=100' --jq '.[].title')"

milestone() {
  local title="$1" desc="$2"
  if printf '%s\n' "$existing_milestones" | grep -Fxq "$title"; then
    echo "already there: milestone '$title'"
  else
    run gh api -X POST 'repos/{owner}/{repo}/milestones' -f title="$title" -f description="$desc"
  fi
}

milestone "Wave 1 — triage" \
  "X-1, SW-1, OSC-1, EXO-1. No new route. Ends when tools/drift/gate.sh check returns 0 against the three new baselines and every untriaged column is a work list, not a wall."
milestone "Wave 2 — first Terraform proofs" \
  "OSC-2, EXO-2. Ends when terraform apply, an empty second plan and a clean destroy pass in conformance for both packs, and the Exoscale preview label comes off."
milestone "Wave 3 — the Scaleway golden image" \
  "SW-2, SW-3. Ends when a module declaring a server, an attached volume, a snapshot and an image built from it applies and destroys with no diff on re-plan."
milestone "Wave 4 — networks that route" \
  "OSC-3, SW-4, EXO-3. Isolation is asserted under OVN, skipped elsewhere, and never claimed without naming the mode."
milestone "Wave 5 — storage on the two starters" \
  "OSC-4, EXO-4. Every relation stored on one side, computed on the other; deletion rules exercised by each fixture's destroy."
milestone "Wave 6 — load balancing and gateways" \
  "SW-5, SW-6, OSC-5, EXO-5, EXO-6. Control plane first, waiters converge without timeouts, runtime backing only as a declared capability."
