#!/bin/bash

# Verify Code Review and Merge Process

# Step 1: Check if there are any uncommitted changes
echo "Step 1: Checking for uncommitted changes..."
if [ -z "$(git status --porcelain)" ]; then
    echo "✓ No uncommitted changes"
else
    echo "✗ There are uncommitted changes"
    git status --porcelain
    exit 1
fi

# Step 2: Check if the current branch is up to date with the main branch
echo "Step 2: Checking if the current branch is up to date with the main branch..."
if git diff --quiet origin/main; then
    echo "✓ Current branch is up to date with main"
else
    echo "✗ Current branch is not up to date with main"
    exit 1
fi

# Step 3: Verify that all features in feature_list.json have passes: true
echo "Step 3: Verifying all features have passes: true..."
if python3 -c "
import json
with open('feature_list.json', 'r') as f:
    features = json.load(f)['features']
    for feature in features:
        if not feature.get('passes', False):
            print(f'Feature {feature[\"id\"]} does not pass')
            exit(1)
    print('All features pass')
"; then
    echo "✓ All features pass"
else
    echo "✗ Some features do not pass"
    exit 1
fi

echo "All code review and merge checks passed!"
