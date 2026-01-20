#!/bin/bash
set -e

# Build the runner tool
echo "Building recac-e2e..."
go build -o recac-e2e ./cmd/e2e

# Define state file
STATE_FILE="e2e_state_$(date +%s).json"

# Cleanup function
cleanup() {
    if [ "$SKIP_CLEANUP" == "true" ]; then
        echo "Skipping cleanup as requested. State file: $STATE_FILE"
        return
    fi
    echo "Cleaning up..."
    ./recac-e2e cleanup -state-file "$STATE_FILE" || true
    rm -f "$STATE_FILE"
}

# Trap cleanup
trap cleanup EXIT

# 1. Setup
echo "Running Setup..."
./recac-e2e setup -state-file "$STATE_FILE" "$@"

# 2. Deploy
echo "Running Deploy..."
./recac-e2e deploy -state-file "$STATE_FILE"

# 3. Wait
echo "Running Wait..."
./recac-e2e wait -state-file "$STATE_FILE"

# 4. Verify
echo "Running Verify..."
./recac-e2e verify -state-file "$STATE_FILE"

echo "CI Simulation Complete!"
