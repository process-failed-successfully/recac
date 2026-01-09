#!/bin/bash
source .env
REPO_URL="https://github.com/process-failed-successfully/recac-jira-e2e.git"
EMAIL_ENCODED="${GITHUB_EMAIL/@/%40}"

echo "Testing EncodedEmail:Token..."
AUTH_URL="https://$EMAIL_ENCODED:$GITHUB_API_KEY@github.com/process-failed-successfully/recac-jira-e2e.git"
git clone "$AUTH_URL" /tmp/recac-debug-clone-email && echo "Success" || echo "Fail"
