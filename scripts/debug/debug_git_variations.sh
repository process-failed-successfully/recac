#!/bin/bash
source .env
REPO_URL="https://github.com/process-failed-successfully/recac-jira-e2e.git"

# Try 1: User:Token
echo "Testing User:Token..."
AUTH_URL_1="${REPO_URL/https:\/\//https:\/\/$GITHUB_EMAIL:$GITHUB_API_KEY@}"
git clone "$AUTH_URL_1" /tmp/recac-debug-clone-1 && echo "Success 1" || echo "Fail 1"

# Try 2: x-access-token:Token
echo "Testing x-access-token..."
AUTH_URL_2="${REPO_URL/https:\/\//https:\/\/x-access-token:$GITHUB_API_KEY@}"
git clone "$AUTH_URL_2" /tmp/recac-debug-clone-2 && echo "Success 2" || echo "Fail 2"

# Try 3: git:Token
echo "Testing git:Token..."
AUTH_URL_3="${REPO_URL/https:\/\//https:\/\/git:$GITHUB_API_KEY@}"
git clone "$AUTH_URL_3" /tmp/recac-debug-clone-3 && echo "Success 3" || echo "Fail 3"
