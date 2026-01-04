#!/bin/bash

# Test script for k8s-apply-success feature

echo "Testing Kubernetes deployment application..."

# Test 1: Check if deployment.yaml exists
if [ ! -f "deployment.yaml" ]; then
    echo "FAIL: deployment.yaml does not exist"
    exit 1
fi

# Test 2: Check if service-account.yaml exists
if [ ! -f "service-account.yaml" ]; then
    echo "FAIL: service-account.yaml does not exist"
    exit 1
fi

# Test 3: Check if env-variables.yaml exists
if [ ! -f "env-variables.yaml" ]; then
    echo "FAIL: env-variables.yaml does not exist"
    exit 1
fi

# Test 4: Dry-run apply deployment
echo "Testing deployment.yaml with kubectl dry-run..."
if ! kubectl apply --dry-run=client -f deployment.yaml > /dev/null 2>&1; then
    echo "FAIL: deployment.yaml failed kubectl dry-run validation"
    exit 1
fi

# Test 5: Dry-run apply service account
echo "Testing service-account.yaml with kubectl dry-run..."
if ! kubectl apply --dry-run=client -f service-account.yaml > /dev/null 2>&1; then
    echo "FAIL: service-account.yaml failed kubectl dry-run validation"
    exit 1
fi

# Test 6: Dry-run apply env variables
echo "Testing env-variables.yaml with kubectl dry-run..."
if ! kubectl apply --dry-run=client -f env-variables.yaml > /dev/null 2>&1; then
    echo "FAIL: env-variables.yaml failed kubectl dry-run validation"
    exit 1
fi

echo "PASS: All Kubernetes resources can be successfully applied"
