#!/usr/bin/env bash
# Mirrors the `after:` front matter of tools/issues/batches/*.md into GitHub's
# native issue dependencies (blocked-by relationships).
#
# The issue bodies already state their blocker in prose ("Blocked by: #N"), but
# prose is for readers: nothing can filter on it, and nothing notices when the
# blocker closes. The native relationship is the machine-readable copy — the
# issue page shows it, the REST API counts how many blockers are still open,
# and .github/workflows/unblock.yml reads that count to maintain the `blocked`
# label the project board filters on.
#
# Idempotent: a relationship that already exists is left alone (GitHub answers
# 422 on a duplicate, which is treated as "already there"). Nothing is ever
# removed here — dropping a dependency is a decision, taken by hand.
#
# Usage: tools/issues/dependencies.sh [--dry-run]
# Exit:  0 done, 1 error.
set -euo pipefail

DRY_RUN=0
case "${1:-}" in
  --dry-run) DRY_RUN=1 ;;
  "") ;;
  *) echo "usage: $0 [--dry-run]" >&2; exit 1 ;;
esac

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/batches"
REPO="${FEINT_PROJECT_REPO:-stephrobert/feint}"

command -v gh >/dev/null 2>&1 || { echo "gh is not installed" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq is not installed" >&2; exit 1; }
[ -d "$DIR" ] || { echo "no batch definitions at $DIR" >&2; exit 1; }

# field <key> <file> — one front matter value, quotes stripped.
field() {
  awk -v k="$1" '
    NR > 1 && /^---$/ { exit }
    $0 ~ "^" k ": " { sub("^" k ": ", ""); gsub(/^"|"$/, ""); print; exit }
  ' "$2"
}

# The issues are found by title, never by remembered number: the batch id is the
# title's prefix, so "SW-2: …" belongs to SW-2 whatever number it got. Listed
# through REST rather than `gh issue list` because the dependency endpoint wants
# the numeric issue id, which the GraphQL-backed listing does not carry.
issues="$(gh api "repos/$REPO/issues?state=all&labels=roadmap&per_page=100" \
  --jq '[.[] | {number, title, id}]')"

number_for() {
  printf '%s' "$issues" | jq -r --arg id "$1" \
    'map(select(.title | startswith($id + ":"))) | if length == 1 then .[0].number else "" end'
}

created=0
skipped=0
for file in "$DIR"/*.md; do
  id="$(field id "$file")"
  after="$(field after "$file")"
  [ -n "$after" ] || continue

  num="$(number_for "$id")"
  [ -n "$num" ] || { echo "! no single issue titled \"$id: …\" in $REPO" >&2; exit 1; }

  for dep in ${after//,/ }; do
    dep="${dep// /}"
    dep_num="$(number_for "$dep")"
    [ -n "$dep_num" ] || { echo "! $id waits on $dep, which has no issue" >&2; exit 1; }
    dep_id="$(printf '%s' "$issues" | jq -r --argjson n "$dep_num" \
      'map(select(.number == $n)) | .[0].id')"

    if [ "$DRY_RUN" -eq 1 ]; then
      echo "  + would mark #$num ($id) blocked by #$dep_num ($dep)"
      created=$((created + 1))
      continue
    fi

    # 201 on creation, 422 when the relationship already exists.
    if out="$(gh api --method POST "repos/$REPO/issues/$num/dependencies/blocked_by" \
      -F "issue_id=$dep_id" 2>&1)"; then
      echo "  + #$num ($id) blocked by #$dep_num ($dep)"
      created=$((created + 1))
    elif printf '%s' "$out" | grep -q "HTTP 422"; then
      echo "  = #$num ($id) already blocked by #$dep_num ($dep)"
      skipped=$((skipped + 1))
    else
      echo "! marking #$num blocked by #$dep_num failed:" >&2
      printf '%s\n' "$out" >&2
      exit 1
    fi
  done
done

echo "done: $created created, $skipped already there"
