#!/bin/bash

# Test 1: Check if agent manifests exist
echo "Test 1: Checking agent manifests..."
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

# Test 2: Check if kustomization file exists
echo "Test 2: Checking kustomization file..."
if [ -f "agent/manifests/kustomization.yaml" ]; then
    echo "✓ Kustomization file exists"
else
    echo "✗ Kustomization file does not exist"
    exit 1
fi

# Test 3: Validate YAML syntax
echo "Test 3: Validating YAML syntax..."
for file in agent/manifests/*.yaml; do
    if ! python3 -c "import yaml; yaml.safe_load(open('$file'))" 2>/dev/null; then
        echo "✗ $file has invalid syntax"
        exit 1
    fi
done
echo "✓ All YAML files have valid syntax"

echo "All tests passed!"
