#!/bin/bash
set -e

# If GITHUB_API_KEY is set, configure Git to use it for HTTPS authentication
if [ -n "$GITHUB_API_KEY" ]; then
  git config --global url."https://x-access-token:${GITHUB_API_KEY}@github.com/".insteadOf "https://github.com/"
  git config --global url."https://github.com/".insteadOf "git@github.com:"
  git config --global url."https://".insteadOf "ssh://"
fi

# Execute the command passed to the entrypoint
exec "$@"
