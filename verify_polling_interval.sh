#!/bin/bash

echo "Verifying Jira Polling Interval Configuration..."

# Test 1: Default interval
echo "Test 1: Default interval (5 minutes)"
unset JIRA_POLLING_INTERVAL
go run -exec echo main.go 2>&1 | grep -q "interval=5m" && echo "✓ Default interval works" || echo "✗ Default interval failed"

# Test 2: Custom interval via environment variable
echo "Test 2: Custom interval (30 seconds)"
JIRA_POLLING_INTERVAL=30s go run -exec echo main.go 2>&1 | grep -q "interval=30s" && echo "✓ Custom interval works" || echo "✗ Custom interval failed"

# Test 3: Invalid interval falls back to default
echo "Test 3: Invalid interval falls back to default"
JIRA_POLLING_INTERVAL=invalid go run -exec echo main.go 2>&1 | grep -q "interval=5m" && echo "✓ Fallback to default works" || echo "✗ Fallback failed"

echo "All verification tests completed."
