#!/bin/bash

echo "Testing Feature #1: User can initialize a new project with default configuration"

# Clean up any existing test directory
rm -rf myproject

# Step 1: Run recac init myproject
echo "Step 1: Running recac init myproject"
./recac init myproject
if [ $? -ne 0 ]; then
    echo "FAIL: recac init command failed"
    exit 1
fi

# Step 2: Verify directory is created
echo "Step 2: Verifying directory myproject is created"
if [ ! -d "myproject" ]; then
    echo "FAIL: Directory myproject not created"
    exit 1
fi

# Step 3: Verify app_spec.txt exists with default template
echo "Step 3: Verifying app_spec.txt"
if [ ! -f "myproject/app_spec.txt" ]; then
    echo "FAIL: app_spec.txt not found"
    exit 1
fi

# Check if it contains the expected header
if ! grep -q "# Application Specification" myproject/app_spec.txt; then
    echo "FAIL: app_spec.txt doesn't contain expected header"
    exit 1
fi

# Step 4: Verify feature_list.json exists with default template
echo "Step 4: Verifying feature_list.json"
if [ ! -f "myproject/feature_list.json" ]; then
    echo "FAIL: feature_list.json not found"
    exit 1
fi

# Check if it's valid JSON with tests array
if ! jq -e '.tests' myproject/feature_list.json > /dev/null 2>&1; then
    echo "FAIL: feature_list.json is not valid JSON or doesn't have tests field"
    exit 1
fi

# Step 5: Verify config.yaml exists with default settings
echo "Step 5: Verifying config.yaml"
if [ ! -f "myproject/config.yaml" ]; then
    echo "FAIL: config.yaml not found"
    exit 1
fi

# Check if it contains expected sections
if ! grep -q "agent:" myproject/config.yaml; then
    echo "FAIL: config.yaml doesn't contain agent section"
    exit 1
fi

echo "SUCCESS: All tests passed for Feature #1"
rm -rf myproject