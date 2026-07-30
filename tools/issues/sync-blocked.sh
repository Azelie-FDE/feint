#!/usr/bin/env bash
# Keeps the `blocked` label equal to the truth of the native issue dependencies:
# an open issue wears it while at least one of the issues blocking it is open,
# and loses it when the last one closes.
#
# Why a label at all, when the dependency is native: measured on 2026-07-30, the
# project-board filter language has no qualifier for dependencies — `label:` and
# `status:` are the whole filterable space (the introspected schema exposes
# Issue.blockedBy, and the Projects filter docs list no is:blocked). So the
# board's "Ready" view can only see readiness through a label, and a label
# nobody maintains is a comment. This script is the maintenance;
# .github/workflows/unblock.yml runs it whenever an issue closes or reopens.
#
# Scope: only issues that have at least one dependency, open or closed, are
# touched. An issue somebody labelled `blocked` by hand, blocker named in the
# body per CONTRIBUTING.md, has no dependency records and is left alone.
#
# The open-blocker count is computed from the blocker list itself, never from
# the summary counters, so nothing here rests on an undocumented aggregation.
#
# Usage: tools/issues/sync-blocked.sh [--dry-run]
# Exit:  0 done (labels now match the dependencies), 1 error.
set -euo pipefail

DRY_RUN=0
case "${1:-}" in
  --dry-run) DRY_RUN=1 ;;
  "") ;;
  *) echo "usage: $0 [--dry-run]" >&2; exit 1 ;;
esac

REPO="${FEINT_PROJECT_REPO:-stephrobert/feint}"
LABEL="blocked"

command -v gh >/dev/null 2>&1 || { echo "gh is not installed" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq is not installed" >&2; exit 1; }

# Every open issue, with its dependency summary and current labels. The summary
# is used only to decide whether any dependency exists at all; the verdict on
# each candidate comes from listing its blockers.
candidates="$(gh api "repos/$REPO/issues?state=open&per_page=100" --paginate \
  --jq '[.[] | select(.pull_request | not)
    | select((.issue_dependencies_summary.total_blocked_by // 0) > 0
             or (.issue_dependencies_summary.blocked_by // 0) > 0)
    | {number, labelled: ([.labels[].name] | contains(["blocked"]))}]')"

count="$(printf '%s' "$candidates" | jq length)"
echo "$count open issue(s) carry at least one dependency"

added=0
removed=0
unchanged=0
while IFS=$'\t' read -r number labelled; do
  open_blockers="$(gh api "repos/$REPO/issues/$number/dependencies/blocked_by?per_page=100" \
    --paginate --jq '[.[] | select(.state == "open")] | length')"

  if [ "$open_blockers" -gt 0 ] && [ "$labelled" = "false" ]; then
    if [ "$DRY_RUN" -eq 1 ]; then
      echo "  + would label #$number $LABEL ($open_blockers open blocker(s))"
    else
      gh issue edit "$number" --repo "$REPO" --add-label "$LABEL" >/dev/null
      echo "  + #$number labelled $LABEL ($open_blockers open blocker(s))"
    fi
    added=$((added + 1))
  elif [ "$open_blockers" -eq 0 ] && [ "$labelled" = "true" ]; then
    if [ "$DRY_RUN" -eq 1 ]; then
      echo "  - would unlabel #$number: every blocker is closed"
    else
      gh issue edit "$number" --repo "$REPO" --remove-label "$LABEL" >/dev/null
      echo "  - #$number unlabelled: every blocker is closed"
    fi
    removed=$((removed + 1))
  else
    unchanged=$((unchanged + 1))
  fi
done < <(printf '%s' "$candidates" | jq -r '.[] | [.number, .labelled] | @tsv')

echo "done: $added labelled, $removed unlabelled, $unchanged already right"
