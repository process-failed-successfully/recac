#!/bin/bash

# init.sh - Setup development environment for RECAC (Go Refactor)

set -e

# Ensure Go is in PATH
export PATH=$PATH:$HOME/go-dist/go/bin:$HOME/go/bin:/home/appuser/go-toolset/bin

echo "=== Initializing RECAC Development Environment ==="

# 1. Check Go Version
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21+."
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "✅ Found Go version: $GO_VERSION"

# 2. Check Docker
if ! command -v docker &> /dev/null; then
    echo "⚠️ Docker is not found. Some features will be disabled."
else
    if docker info > /dev/null 2>&1; then
        echo "✅ Docker is running."
    else
        echo "⚠️ Docker is installed but not running (daemon not responsive)."
    fi
fi

# 3. Initialize Go Module if missing
if [ ! -f "go.mod" ]; then
    echo "Creating go.mod..."
    go mod init recac
    echo "✅ Initialized go module 'recac'"
else
    echo "✅ go.mod found."
fi

# 4. Install/Tidy Dependencies
echo "Tidying dependencies..."
go mod tidy
echo "✅ Dependencies ready."

# 5. Create basic directory structure if missing
echo "Ensuring directory structure..."
mkdir -p cmd/recac
mkdir -p internal/agent
mkdir -p internal/runner
mkdir -p internal/ui
mkdir -p internal/docker
mkdir -p internal/jira
mkdir -p internal/notify
mkdir -p pkg

echo "=== Setup Complete ==="
echo "To run the application (once built):"
echo "  go run cmd/recac/main.go"
echo "To run tests:"
echo "  go test ./..."
