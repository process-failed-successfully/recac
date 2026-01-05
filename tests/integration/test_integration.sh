#!/bin/bash

# Integration Test 1: Check if operator manifests exist
echo "Integration Test 1: Checking operator manifests..."
if [ -f "operator/manifests/deployment.yaml" ] && \
   [ -f "operator/manifests/service_account.yaml" ] && \
   [ -f "operator/manifests/role.yaml" ] && \
   [ -f "operator/manifests/role_binding.yaml" ] && \
   [ -f "operator/manifests/kustomization.yaml" ]; then
    echo "✓ All required operator manifests exist"
else
    echo "✗ Missing operator manifests"
    exit 1
fi

# Integration Test 2: Check if agent manifests exist
echo "Integration Test 2: Checking agent manifests..."
if [ -f "agent/manifests/deployment.yaml" ] && \
   [ -f "agent/manifests/service_account.yaml" ] && \
   [ -f "agent/manifests/role.yaml" ] && \
   [ -f "agent/manifests/role_binding.yaml" ] && \
   [ -f "agent/manifests/kustomization.yaml" ]; then
    echo "✓ All required agent manifests exist"
else
    echo "✗ Missing agent manifests"
    exit 1
fi

# Integration Test 3: Validate YAML syntax for operator manifests
echo "Integration Test 3: Validating YAML syntax for operator manifests..."
for file in operator/manifests/*.yaml; do
    if ! python3 -c "import yaml; yaml.safe_load(open('$file'))" 2>/dev/null; then
        echo "✗ $file has invalid syntax"
        exit 1
    fi
done
echo "✓ All operator YAML files have valid syntax"

# Integration Test 4: Validate YAML syntax for agent manifests
echo "Integration Test 4: Validating YAML syntax for agent manifests..."
for file in agent/manifests/*.yaml; do
    if ! python3 -c "import yaml; yaml.safe_load(open('$file'))" 2>/dev/null; then
        echo "✗ $file has invalid syntax"
        exit 1
    fi
done
echo "✓ All agent YAML files have valid syntax"

# Integration Test 5: Check if operator and agent can communicate
echo "Integration Test 5: Checking operator and agent communication..."
if grep -q "OPERATOR_ENDPOINT" agent/manifests/deployment.yaml; then
    echo "✓ Agent is configured to communicate with the operator"
else
    echo "✗ Agent is not configured to communicate with the operator"
    exit 1
fi

echo "All integration tests passed!"
