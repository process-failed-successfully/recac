#!/bin/bash
# cleanup_branches.sh - Deletes remote branches older than 1 day
# Usage: ./scripts/cleanup_branches.sh [--dry-run]

set -e

REPO="process-failed-successfully/recac"
PROTECTED_BRANCHES="^(main|master|develop|staging|production)$"
DAYS_OLD=1
DRY_RUN=false

if [[ "$1" == "--dry-run" ]]; then
    DRY_RUN=true
    echo "--- DRY RUN MODE: No branches will be deleted ---"
fi

# Calculate the cutoff timestamp in UTC
# Uses GNU date (standard on Linux)
CUTOFF=$(date -u -d "$DAYS_OLD days ago" +"%Y-%m-%dT%H:%M:%SZ")

echo "Cleaning up branches in $REPO..."
echo "Target: Older than $DAYS_OLD day(s) (Committed before $CUTOFF)"
echo "Protecting: $PROTECTED_BRANCHES"
echo "---------------------------------------------------------------"

# Fetch branches and their last commit dates using GraphQL
# Filtering happens in jq:
# 1. Select branches committed before the cutoff
# 2. Filter out protected branches using a regex
branches_to_delete=$(gh api graphql --paginate -f query='
  query($name: String!, $owner: String!, $endCursor: String) {
    repository(name: $name, owner: $owner) {
      refs(refPrefix: "refs/heads/", first: 100, after: $endCursor) {
        nodes {
          name
          target {
            ... on Commit {
              committedDate
            }
          }
        }
        pageInfo {
          hasNextPage
          endCursor
        }
      }
    }
  }' -f owner="${REPO%/*}" -f name="${REPO#*/}" \
  --jq ".data.repository.refs.nodes[] | select(.target.committedDate < \"$CUTOFF\") | .name" | grep -vE "$PROTECTED_BRANCHES" || true)

if [ -z "$branches_to_delete" ]; then
    echo "No stale branches found."
    exit 0
fi

echo "Stale branches identified:"
echo "$branches_to_delete" | nl
echo ""

for branch in $branches_to_delete; do
    if [ "$DRY_RUN" = true ]; then
        echo "[DRY-RUN] Would delete: $branch"
    else
        echo "Deleting: $branch ..."
        gh api -X DELETE "repos/$REPO/git/refs/heads/$branch" --silent
    fi
done

echo "---------------------------------------------------------------"
echo "Done."
