#!/usr/bin/env bash
# The branch ruleset, applied and verified from the file that declares it.
#
# A JSON file describing a protection nobody applies is the defect this
# repository already found twice, in .poutine.yml and in .plumber.yaml: a
# configuration with no consumer reads as a control in force. So the file is
# never the claim — this script is, and it works in both directions:
#
#   apply   push .github/rulesets/main.json to the repository
#   check   fail (exit 2) when the live ruleset differs from the file
#
# Why a ruleset rather than classic branch protection, which is what most
# repositories reach for: rulesets are readable with the default GITHUB_TOKEN.
# Classic protection is not, so OpenSSF Scorecard's Branch-Protection check and
# plumber's branchMustBeProtected both need a privileged fine-grained PAT
# (Administration: read) stored as a secret. One fewer privileged credential to
# create, scope, store and rotate is worth more than the difference in features.
#
# Usage: tools/governance/ruleset.sh {apply|check|show} [owner/repo]
# Exit:  0 in agreement, 2 drifted, 1 could not tell.
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.." || exit 1

ACTION="${1:-check}"
FILE=".github/rulesets/main.json"

REPO="${2:-}"
if [ -z "$REPO" ]; then
  REPO="$(git remote get-url origin 2>/dev/null | sed -E 's#(git@github.com:|https://github.com/)##; s#\.git$##')"
fi
if [ -z "$REPO" ]; then
  echo "no repository: pass owner/repo, or add an origin remote" >&2
  exit 1
fi
if [ ! -f "$FILE" ]; then
  echo "missing $FILE" >&2
  exit 1
fi

need() { command -v "$1" >/dev/null 2>&1 || { echo "$1 is required" >&2; exit 1; }; }
need gh
need jq

# The comparable shape: the API answers with ids, timestamps and _links that the
# declaration cannot carry, so both sides are reduced to what was decided.
normalise() {
  jq -S '{
    name, target, enforcement,
    conditions,
    bypass_actors: (.bypass_actors // [] | map({actor_id, actor_type, bypass_mode}) | sort_by(.actor_id)),
    rules: (.rules // [] | map(select(.type != "creation")) | sort_by(.type) | map({
      type,
      parameters: (
        if .parameters == null then null
        else .parameters
          # Ordering of the required checks is not a decision.
          | (if has("required_status_checks")
             then .required_status_checks |= (map({context}) | sort_by(.context))
             else . end)
        end)
    }))
  }'
}

live_ruleset() {
  local id
  id="$(gh api "repos/${REPO}/rulesets" --jq '.[] | select(.target == "branch") | .id' 2>/dev/null | head -1)"
  [ -z "$id" ] && return 1
  gh api "repos/${REPO}/rulesets/${id}" 2>/dev/null
}

case "$ACTION" in
  apply)
    if live=$(live_ruleset); then
      id="$(printf '%s' "$live" | jq -r '.id')"
      echo "updating ruleset ${id} on ${REPO}"
      gh api --method PUT "repos/${REPO}/rulesets/${id}" --input "$FILE" >/dev/null || exit 1
    else
      echo "creating the ruleset on ${REPO}"
      gh api --method POST "repos/${REPO}/rulesets" --input "$FILE" >/dev/null || exit 1
    fi
    echo "applied. Verify with: $0 check"
    ;;

  show)
    live_ruleset | jq '.' || { echo "no branch ruleset on ${REPO}" >&2; exit 2; }
    ;;

  check)
    if ! live=$(live_ruleset); then
      echo "no branch ruleset on ${REPO}: main is unprotected" >&2
      echo "apply the declared one: $0 apply" >&2
      exit 2
    fi
    want="$(normalise < "$FILE")"
    got="$(printf '%s' "$live" | normalise)"
    if [ "$want" = "$got" ]; then
      echo "the ruleset on ${REPO} matches ${FILE}"
      exit 0
    fi
    echo "the ruleset on ${REPO} differs from ${FILE}:" >&2
    diff <(printf '%s\n' "$want") <(printf '%s\n' "$got") | head -40 >&2
    echo >&2
    echo "left is the file, right is what the repository enforces." >&2
    exit 2
    ;;

  *)
    echo "usage: $0 {apply|check|show} [owner/repo]" >&2
    exit 1
    ;;
esac
