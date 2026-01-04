#!/bin/bash

# Test script for service account configuration

echo "Testing service account configuration..."

# Test 1: File exists
if [ ! -f "service-account.yaml" ]; then
    echo "FAIL: service-account.yaml does not exist"
    exit 1
fi

# Test 2: Valid YAML
if ! python3 -c "import yaml; yaml.safe_load_all(open('service-account.yaml'))" 2>/dev/null; then
    echo "FAIL: Invalid YAML syntax"
    exit 1
fi

# Test 3: ServiceAccount defined
if ! grep -q "kind: ServiceAccount" service-account.yaml; then
    echo "FAIL: ServiceAccount not defined"
    exit 1
fi

# Test 4: Role defined
if ! grep -q "kind: Role" service-account.yaml; then
    echo "FAIL: Role not defined"
    exit 1
fi

# Test 5: RoleBinding defined
if ! grep -q "kind: RoleBinding" service-account.yaml; then
    echo "FAIL: RoleBinding not defined"
    exit 1
fi

# Test 6: Correct permissions
if ! grep -q "resources:.*pods" service-account.yaml; then
    echo "FAIL: Pods permissions not defined"
    exit 1
fi

echo "PASS: All service account configuration tests passed"
