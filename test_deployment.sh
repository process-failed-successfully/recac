#!/bin/bash

# Test script for operator deployment

set -e

echo "Running operator deployment tests..."

# Test 1: Check if manifests exist
echo "Test 1: Checking operator manifests..."
if [ -f "operator/manifests/deployment.yaml" ] && \
   [ -f "operator/manifests/service_account.yaml" ] && \
   [ -f "operator/manifests/role.yaml" ] && \
   [ -f "operator/manifests/role_binding.yaml" ] && \
   [ -f "operator/manifests/namespace.yaml" ]; then
    echo "✓ All required manifests exist"
else
    echo "✗ Missing required manifests"
    exit 1
fi

# Test 2: Check if kustomization file exists
echo "Test 2: Checking kustomization file..."
if [ -f "operator/manifests/kustomization.yaml" ]; then
    echo "✓ Kustomization file exists"
else
    echo "✗ Kustomization file missing"
    exit 1
fi

# Test 3: Validate YAML syntax
echo "Test 3: Validating YAML syntax..."
for file in operator/manifests/*.yaml; do
    if [ -f "$file" ]; then
        if kubectl apply --dry-run=client -f "$file" >/dev/null 2>&1; then
            echo "✓ $file is valid"
        else
            echo "✗ $file has invalid syntax"
            exit 1
        fi
    fi
done

# Test 4: Check if operator can be built
echo "Test 4: Checking operator build..."
if [ -f "cmd/operator/main.go" ]; then
    echo "✓ Operator source code exists"
    if go build -o /tmp/test-operator cmd/operator/main.go >/dev/null 2>&1; then
        echo "✓ Operator builds successfully"
        rm -f /tmp/test-operator
    else
        echo "✗ Operator build failed"
        exit 1
    fi
else
    echo "✗ Operator source code missing"
    exit 1
fi

echo "All deployment tests passed!"
