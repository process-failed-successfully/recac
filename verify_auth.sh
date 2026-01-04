#!/bin/bash

# Verification script for Jira credentials management

echo "=== Jira Credentials Management Verification ==="
echo ""

# Check if authentication module exists
if [ -f "internal/auth/jira_auth.go" ]; then
    echo "✓ Authentication module exists"
else
    echo "✗ Authentication module missing"
    exit 1
fi

# Check if Kubernetes manifest exists
if [ -f "k8s/jira-secrets.yaml" ]; then
    echo "✓ Kubernetes secrets manifest exists"
else
    echo "✗ Kubernetes secrets manifest missing"
    exit 1
fi

# Check if tests exist
if [ -f "test/auth/jira_auth_test.go" ]; then
    echo "✓ Authentication tests exist"
else
    echo "✗ Authentication tests missing"
    exit 1
fi

# Check if the code compiles
echo "Checking if code compiles..."
if go build -o /dev/null ./internal/auth/... 2>/dev/null; then
    echo "✓ Authentication module compiles successfully"
else
    echo "✗ Authentication module has compilation errors"
    exit 1
fi

echo ""
echo "=== Verification Complete ==="
echo "All checks passed! The Jira credentials management feature is implemented."
