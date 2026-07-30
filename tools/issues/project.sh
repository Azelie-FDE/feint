#!/usr/bin/env bash
# Builds the GitHub Project that steers the roadmap: the board, its fields, its
# views, and the eighteen batch issues as items — so the sequence can be driven
# from one screen instead of re-derived from eighteen issue bodies.
#
# What the board adds that the issues do not already carry: a Status that is not
# "open or closed", and the batch's relative size, which lives in three separate
# docs/roadmap-*-iaas.md files and meets itself only here. Everything else is
# native and deliberately not duplicated: the wave is the milestone, the
# provider is a label, and readiness is the `blocked` label that
# tools/issues/sync-blocked.sh maintains from the native issue dependencies.
#
# What the API can and cannot do here, measured on 2026-07-30 by introspecting
# the GraphQL schema: createProjectV2View takes a name and a layout,
# updateProjectV2View also takes the filter string — so the four views and
# their filters are created below. Neither mutation carries group-by, sort or
# slice-by, so those are configured once in the web UI; this script prints the
# exact remaining clicks when it ends.
#
# Idempotent: the project is found by exact title, fields and views by name,
# and a field value is only ever written where none is set — once a human has
# moved a card to In Progress, a script that re-sorts it is a script the human
# stops running.
#
# Requires the `project` OAuth scope, which the default `gh` login lacks:
#   gh auth refresh -s project
#
# Usage: tools/issues/project.sh [--dry-run]
# Exit:  0 done, 1 error.
set -euo pipefail

DRY_RUN=0
case "${1:-}" in
  --dry-run) DRY_RUN=1 ;;
  "") ;;
  *) echo "usage: $0 [--dry-run]" >&2; exit 1 ;;
esac

OWNER="${FEINT_PROJECT_OWNER:-stephrobert}"
REPO="${FEINT_PROJECT_REPO:-stephrobert/feint}"
TITLE="${FEINT_PROJECT_TITLE:-feint roadmap}"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/batches"

