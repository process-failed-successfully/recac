#!/bin/bash

# Test script for environment variables configuration

echo "Testing environment variables configuration..."

# Test 1: File exists
if [ ! -f "env-variables.yaml" ]; then
    echo "FAIL: env-variables.yaml does not exist"
    exit 1
fi

# Test 2: ConfigMap defined
if ! grep -q "kind: ConfigMap" env-variables.yaml; then
    echo "FAIL: ConfigMap not defined"
    exit 1
fi

# Test 3: Secret defined
if ! grep -q "kind: Secret" env-variables.yaml; then
    echo "FAIL: Secret not defined"
    exit 1
fi

# Test 4: Required environment variables present
if ! grep -q "LOG_LEVEL" env-variables.yaml; then
    echo "FAIL: LOG_LEVEL not defined"
    exit 1
fi

if ! grep -q "JIRA_BASE_URL" env-variables.yaml; then
    echo "FAIL: JIRA_BASE_URL not defined"
    exit 1
fi

if ! grep -q "GITHUB_API_URL" env-variables.yaml; then
    echo "FAIL: GITHUB_API_URL not defined"
    exit 1
fi

# Test 5: Secrets for Jira and GitHub
if ! grep -q "jira-api-key" env-variables.yaml; then
    echo "FAIL: jira-api-key secret not defined"
    exit 1
fi

if ! grep -q "github-token" env-variables.yaml; then
    echo "FAIL: github-token secret not defined"
    exit 1
fi

echo "PASS: All environment variables configuration tests passed"
