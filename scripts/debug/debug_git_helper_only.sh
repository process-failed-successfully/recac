#!/bin/bash
REPO_URL="https://github.com/process-failed-successfully/recac-jira-e2e.git"

echo "Testing Clone using only credential helper (unset GITHUB_API_KEY)..."
unset GITHUB_API_KEY
git clone "$REPO_URL" /tmp/recac-debug-clone-helper-only
if [ $? -eq 0 ]; then
    echo "Success: Credential helper worked!"
    rm -rf /tmp/recac-debug-clone-helper-only
else
    echo "Fail: Helper failed too"
fi
