#!/bin/bash
set -e

REPO_URL="https://github.com/process-failed-successfully/recac-jira-e2e.git"
TEMP_DIR=$(mktemp -d)

echo "Cloning $REPO_URL to $TEMP_DIR..."
# Use the GH token from env
git clone "https://x-access-token:$GITHUB_API_KEY@github.com/process-failed-successfully/recac-jira-e2e.git" "$TEMP_DIR"

cd "$TEMP_DIR"

if [ ! -f "app_spec.txt" ]; then
    echo "Creating app_spec.txt..."
    echo "Recac E2E Test Repository" > app_spec.txt
    git add app_spec.txt
    git commit -m "chore: add app_spec.txt for e2e tests"
    git push
    echo "Pushed app_spec.txt"
else
    echo "app_spec.txt already exists."
fi

rm -rf "$TEMP_DIR"
echo "Done."
