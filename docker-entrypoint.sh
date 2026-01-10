#!/bin/sh
set -e

# If GITHUB_API_KEY is provided, configure git to use it for authentication
if [ -n "$GITHUB_API_KEY" ]; then
    git config --global url."https://x-access-token:${GITHUB_API_KEY}@github.com/".insteadOf "https://github.com/"
    git config --global url."https://x-access-token:${GITHUB_API_KEY}@github.com/".insteadOf "git@github.com:"
fi

# Execute the command passed to the entrypoint
exec "$@"
