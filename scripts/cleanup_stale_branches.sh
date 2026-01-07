#!/bin/bash
set -e

REPO_URL="https://github.com/process-failed-successfully/recac-jira-e2e.git"
TEMP_DIR=$(mktemp -d)

echo "Cloning $REPO_URL to $TEMP_DIR..."
# Use the GH token from env
git clone "https://x-access-token:$GITHUB_API_KEY@github.com/process-failed-successfully/recac-jira-e2e.git" "$TEMP_DIR"

cd "$TEMP_DIR"

echo "Deleting stale branches..."
git push origin --delete agent/direct-task || echo "Branch agent/direct-task does not exist"

rm -rf "$TEMP_DIR"
echo "Done."
