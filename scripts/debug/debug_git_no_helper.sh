#!/bin/bash
source .env
REPO_URL="https://github.com/process-failed-successfully/recac-jira-e2e.git"
AUTH_URL="${REPO_URL/https:\/\//https:\/\/$GITHUB_API_KEY@}"

echo "Testing Clone with credential.helper disabled..."
git -c credential.helper= clone "$AUTH_URL" /tmp/recac-debug-clone-no-helper
if [ $? -eq 0 ]; then
    echo "Success: Disabling credential helper worked!"
    rm -rf /tmp/recac-debug-clone-no-helper
else
    echo "Fail: Still 403"
fi