command -v gh >/dev/null 2>&1 || { echo "gh is not installed" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq is not installed" >&2; exit 1; }
[ -d "$DIR" ] || { echo "no batch definitions at $DIR" >&2; exit 1; }

gh project list --owner "$OWNER" --format json >/dev/null 2>&1 || {
  echo "gh lacks the project scope; run: gh auth refresh -s project" >&2
  exit 1
}

run() {
  if [ "$DRY_RUN" -eq 1 ]; then
    echo "would run: $*"
    return 0
  fi
  "$@"
}

# field_of <key> <file> — one front matter value, quotes stripped.
field_of() {
  awk -v k="$1" '
    NR > 1 && /^---$/ { exit }
    $0 ~ "^" k ": " { sub("^" k ": ", ""); gsub(/^"|"$/, ""); print; exit }
  ' "$2"
}

# ---------------------------------------------------------------- the project

# Found by exact title: this owner has unrelated projects, and a substring
# match could land the roadmap on one of them.
number="$(gh project list --owner "$OWNER" --format json \
  | jq -r --arg t "$TITLE" '.projects[] | select(.title == $t) | .number' | head -1)"

if [ -z "$number" ]; then
  if [ "$DRY_RUN" -eq 1 ]; then
    echo "would run: gh project create --owner $OWNER --title \"$TITLE\""
    echo "dry run stops here: everything below needs the project number"
    exit 0
  fi
  number="$(gh project create --owner "$OWNER" --title "$TITLE" --format json | jq -r .number)"
  echo "created project #$number ($TITLE)"
else
  echo "project #$number ($TITLE) already exists"
fi

project_id="$(gh project view "$number" --owner "$OWNER" --format json | jq -r .id)"

# Linked to the repository so the board is one click from the Projects tab.
# shellcheck disable=SC2016 # $t is a jq variable, bound by --arg
linked="$(gh api "repos/$REPO/projects_v2" --jq \
  --arg t "$TITLE" '[.[] | select(.title == $t)] | length' 2>/dev/null || echo 0)"
if [ "$linked" -eq 0 ]; then
  run gh project link "$number" --owner "$OWNER" --repo "$REPO"
else
  echo "already linked to $REPO"
fi

# ------------------------------------------------------------------ the field

# One custom field. Size is the arbitration datum when several batches are
# ready at once, and the one figure the roadmap documents carry that nothing
# native holds. Status needs no creation: every project is born with it.
fields="$(gh project field-list "$number" --owner "$OWNER" --format json)"
if [ -z "$(printf '%s' "$fields" | jq -r '.fields[] | select(.name == "Size") | .id')" ]; then
  run gh project field-create "$number" --owner "$OWNER" \
    --name Size --data-type SINGLE_SELECT --single-select-options "S,M,L"
  fields="$(gh project field-list "$number" --owner "$OWNER" --format json)"
else
  echo "field Size already there"
fi

field_id() { printf '%s' "$fields" | jq -r --arg n "$1" '.fields[] | select(.name == $n) | .id'; }
option_id() {
  printf '%s' "$fields" | jq -r --arg f "$1" --arg o "$2" \
    '.fields[] | select(.name == $f) | .options[] | select(.name == $o) | .id'
}

# ------------------------------------------------------------------ the items

# Every roadmap issue goes on the board, closed ones included: the Waves view
# reads progress, and progress is made of closed items. The label is the
# filter rather than a list of numbers, so a batch added later is picked up by
# a rerun.
mapfile -t urls < <(gh issue list --repo "$REPO" --label roadmap --state all \
  --limit 200 --json url --jq '.[].url')
[ "${#urls[@]}" -gt 0 ] || { echo "no issue carries the roadmap label" >&2; exit 1; }

items() { gh project item-list "$number" --owner "$OWNER" --limit 200 --format json; }

on_board="$(items | jq -r '.items[] | .content.url // empty')"
added=0
for url in "${urls[@]}"; do
  if printf '%s\n' "$on_board" | grep -qxF "$url"; then
    continue
  fi
  run gh project item-add "$number" --owner "$OWNER" --url "$url" >/dev/null
  echo "  + added ${url##*/} to the board"
  added=$((added + 1))
done
echo "items: $added added, $(( ${#urls[@]} - added )) already on the board"

# ----------------------------------------------------------- the field values

# Sizes come from the batch front matter, matched by the title's "ID:" prefix.
declare -A size_of
for file in "$DIR"/*.md; do
  size_of["$(field_of id "$file")"]="$(field_of size "$file")"
done

# Only empty fields are written. `gh project item-list` flattens field values
# into each item, so an unset field is simply an absent key. The rows are read
# into an array first, and the inner gh calls take stdin from /dev/null: a
# command that drains the loop's stdin silently eats the remaining rows, and
# the first run of this script lost exactly one item that way.
mapfile -t rows < <(items | jq -r '.items[]
  | [.id, .content.title, (.status // "null"), (.size // "null"),
     (if .content.state then .content.state else "null" end)] | @tsv')
set_count=0
for row in "${rows[@]}"; do
  IFS=$'\t' read -r item_id title status size state <<<"$row"
  id="${title%%:*}"
  if [ "$status" = "null" ]; then
    to="Todo"
    [ "$state" = "CLOSED" ] && to="Done"
    run gh project item-edit --id "$item_id" --project-id "$project_id" \
      --field-id "$(field_id Status)" \
      --single-select-option-id "$(option_id Status "$to")" >/dev/null </dev/null
    echo "  + $id: Status = $to"
    set_count=$((set_count + 1))
  fi
  if [ "$size" = "null" ] && [ -n "${size_of[$id]:-}" ]; then
    run gh project item-edit --id "$item_id" --project-id "$project_id" \
      --field-id "$(field_id Size)" \
      --single-select-option-id "$(option_id Size "${size_of[$id]}")" >/dev/null </dev/null
    echo "  + $id: Size = ${size_of[$id]}"
    set_count=$((set_count + 1))
  fi
done
echo "field values: $set_count written, the rest left as they are"

# ------------------------------------------------------------------ the views

views_json() {
  # shellcheck disable=SC2016 # $id is a GraphQL variable, bound by -F
  gh api graphql -f query='
    query($id: ID!) { node(id: $id) { ... on ProjectV2 {
      views(first: 20) { nodes { id name layout filter } } } } }' \
    -F id="$project_id" --jq '.data.node.views.nodes'
}

# ensure_view <name> <layout> <filter>
ensure_view() {
  local name="$1" layout="$2" filter="$3" existing view_id current
  existing="$(views_json)"
  view_id="$(printf '%s' "$existing" | jq -r --arg n "$name" \
    '.[] | select(.name == $n) | .id' | head -1)"

  # The project is born with a table called "View 1"; the first table asked of
  # this function takes its place instead of leaving a dead tab.
  if [ -z "$view_id" ] && [ "$layout" = "TABLE_LAYOUT" ]; then
    view_id="$(printf '%s' "$existing" | jq -r \
      '.[] | select(.name == "View 1") | .id' | head -1)"
  fi

  if [ -z "$view_id" ]; then
    if [ "$DRY_RUN" -eq 1 ]; then
      echo "would create view $name ($layout, filter: ${filter:-none})"
      return 0
    fi
    # shellcheck disable=SC2016 # $p, $n, $l are GraphQL variables, bound by -F
    view_id="$(gh api graphql -f query='
      mutation($p: ID!, $n: String!, $l: ProjectV2ViewLayout!) {
        createProjectV2View(input: {projectId: $p, name: $n, layout: $l}) {
          projectV2View { id } } }' \
      -F p="$project_id" -F n="$name" -F l="$layout" \
      --jq '.data.createProjectV2View.projectV2View.id')"
    echo "  + view $name created"
  fi

  current="$(printf '%s' "$existing" | jq -r --arg i "$view_id" \
    '.[] | select(.id == $i) | [.name, (.filter // "")] | @tsv')"
  if [ "$current" = "$(printf '%s\t%s' "$name" "$filter")" ]; then
    echo "  = view $name already as declared"
    return 0
  fi
  if [ "$DRY_RUN" -eq 1 ]; then
    echo "would set view $name filter to: ${filter:-none}"
    return 0
  fi
  # shellcheck disable=SC2016 # $v, $n, $f are GraphQL variables, bound by -F
  gh api graphql -f query='
    mutation($v: ID!, $n: String!, $f: String!) {
      updateProjectV2View(input: {viewId: $v, name: $n, filter: $f}) {
        projectV2View { id } } }' \
    -F v="$view_id" -F n="$name" -F f="$filter" >/dev/null
  echo "  = view $name: filter set to ${filter:-none}"
}

# The four views, argued in docs/project.md. Filters use only labels, status
# and state — the whole filterable space, since the filter language has no
# qualifier for native dependencies (measured, see sync-blocked.sh).
ensure_view "Waves"          TABLE_LAYOUT ""
ensure_view "Now"            BOARD_LAYOUT "is:open -label:blocked"
ensure_view "Ready to start" TABLE_LAYOUT "is:open status:Todo -label:blocked"
ensure_view "Providers"      TABLE_LAYOUT ""

echo
echo "done: https://github.com/users/$OWNER/projects/$number"
echo
echo "Three settings the API cannot write (no group-by, sort or slice-by on"
echo "createProjectV2View/updateProjectV2View — measured), to click once:"
echo "  Waves           group by Milestone, sort by Title ascending"
echo "  Ready to start  sort by Milestone ascending"
echo "  Providers       slice by Labels"
