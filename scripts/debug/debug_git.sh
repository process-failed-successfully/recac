#!/bin/bash
source .env
REPO_URL="https://github.com/process-failed-successfully/recac-jira-e2e.git"
AUTH_URL="${REPO_URL/https:\/\//https:\/\/$GITHUB_API_KEY@}"

echo "Testing single clone..."
git clone "$AUTH_URL" /tmp/recac-debug-clone
if [ $? -eq 0 ]; then
    echo "Clone successful!"
    rm -rf /tmp/recac-debug-clone
else
    echo "Clone failed!"
fi
