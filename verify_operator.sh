#!/bin/bash

# Script to verify operator deployment

echo "Verifying operator deployment..."

# Check if namespace exists
if kubectl get namespace recac-system >/dev/null 2>&1; then
    echo "✓ Namespace recac-system exists"
else
    echo "✗ Namespace recac-system does not exist"
    exit 1
fi

# Check if deployment exists
if kubectl get deployment recac-operator -n recac-system >/dev/null 2>&1; then
    echo "✓ Deployment recac-operator exists"
else
    echo "✗ Deployment recac-operator does not exist"
    exit 1
fi

# Check if pod is running
POD_STATUS=$(kubectl get pods -n recac-system -l app=recac-operator -o jsonpath='{.items[0].status.phase}')
if [ "$POD_STATUS" = "Running" ]; then
    echo "✓ Operator pod is running"
else
    echo "✗ Operator pod is not running (status: $POD_STATUS)"
    exit 1
fi

# Check pod logs for errors
if kubectl logs -n recac-system -l app=recac-operator | grep -i "error" >/dev/null 2>&1; then
    echo "✗ Errors found in operator logs"
    kubectl logs -n recac-system -l app=recac-operator
    exit 1
else
    echo "✓ No errors found in operator logs"
fi

echo "Operator deployment verification successful!"
