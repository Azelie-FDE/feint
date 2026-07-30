#!/usr/bin/env bash
# Creates the roadmap batch issues from tools/issues/batches/*.md.
#
# The batch definitions are versioned next to this script rather than typed into
# GitHub, for the same reason the coverage baselines are versioned: a plan that
# lives only in a web UI cannot be reviewed in a pull request, and cannot be
# recreated if the repository moves.
#
# Idempotent by title: an issue whose exact title already exists is left alone,
# so a rerun after adding a batch creates only what is missing. Nothing is ever
# edited or closed here — a batch whose body changed is updated by hand, because
# a script that rewrites issue bodies would silently discard the discussion a
# maintainer added to them.
#
# Creation order is the sequence of docs/roadmap.md: by wave, X-* first inside a
# wave, then alphabetical. That order is what makes the dependency links work in
# one pass — a batch is always created after the batches it waits on, so their
# issue numbers are known by the time its "Blocked by" line is written.
#
# Usage: tools/issues/create.sh [--dry-run]
# Exit:  0 done, 1 error.
set -euo pipefail

DRY_RUN=0
case "${1:-}" in
  --dry-run) DRY_RUN=1 ;;
  "") ;;
  *) echo "usage: $0 [--dry-run]" >&2; exit 1 ;;
esac

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/batches"
[ -d "$DIR" ] || { echo "no batch definitions at $DIR" >&2; exit 1; }

command -v gh >/dev/null 2>&1 || { echo "gh is not installed" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq is not installed" >&2; exit 1; }

# field <key> <file> — one front matter value, quotes stripped.
field() {
  awk -v k="$1" '
    NR > 1 && /^---$/ { exit }
    $0 ~ "^" k ": " { sub("^" k ": ", ""); gsub(/^"|"$/, ""); print; exit }
  ' "$2"
}

# body <file> — everything after the closing front matter delimiter.
body() {
  awk '/^---$/ { seen++; if (seen == 2) { found = 1; next } } found { print }' "$1"
}

# Sort key: wave number, then X-* before the rest, then the identifier.
ordered() {
  local file id milestone wave rank
  for file in "$DIR"/*.md; do
    id="$(field id "$file")"
    milestone="$(field milestone "$file")"
    wave="$(printf '%s' "$milestone" | sed -n 's/^Wave \([0-9]\).*/\1/p')"
    [ -n "$wave" ] || wave=9
    case "$id" in
      X-*) rank=0 ;;
      *) rank=1 ;;
    esac
    printf '%s|%s|%s|%s\n' "$wave" "$rank" "$id" "$file"
  done | sort -t'|' -k1,1n -k2,2n -k3,3
}

echo "reading batch definitions from $DIR"
existing="$(gh issue list --state all --limit 200 --json number,title)"

declare -A number_of
created=0
skipped=0

while IFS='|' read -r _wave _rank id file; do
  title="$(field title "$file")"
  milestone="$(field milestone "$file")"
  after="$(field after "$file")"

  found="$(printf '%s' "$existing" | jq -r --arg t "$title" \
    'map(select(.title == $t)) | if length > 0 then .[0].number else "" end')"
  if [ -n "$found" ]; then
    echo "  = $id already exists as #$found"
    number_of["$id"]="$found"
    skipped=$((skipped + 1))
    continue
  fi

  # The dependency line is appended rather than woven into the prose: the body
  # states the dependency in words for a reader, and this gives GitHub the cross
  # reference that makes it navigable.
  text="$(body "$file")"
  links=""
  if [ -n "$after" ]; then
    for dep in ${after//,/ }; do
      dep="${dep// /}"
      if [ -n "${number_of[$dep]:-}" ]; then
        links="$links #${number_of[$dep]} ($dep)"
      else
        # Only possible if the creation order stops matching the dependencies.
        echo "  ! $id waits on $dep, which has no issue yet — link it by hand" >&2
        links="$links $dep"
      fi
    done
    text="$text"$'\n'"---"$'\n\n'"Blocked by:${links}."
  fi

  # Split on the comma in two steps: `${$(...)//,/ }` is zsh, not bash, and this
  # script is run by bash.
  labels_raw="$(field labels "$file")"
  args=(--title "$title" --body "$text" --milestone "$milestone")
  for label in ${labels_raw//,/ }; do
    args+=(--label "${label// /}")
  done

  if [ "$DRY_RUN" -eq 1 ]; then
    echo "  + would create $id — $title"
    printf '      milestone: %s, labels: %s, blocked by:%s\n' \
      "$milestone" "$(field labels "$file")" "${links:-" none"}"
    number_of["$id"]="?"
    created=$((created + 1))
    continue
  fi

  url="$(gh issue create "${args[@]}")"
  num="${url##*/}"
  echo "  + $id — #$num $title"
  number_of["$id"]="$num"
  created=$((created + 1))
done < <(ordered)

if [ "$DRY_RUN" -eq 1 ]; then
  echo "dry run: $created would be created, $skipped already there"
else
  echo "done: $created created, $skipped already there"
fi
