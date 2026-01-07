#!/bin/sh
set -e

# Usage: ./test_sharded.sh <SHARD_INDEX> <TOTAL_SHARDS>
# Example: ./test_sharded.sh 1 4

SHARD_INDEX=$1
TOTAL_SHARDS=$2

if [ -z "$SHARD_INDEX" ] || [ -z "$TOTAL_SHARDS" ]; then
  echo "Usage: $0 <SHARD_INDEX> <TOTAL_SHARDS>"
  exit 1
fi

echo "Running tests for shard $SHARD_INDEX of $TOTAL_SHARDS"

# List all packages, filter by shard index using round-robin
PACKAGES=$(go list -buildvcs=false ./... | awk "NR % $TOTAL_SHARDS == ($SHARD_INDEX - 1)")

if [ -z "$PACKAGES" ]; then
  echo "No packages found for this shard."
  exit 0
fi

echo "Testing packages:"
echo "$PACKAGES"

# Run tests for selected packages
# We use xargs to pass all packages as arguments to a single go test command.
# This is more efficient and handles dependencies between packages in the same shard correctly.
echo "$PACKAGES" | xargs go test -buildvcs=false -v
